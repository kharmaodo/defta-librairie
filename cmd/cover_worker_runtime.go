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
	coverWorkerBatchSize      = 8
	coverWorkerFetchWait      = time.Second
	coverWorkerReconnectDelay = 5 * time.Second
)

func runBookCoverWorker(
	ctx context.Context,
	cfg *config.Config,
	db *sql.DB,
	logger *slog.Logger,
) {
	workerID, err := identity.NewID()
	if err != nil {
		logger.Error("cover_worker_id_failed", "error", err)
		return
	}

	processingStore, err := covers.NewMinIOProcessingStore(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucketCovers,
		cfg.MinIOUseSSL,
	)
	if err != nil {
		logger.Error("cover_worker_minio_invalid", "error", err)
		return
	}

	variantProcessor, err := services.NewStoredCoverVariantProcessor(
		processingStore,
		processingStore,
		covers.NewImageProcessor(),
	)
	if err != nil {
		logger.Error("cover_worker_processor_invalid", "error", err)
		return
	}

	worker, err := services.NewBookCoverWorker(
		repositories.NewCoverProcessingRepository(db),
		variantProcessor,
		workerID,
	)
	if err != nil {
		logger.Error("cover_worker_invalid", "error", err)
		return
	}

	for ctx.Err() == nil {
		consumer, connectErr := covers.NewJetStreamConsumer(
			ctx,
			cfg.NATSURL,
			cfg.NATSUser,
			cfg.NATSPassword,
			cfg.NATSCoversStream,
			cfg.NATSCoversSubject,
			cfg.NATSCoversConsumer,
			cfg.CoverWorkerMaxDeliver,
		)
		if connectErr != nil {
			logger.Warn("cover_worker_nats_unavailable", "error", connectErr)
			if !waitForCoverWorker(ctx, coverWorkerReconnectDelay) {
				return
			}
			continue
		}

		logger.Info("cover_worker_connected", "worker_id", workerID)

		reconnect := false
		for ctx.Err() == nil {
			_, consumeErr := consumer.FetchAndHandle(
				ctx,
				coverWorkerBatchSize,
				coverWorkerFetchWait,
				worker,
			)
			if consumeErr != nil {
				if ctx.Err() == nil {
					logger.Warn(
						"cover_worker_consume_failed",
						"error", consumeErr,
						"worker_id", workerID,
					)
					reconnect = true
				}
				break
			}
		}

		if closeErr := consumer.Close(); closeErr != nil && ctx.Err() == nil {
			logger.Warn("cover_worker_nats_close_failed", "error", closeErr)
		}
		if !reconnect || !waitForCoverWorker(ctx, coverWorkerReconnectDelay) {
			return
		}
	}
}

func waitForCoverWorker(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
