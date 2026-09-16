//go:build fts5
// +build fts5

package main

import (
	"context"
	"database/sql"
	"log/slog"

	"defta-librairie/internal/config"
	"defta-librairie/internal/coverruntime"
)

func runBookCoverWorker(
	ctx context.Context,
	cfg *config.Config,
	db *sql.DB,
	logger *slog.Logger,
) {
	coverruntime.RunWorker(ctx, cfg, db, logger)
}
