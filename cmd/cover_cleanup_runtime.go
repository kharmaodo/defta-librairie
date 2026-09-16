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
	coverCleanupBatchSize    = 16
	coverCleanupPollInterval      = 2 * time.Second
	coverCleanupReconcileInterval = 5 * time.Minute
)

func runCoverCleanup(
	ctx context.Context,
	cfg *config.Config,
	db *sql.DB,
	logger *slog.Logger,
) {
	workerID, err := identity.NewID()
	if err != nil {
		logger.Error("cover_cleanup_worker_id_failed", "error", err)
		return
	}
	objects, err := covers.NewMinIOProcessingStore(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucketCovers,
		cfg.MinIOUseSSL,
	)
	if err != nil {
		logger.Error("cover_cleanup_store_invalid", "error", err)
		return
	}
	cleaner, err := services.NewCoverCleanupService(
		repositories.NewCoverCleanupRepository(db),
		objects,
		workerID,
	)
	if err != nil {
		logger.Error("cover_cleanup_worker_invalid", "error", err)
		return
	}
	cleaner.WithSourceRetention(
		time.Duration(cfg.MinIOSourceRetentionHours) * time.Hour,
	)
	logger.Info("cover_cleanup_worker_started", "worker_id", workerID)

	nextReconciliation := time.Time{}
	for ctx.Err() == nil {
		now := time.Now().UTC()
		if !now.Before(nextReconciliation) {
			reconciled, reconcileErr := cleaner.Reconcile(ctx)
			if reconcileErr != nil && ctx.Err() == nil {
				logger.Warn(
					"cover_cleanup_reconcile_failed",
					"error", reconcileErr,
					"worker_id", workerID,
				)
			} else if reconciled > 0 {
				logger.Info(
					"cover_cleanup_reconciled",
					"jobs", reconciled,
					"worker_id", workerID,
				)
			}
			nextReconciliation = now.Add(coverCleanupReconcileInterval)
		}

		count, cleanupErr := cleaner.CleanAvailable(ctx, coverCleanupBatchSize)
		if cleanupErr != nil && ctx.Err() == nil {
			logger.Warn(
				"cover_cleanup_failed",
				"error", cleanupErr,
				"worker_id", workerID,
			)
		}
		if count == coverCleanupBatchSize {
			continue
		}
		if !waitForCoverCleanup(ctx, coverCleanupPollInterval) {
			return
		}
	}
}

func waitForCoverCleanup(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
