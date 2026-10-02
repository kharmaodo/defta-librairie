//go:build fts5

package main

import (
	"context"
	"database/sql"
	"defta-librairie/internal/config"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

type coverImportMatchMessage struct {
	SchemaVersion int    `json:"schemaVersion"`
	EventID       string `json:"eventId"`
	JobID         string `json:"jobId"`
	LibraryID     string `json:"libraryId"`
}

func runCoverImportMatchWorker(ctx context.Context, cfg *config.Config, db *sql.DB, logger *slog.Logger) {
	opts := []nats.Option{nats.Name("defta-librairie-cover-import-match-worker")}
	if cfg.NATSUser != "" || cfg.NATSPassword != "" {
		opts = append(opts, nats.UserInfo(cfg.NATSUser, cfg.NATSPassword))
	}
	connection, err := nats.Connect(cfg.NATSURL, opts...)
	if err != nil {
		logger.Error("cover_import_match_nats_unavailable", "error", err)
		return
	}
	defer connection.Close()
	stream, err := connection.JetStream()
	if err != nil {
		logger.Error("cover_import_match_jetstream_unavailable", "error", err)
		return
	}
	if _, err = stream.ConsumerInfo(cfg.NATSImportStream, cfg.NATSMatchConsumer); err != nil {
		_, err = stream.AddConsumer(cfg.NATSImportStream, &nats.ConsumerConfig{Durable: cfg.NATSMatchConsumer, FilterSubject: cfg.NATSMatchSubject, AckPolicy: nats.AckExplicitPolicy, AckWait: 30 * time.Second, MaxDeliver: cfg.CoverWorkerMaxDeliver})
		if err != nil {
			logger.Error("cover_import_match_consumer_failed", "error", err)
			return
		}
	}
	sub, err := stream.PullSubscribe(cfg.NATSMatchSubject, cfg.NATSMatchConsumer, nats.BindStream(cfg.NATSImportStream), nats.ManualAck())
	if err != nil {
		logger.Error("cover_import_match_subscribe_failed", "error", err)
		return
	}
	service := services.NewCoverImportMatchingService(repositories.NewCoverImportRepository(db)).WithQualityMatching(cfg.OCRExperimentalEnabled)
	for ctx.Err() == nil {
		messages, fetchErr := sub.Fetch(1, nats.MaxWait(time.Second))
		if errors.Is(fetchErr, nats.ErrTimeout) {
			continue
		}
		if fetchErr != nil {
			logger.Warn("cover_import_match_fetch_failed", "error", fetchErr)
			continue
		}
		for _, message := range messages {
			var event coverImportMatchMessage
			if json.Unmarshal(message.Data, &event) != nil || event.SchemaVersion != 1 || event.EventID == "" || event.JobID == "" || event.LibraryID == "" || message.Header.Get(nats.MsgIdHdr) != event.EventID {
				logger.Warn("cover_import_match_invalid_event")
				_ = message.Term()
				continue
			}
			err = service.Process(ctx, event.JobID, event.LibraryID)
			if err == nil || services.IsMatchingAlreadyProcessed(err) {
				_ = message.Ack()
				continue
			}
			metadata, metaErr := message.Metadata()
			// #nosec G115 -- Value is validated positive and bounded before conversion; no unsigned-to-signed narrowing.
			if metaErr == nil && metadata.NumDelivered >= uint64(cfg.CoverWorkerMaxDeliver) {
				if failErr := service.Fail(ctx, event.JobID, event.LibraryID); failErr == nil || services.IsMatchingAlreadyProcessed(failErr) {
					logger.Error("cover_import_match_failed_terminal", "job_id", event.JobID, "library_id", event.LibraryID, "error", err)
					_ = message.Ack()
					continue
				}
			}
			logger.Warn("cover_import_match_retry", "job_id", event.JobID, "library_id", event.LibraryID, "error", err)
			_ = message.NakWithDelay(5 * time.Second)
		}
	}
}
