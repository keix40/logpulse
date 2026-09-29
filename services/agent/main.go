package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ingestURL := flag.String("ingest-url", envOr("INGEST_URL", "http://localhost:8080/v1/logs"), "ingest batch URL")
	file := flag.String("file", "", "log file to tail (required)")
	service := flag.String("service", "file-agent", "default service name")
	batchSize := flag.Int("batch-size", 20, "max batch size")
	flushEvery := flag.Duration("flush-interval", time.Second, "max time between batches")
	flag.Parse()

	if *file == "" {
		log.Fatal("--file is required")
	}

	ingest := &ingestClient{
		url:    *ingestURL,
		client: &http.Client{Timeout: 10 * time.Second},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("tailing %s → %s", *file, *ingestURL)
	if err := tailFile(ctx, *file, *service, *batchSize, *flushEvery, ingest, nil); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
	fmt.Println("agent stopped")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
