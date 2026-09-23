//go:build fts5
// +build fts5

package main

import (
	"context"
	"database/sql"
	"defta-librairie/internal/config"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"log/slog"
	"time"
)

func runApprovedCoverPromotionWorker(ctx context.Context, cfg *config.Config, db *sql.DB, logger *slog.Logger) {
	store, err := covers.NewMinIOProcessingStore(
		cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey,
		cfg.MinIOBucketCovers, cfg.MinIOUseSSL,
	)
	if err != nil {
		logger.Error("approved_cover_promotion_store_invalid", "error", err)
		return
	}
	worker := services.NewApprovedCoverPromotionService(
		repositories.NewBookSubmissionRepository(db),
		repositories.NewCoverRepository(db),
		store,
	)
	for {
		if err = worker.RunOnce(ctx, 50); err != nil {
			logger.Warn("approved_cover_promotion_retry", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
