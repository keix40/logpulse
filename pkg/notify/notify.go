package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/logpulse/logpulse/pkg/alertengine"
)

// Config holds outbound webhook settings.
type Config struct {
	SlackWebhookURL   string
	DiscordWebhookURL string
	TelegramBotToken  string
	TelegramChatID    string
}

type Notifier struct {
	cfg    Config
	client *http.Client
	logger *slog.Logger
}

func New(cfg Config, logger *slog.Logger) *Notifier {
	return NewWithClient(cfg, logger, &http.Client{Timeout: 10 * time.Second})
}

func NewWithClient(cfg Config, logger *slog.Logger, client *http.Client) *Notifier {
	return &Notifier{cfg: cfg, client: client, logger: logger}
}

func (n *Notifier) Notify(ctx context.Context, inc alertengine.Incident) error {
	text := FormatIncidentMessage(inc)
	var errs []string
	for _, ch := range inc.Channels {
		switch strings.ToLower(ch) {
		case "slack":
			if n.cfg.SlackWebhookURL == "" {
				if n.logger != nil {
					n.logger.Warn("slack webhook not configured")
				}
				continue
			}
			if err := n.postJSON(ctx, n.cfg.SlackWebhookURL, map[string]interface{}{
				"text": text,
			}); err != nil {
				errs = append(errs, err.Error())
			}
		case "discord":
			if n.cfg.DiscordWebhookURL == "" {
				if n.logger != nil {
					n.logger.Warn("discord webhook not configured")
				}
				continue
			}
			if err := n.postJSON(ctx, n.cfg.DiscordWebhookURL, map[string]interface{}{
				"content": text,
			}); err != nil {
				errs = append(errs, err.Error())
			}
		case "telegram":
			if n.cfg.TelegramBotToken == "" || n.cfg.TelegramChatID == "" {
				if n.logger != nil {
					n.logger.Warn("telegram not configured")
				}
				continue
			}
			url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.cfg.TelegramBotToken)
			if err := n.postJSON(ctx, url, map[string]interface{}{
				"chat_id": n.cfg.TelegramChatID,
				"text":    text,
			}); err != nil {
				errs = append(errs, err.Error())
			}
		default:
			if n.logger != nil {
				n.logger.Warn("unknown channel", "channel", ch)
			}
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (n *Notifier) postJSON(ctx context.Context, url string, body map[string]interface{}) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}

// FormatIncidentMessage renders a human-readable alert body.
func FormatIncidentMessage(inc alertengine.Incident) string {
	return fmt.Sprintf("[LogPulse] %s — %s (count=%d)\n%s",
		inc.RuleName, inc.RuleID, inc.Count, inc.Sample)
}
