//go:build fts5
// +build fts5

package main

import (
	"context"
	"database/sql"
	"defta-librairie/internal/config"
	"defta-librairie/internal/repositories"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

func runBookSubmissionOutboxPublisher(ctx context.Context, cfg *config.Config, db *sql.DB, logger *slog.Logger) {
	if cfg == nil || db == nil {
		logger.Error("submission_outbox_publisher_invalid_configuration")
		return
	}
	for {
		if err := publishBookSubmissionOutbox(ctx, cfg, db); err != nil {
			logger.Warn("submission_outbox_publisher_retry", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func publishBookSubmissionOutbox(ctx context.Context, cfg *config.Config, db *sql.DB) error {
	options := []nats.Option{nats.Name("defta-librairie-book-submission-outbox")}
	if cfg.NATSUser != "" || cfg.NATSPassword != "" {
		options = append(options, nats.UserInfo(cfg.NATSUser, cfg.NATSPassword))
	}
	connection, err := nats.Connect(cfg.NATSURL, options...)
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}
	defer connection.Close()
	stream, err := connection.JetStream()
	if err != nil {
		return fmt.Errorf("open jetstream: %w", err)
	}
	if _, err = stream.StreamInfo(cfg.NATSSubmissionStream); err != nil {
		if _, err = stream.AddStream(&nats.StreamConfig{
			Name:     cfg.NATSSubmissionStream,
			Subjects: []string{cfg.NATSSubmissionSubject},
			Storage:  nats.FileStorage,
		}); err != nil {
			return fmt.Errorf("create submission stream: %w", err)
		}
	}
	repository := repositories.NewBookSubmissionRepository(db)
	now := time.Now().UTC()
	events, err := repository.PendingOutbox(ctx, now.Format(time.RFC3339Nano), 50)
	if err != nil {
		return err
	}
	for _, event := range events {
		message := nats.NewMsg(cfg.NATSSubmissionSubject)
		message.Header.Set(nats.MsgIdHdr, event.EventID)
		message.Data = []byte(event.Payload)
		if _, err = stream.PublishMsg(message, nats.Context(ctx)); err != nil {
			retryAt := time.Now().UTC().Add(5 * time.Second).Format(time.RFC3339Nano)
			_ = repository.RecordOutboxFailure(ctx, event.EventID, boundedPublishError(err), retryAt)
			return fmt.Errorf("publish submission event %s: %w", event.EventID, err)
		}
		if err = repository.MarkOutboxPublished(ctx, event.EventID, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return nil
}

func boundedPublishError(err error) string {
	message := strings.TrimSpace(err.Error())
	if len(message) > 500 {
		return message[:500]
	}
	return message
}
