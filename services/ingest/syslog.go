package main

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/logpulse/logpulse/pkg/logevent"
)

// RFC5424-ish: <pri>version timestamp hostname app procid msgid [sd] msg
var syslogRe = regexp.MustCompile(`^<\d+>\d*\s+\S+\s+\S+\s+(?P<app>\S+)\s+\S*\s+\S*\s+(?P<msg>.*)$`)

func serveSyslog(ln net.Listener, p *StreamPublisher, logger *slog.Logger) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go handleSyslogConn(conn, p, logger)
	}
}

func handleSyslogConn(conn net.Conn, p *StreamPublisher, logger *slog.Logger) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		entry := parseSyslogLine(line)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := p.Publish(ctx, []logevent.Entry{entry})
		cancel()
		if err != nil {
			logger.Error("syslog publish failed", "err", err)
		}
	}
}

func parseSyslogLine(line string) logevent.Entry {
	entry := logevent.Entry{
		Timestamp: time.Now().UTC(),
		Level:     logevent.LevelInfo,
		Service:   "syslog",
		Message:   line,
	}
	if m := syslogRe.FindStringSubmatch(line); len(m) == 3 {
		entry.Service = m[1]
		entry.Message = m[2]
	}
	lower := strings.ToLower(entry.Message)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "err:"):
		entry.Level = logevent.LevelError
	case strings.Contains(lower, "warn"):
		entry.Level = logevent.LevelWarn
	case strings.Contains(lower, "debug"):
		entry.Level = logevent.LevelDebug
	}
	return entry
}
