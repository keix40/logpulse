package main

import "os"

type config struct {
	HTTPAddr          string
	RedisAddr         string
	RulesPath         string
	SlackWebhookURL   string
	DiscordWebhookURL string
	TelegramBotToken  string
	TelegramChatID    string
}

func loadConfig() config {
	return config{
		HTTPAddr:          envOr("ALERTER_HTTP_ADDR", "0.0.0.0:8082"),
		RedisAddr:         envOr("REDIS_ADDR", "redis:6379"),
		RulesPath:         envOr("ALERT_RULES_PATH", "/etc/logpulse/alerts.yaml"),
		SlackWebhookURL:   os.Getenv("SLACK_WEBHOOK_URL"),
		DiscordWebhookURL: os.Getenv("DISCORD_WEBHOOK_URL"),
		TelegramBotToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:    os.Getenv("TELEGRAM_CHAT_ID"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
