package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/logpulse/logpulse/pkg/alertengine"
)

type Notifier struct {
	cfg    config
	client *http.Client
	logger *slog.Logger
}

func NewNotifier(cfg config, logger *slog.Logger) *Notifier {
	return &Notifier{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}

func (n *Notifier) Notify(ctx context.Context, inc alertengine.Incident) error {
	text := fmt.Sprintf("[LogPulse] %s — %s (count=%d)\n%s",
		inc.RuleName, inc.RuleID, inc.Count, inc.Sample)
	var errs []string
	for _, ch := range inc.Channels {
		switch strings.ToLower(ch) {
		case "slack":
			if n.cfg.SlackWebhookURL == "" {
				n.logger.Warn("slack webhook not configured")
				continue
			}
			if err := n.postJSON(ctx, n.cfg.SlackWebhookURL, map[string]interface{}{
				"text": text,
			}); err != nil {
				errs = append(errs, err.Error())
			}
		case "discord":
			if n.cfg.DiscordWebhookURL == "" {
				n.logger.Warn("discord webhook not configured")
				continue
			}
			if err := n.postJSON(ctx, n.cfg.DiscordWebhookURL, map[string]interface{}{
				"content": text,
			}); err != nil {
				errs = append(errs, err.Error())
			}
		case "telegram":
			if n.cfg.TelegramBotToken == "" || n.cfg.TelegramChatID == "" {
				n.logger.Warn("telegram not configured")
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
			n.logger.Warn("unknown channel", "channel", ch)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
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
