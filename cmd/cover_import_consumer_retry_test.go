//go:build fts5

package main

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoverImportConsumerRetriesAfterEarlyReturn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	done := make(chan struct{})
	go func() {
		defer close(done)
		retryCoverImportConsumer(ctx, "matching", logger, func() {
			if calls.Add(1) == 2 {
				cancel()
			}
		})
	}()
	select {
	case <-done:
	case <-time.After(2 * coverImportConsumerRetryDelay):
		t.Fatal("consumer did not retry")
	}
	if calls.Load() != 2 {
		t.Fatalf("got %d attempts, want 2", calls.Load())
	}
}

func TestCoverImportConsumerStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls atomic.Int32
	retryCoverImportConsumer(ctx, "ocr", slog.Default(), func() { calls.Add(1) })
	if calls.Load() != 0 {
		t.Fatalf("started %d times after shutdown", calls.Load())
	}
}
