//go:build fts5
// +build fts5

package main

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"defta-librairie/internal/config"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
)

const (
	coverOutboxBatchSize      = 32
	coverOutboxPollInterval   = time.Second
	coverOutboxReconnectDelay = 5 * time.Second
)

func runCoverOutboxPublisher(
	ctx context.Context,
	cfg *config.Config,
	db *sql.DB,
	logger *slog.Logger,
) {
	workerID, err := identity.NewID()
	if err != nil {
		logger.Error("cover_outbox_publisher_id_failed", "error", err)
		return
	}
	store := repositories.NewCoverOutboxRepository(db)

	for ctx.Err() == nil {
		transport, connectErr := covers.NewJetStreamPublisher(
			ctx,
			cfg.NATSURL,
			cfg.NATSUser,
			cfg.NATSPassword,
			cfg.NATSCoversStream,
			cfg.NATSCoversSubject,
		)
		if connectErr != nil {
			logger.Warn("cover_outbox_nats_unavailable", "error", connectErr)
			if !waitForCoverPublisher(ctx, coverOutboxReconnectDelay) {
				return
			}
			continue
		}

		publisher, publisherErr := services.NewCoverOutboxPublisher(
			store, transport, workerID, cfg.NATSCoversSubject,
		)
		if publisherErr != nil {
			_ = transport.Close()
			logger.Error("cover_outbox_publisher_invalid", "error", publisherErr)
			return
		}
		logger.Info("cover_outbox_publisher_connected", "worker_id", workerID)

		reconnect := false
		for ctx.Err() == nil {
			count, publishErr := publisher.PublishAvailable(
				ctx, coverOutboxBatchSize,
			)
			if publishErr != nil {
				if ctx.Err() == nil {
					logger.Warn(
						"cover_outbox_publish_failed",
						"error", publishErr,
						"worker_id", workerID,
					)
					reconnect = true
				}
				break
			}
			if count == coverOutboxBatchSize {
				continue
			}
			if !waitForCoverPublisher(ctx, coverOutboxPollInterval) {
				break
			}
		}
		if closeErr := transport.Close(); closeErr != nil && ctx.Err() == nil {
			logger.Warn("cover_outbox_nats_close_failed", "error", closeErr)
		}
		if !reconnect || !waitForCoverPublisher(ctx, coverOutboxReconnectDelay) {
			return
		}
	}
}

func waitForCoverPublisher(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
