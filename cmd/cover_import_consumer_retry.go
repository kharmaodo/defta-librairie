//go:build fts5

package main

import (
	"context"
	"log/slog"
	"time"
)

const coverImportConsumerRetryDelay = 5 * time.Second

// A consumer may start before the outbox publisher creates its JetStream
// stream, or return when its connection is lost. Reconnect until shutdown.
func retryCoverImportConsumer(ctx context.Context, name string, logger *slog.Logger, run func()) {
	for ctx.Err() == nil {
		run()
		if ctx.Err() != nil {
			return
		}
		logger.Warn("cover_import_consumer_restarting", "consumer", name)
		timer := time.NewTimer(coverImportConsumerRetryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}
