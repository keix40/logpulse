package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/logpulse/logpulse/pkg/logevent"
)

type lineParser func(line, defaultService string) logevent.Entry

func defaultLineParser(line, defaultService string) logevent.Entry {
	level := logevent.LevelInfo
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error"):
		level = logevent.LevelError
	case strings.Contains(lower, "warn"):
		level = logevent.LevelWarn
	case strings.Contains(lower, "debug"):
		level = logevent.LevelDebug
	}
	return logevent.Entry{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Service:   defaultService,
		Message:   line,
	}
}

type ingestClient struct {
	url    string
	client *http.Client
}

func (c *ingestClient) send(ctx context.Context, entries []logevent.Entry) error {
	if len(entries) == 0 {
		return nil
	}
	body, err := json.Marshal(logevent.BatchRequest{Logs: entries})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ingest status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func tailFile(ctx context.Context, path, service string, batchSize int, flushEvery time.Duration, ingest *ingestClient, parse lineParser) error {
	if parse == nil {
		parse = defaultLineParser
	}

	offset, err := fileSize(path)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(flushEvery)
	defer ticker.Stop()

	pending := make([]logevent.Entry, 0, batchSize)
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		if err := ingest.send(ctx, pending); err != nil {
			return err
		}
		pending = pending[:0]
		return nil
	}

 poll:
	for {
		select {
		case <-ctx.Done():
			return flush()
		case <-ticker.C:
			if err := flush(); err != nil {
				return err
			}
		default:
		}

		size, err := fileSize(path)
		if err != nil {
			return err
		}
		if size < offset {
			offset = 0
		}
		if size == offset {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			_ = f.Close()
			return err
		}
		reader := bufio.NewReader(f)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					offset, _ = f.Seek(0, io.SeekCurrent)
					_ = f.Close()
					continue poll
				}
				_ = f.Close()
				return err
			}
			offset += int64(len(line))
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			entry := parse(line, service)
			_ = entry.Validate()
			pending = append(pending, entry)
			if len(pending) >= batchSize {
				if err := flush(); err != nil {
					_ = f.Close()
					return err
				}
			}
		}
	}
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
