package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/logpulse/logpulse/pkg/alertengine"
	"github.com/logpulse/logpulse/pkg/alerterstream"
	"github.com/logpulse/logpulse/pkg/auth"
	"github.com/logpulse/logpulse/pkg/ingestapi"
	"github.com/logpulse/logpulse/pkg/livehub"
	"github.com/logpulse/logpulse/pkg/notify"
	"github.com/logpulse/logpulse/pkg/redisx"
	"github.com/logpulse/logpulse/pkg/storage"
	"github.com/logpulse/logpulse/pkg/workerstream"
)

func main() {
	cfg := loadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	rdb, err := redisx.NewClientFromEnv()
	if err != nil {
		logger.Error("redis config", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("redis ping failed", "err", err)
		os.Exit(1)
	}

	workerstream.EnsureGroup(ctx, rdb, redisx.StreamLogs, redisx.GroupWorker, logger)
	if err := rdb.XGroupCreateMkStream(ctx, redisx.StreamLogs, redisx.GroupAlerter, "0").Err(); err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		logger.Warn("alerter consumer group create", "err", err)
	}

	store, err := storage.OpenFromEnv()
	if err != nil {
		logger.Error("storage open failed", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	hub := livehub.New()

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()
	go workerstream.RunConsumer(runCtx, rdb, store, hub, logger)
	storage.StartRetentionLoop(runCtx, store, logger)

	rules, err := alertengine.LoadRules(cfg.RulesPath)
	if err != nil {
		logger.Error("load rules", "err", err)
		os.Exit(1)
	}
	engine := alertengine.NewEngine(rules)
	notifier := notify.New(cfg.Notify, logger)
	go alerterstream.Run(runCtx, rdb, engine, notifier, alerterstream.NotifyOptions{MaxAttempts: cfg.MaxNotifyAttempts}, logger)

	publisher := &ingestapi.StreamPublisher{Client: rdb, Logger: logger, StreamMaxLen: cfg.StreamMaxLen}
	limits := ingestapi.Limits{MaxBodyBytes: cfg.MaxBodyBytes}

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Timeout(30*time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Post("/v1/logs", auth.RequireBearerFunc(cfg.IngestAPIKey, ingestapi.HandleBatch(publisher, limits, logger)))
	readAuth := func(next http.Handler) http.Handler {
		return auth.RequireBearer(cfg.ReadAPIKey, next)
	}
	r.With(readAuth).Get("/v1/live", hub.SSEHandler())
	r.With(readAuth).Get("/v1/logs/search", store.SearchHandler(logger))

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}
	go func() {
		logger.Info("logpulse-all listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", "err", err)
			os.Exit(1)
		}
	}()

	var syslogLn net.Listener
	if cfg.SyslogAddr != "" {
		ln, err := net.Listen("tcp", cfg.SyslogAddr)
		if err != nil {
			logger.Error("syslog listen failed", "err", err)
			os.Exit(1)
		}
		syslogLn = ln
		go ingestapi.ServeSyslog(syslogLn, publisher, logger)
		logger.Info("syslog listening", "addr", cfg.SyslogAddr)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	runCancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	if syslogLn != nil {
		_ = syslogLn.Close()
	}
	_ = rdb.Close()
}

type config struct {
	HTTPAddr            string
	SyslogAddr          string
	StreamMaxLen        int64
	MaxBodyBytes        int64
	IngestAPIKey        string
	ReadAPIKey          string
	CORSOrigins         []string
	RulesPath           string
	MaxNotifyAttempts   int
	Notify              notify.Config
}

func loadConfig() config {
	maxBody := parseInt64Env("INGEST_MAX_BODY_BYTES", 2<<20)
	streamMax := parseInt64Env("REDIS_STREAM_MAXLEN", 100000)
	origins := envOr("WORKER_CORS_ORIGINS", "http://localhost:3000")
	return config{
		HTTPAddr:          listenAddr(),
		SyslogAddr:        os.Getenv("INGEST_SYSLOG_ADDR"),
		StreamMaxLen:      streamMax,
		MaxBodyBytes:      maxBody,
		IngestAPIKey:      os.Getenv("INGEST_API_KEY"),
		ReadAPIKey:        os.Getenv("READ_API_KEY"),
		CORSOrigins:       strings.Split(origins, ","),
		RulesPath:         envOr("ALERT_RULES_PATH", rulesPathDefault()),
		MaxNotifyAttempts: parseIntEnv("ALERTER_MAX_NOTIFY_ATTEMPTS", 5),
		Notify: notify.Config{
			SlackWebhookURL:   os.Getenv("SLACK_WEBHOOK_URL"),
			DiscordWebhookURL: os.Getenv("DISCORD_WEBHOOK_URL"),
			TelegramBotToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
			TelegramChatID:    os.Getenv("TELEGRAM_CHAT_ID"),
		},
	}
}

func listenAddr() string {
	if port := os.Getenv("PORT"); port != "" {
		return "0.0.0.0:" + port
	}
	return envOr("HTTP_ADDR", "0.0.0.0:8080")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseIntEnv(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func parseInt64Env(key string, def int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func rulesPathDefault() string {
	if _, err := os.Stat("/etc/logpulse/alerts.yaml"); err == nil {
		return "/etc/logpulse/alerts.yaml"
	}
	return "deploy/alerts.yaml"
}
