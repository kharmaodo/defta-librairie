//go:build fts5
// +build fts5

package main

import (
	"context"
	"database/sql"
	"defta-librairie/internal/config"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/moderation"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

type submissionModerationMessage struct {
	SubmissionID string `json:"submissionId"`
}

func runBookSubmissionWorker(ctx context.Context, cfg *config.Config, db *sql.DB, logger *slog.Logger) {
	if cfg == nil || db == nil {
		logger.Error("submission_worker_invalid_configuration")
		return
	}
	options := []nats.Option{nats.Name("defta-librairie-book-submission-worker")}
	if cfg.NATSUser != "" || cfg.NATSPassword != "" {
		options = append(options, nats.UserInfo(cfg.NATSUser, cfg.NATSPassword))
	}
	connection, err := nats.Connect(cfg.NATSURL, options...)
	if err != nil {
		logger.Error("submission_worker_nats_unavailable", "error", err)
		return
	}
	defer connection.Close()
	stream, err := connection.JetStream()
	if err != nil {
		logger.Error("submission_worker_jetstream_unavailable", "error", err)
		return
	}
	if _, err = stream.ConsumerInfo(cfg.NATSSubmissionStream, cfg.NATSSubmissionConsumer); err != nil {
		if _, err = stream.AddConsumer(cfg.NATSSubmissionStream, &nats.ConsumerConfig{
			Durable:       cfg.NATSSubmissionConsumer,
			FilterSubject: cfg.NATSSubmissionSubject,
			AckPolicy:     nats.AckExplicitPolicy,
			AckWait:       30 * time.Second,
			MaxDeliver:    cfg.CoverWorkerMaxDeliver,
		}); err != nil {
			logger.Error("submission_worker_create_consumer_failed", "error", err)
			return
		}
	}
	subscription, err := stream.PullSubscribe(
		cfg.NATSSubmissionSubject,
		cfg.NATSSubmissionConsumer,
		nats.BindStream(cfg.NATSSubmissionStream),
		nats.ManualAck(),
	)
	if err != nil {
		logger.Error("submission_worker_subscribe_failed", "error", err)
		return
	}
	store, err := covers.NewMinIOProcessingStore(
		cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey,
		cfg.MinIOBucketCovers, cfg.MinIOUseSSL,
	)
	if err != nil {
		logger.Error("submission_worker_store_invalid", "error", err)
		return
	}
	client, err := moderation.NewHTTPClient(cfg.NSFWModerationEndpoint)
	if err != nil {
		logger.Error("submission_worker_moderator_invalid", "error", err)
		return
	}
	worker := services.NewBookSubmissionModerationService(
		repositories.NewBookSubmissionRepository(db), store, client,
	)
	for {
		messages, err := subscription.Fetch(1, nats.MaxWait(time.Second))
		if errors.Is(err, nats.ErrTimeout) {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		if err != nil {
			logger.Warn("submission_worker_fetch_failed", "error", err)
			continue
		}
		for _, message := range messages {
			handleBookSubmissionMessage(ctx, worker, message, logger)
		}
	}
}

func handleBookSubmissionMessage(
	ctx context.Context,
	worker *services.BookSubmissionModerationService,
	message *nats.Msg,
	logger *slog.Logger,
) {
	var event submissionModerationMessage
	if err := json.Unmarshal(message.Data, &event); err != nil || event.SubmissionID == "" {
		logger.Error("submission_worker_invalid_message", "error", err)
		_ = message.Term()
		return
	}
	bookID, err := worker.Process(ctx, event.SubmissionID)
	if err == nil {
		logger.Info("submission_worker_decision_persisted", "submission_id", event.SubmissionID, "book_id", bookID)
		_ = message.Ack()
		return
	}
	if errors.Is(err, services.ErrBookSubmissionModerationFailed) ||
		errors.Is(err, repositories.ErrBookSubmissionNotFound) {
		logger.Warn("submission_worker_terminal_decision", "submission_id", event.SubmissionID, "error", err)
		_ = message.Ack()
		return
	}
	logger.Warn("submission_worker_retry", "submission_id", event.SubmissionID, "error", err)
	if nackErr := message.NakWithDelay(5 * time.Second); nackErr != nil {
		logger.Error("submission_worker_nak_failed", "submission_id", event.SubmissionID, "error", fmt.Errorf("%w: %v", err, nackErr))
	}
}
