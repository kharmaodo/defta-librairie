//go:build fts5
// +build fts5

package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"defta-librairie/internal/config"
	"defta-librairie/internal/coverruntime"
	"defta-librairie/internal/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Erreur chargement configuration : %v", err)
	}
	if !cfg.CoversEnabled {
		log.Fatal("Le worker de couvertures exige COVERS_ENABLED=true")
	}
	if err = database.Init(cfg.DBPath); err != nil {
		log.Fatalf("Erreur connexion à la base SQLite : %v", err)
	}
	defer database.Close()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	logger.Info("cover_worker_started")
	coverruntime.RunWorker(ctx, cfg, database.DB, logger)
	logger.Info("cover_worker_stopped")
}
