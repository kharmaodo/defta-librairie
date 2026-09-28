//go:build fts5

package main

import (
	"context"
	"database/sql"
	"defta-librairie/internal/config"
	"defta-librairie/internal/repositories"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

func runCoverImportOutboxPublisher(ctx context.Context, cfg *config.Config, db *sql.DB, logger *slog.Logger) {
	for {
		if err := publishCoverImportOutbox(ctx, cfg, db); err != nil {
			logger.Warn("cover_import_outbox_publisher_retry", "error", err)
		}
		select { case <-ctx.Done(): return; case <-time.After(2*time.Second): }
	}
}

func publishCoverImportOutbox(ctx context.Context, cfg *config.Config, db *sql.DB) error {
	if cfg == nil || db == nil { return fmt.Errorf("invalid cover import outbox configuration") }
	opts := []nats.Option{nats.Name("defta-librairie-cover-import-outbox")}
	if cfg.NATSUser != "" || cfg.NATSPassword != "" { opts = append(opts, nats.UserInfo(cfg.NATSUser, cfg.NATSPassword)) }
	connection, err := nats.Connect(cfg.NATSURL, opts...); if err != nil { return err }; defer connection.Close()
	stream, err := connection.JetStream(); if err != nil { return err }
	if _,err=stream.StreamInfo(cfg.NATSImportStream);err!=nil { if _,err=stream.AddStream(&nats.StreamConfig{Name:cfg.NATSImportStream,Subjects:[]string{cfg.NATSImportSubject,cfg.NATSOCRSubject},Storage:nats.FileStorage});err!=nil{return err} }
	repo:=repositories.NewCoverImportRepository(db); now:=time.Now().UTC().Format(time.RFC3339Nano)
	if err=publishOCREvents(ctx,stream,repo,cfg.NATSOCRSubject,now);err!=nil{return err}
	return publishModerationEvents(ctx,stream,repo,cfg.NATSImportSubject,now)
}

func publishOCREvents(ctx context.Context,stream nats.JetStreamContext,repo *repositories.CoverImportRepository,subject,now string) error { events,err:=repo.PendingOCROutbox(ctx,now,50);if err!=nil{return err};for _,event:=range events{if err=publishCoverImportEvent(ctx,stream,subject,event);err!=nil{_ = repo.RecordOCROutboxFailure(ctx,event.EventID,boundedPublishError(err),time.Now().UTC().Add(5*time.Second).Format(time.RFC3339Nano));return err};if err=repo.MarkOCROutboxPublished(ctx,event.EventID,time.Now().UTC().Format(time.RFC3339Nano));err!=nil{return err}};return nil }
func publishModerationEvents(ctx context.Context,stream nats.JetStreamContext,repo *repositories.CoverImportRepository,subject,now string) error { events,err:=repo.PendingModerationOutbox(ctx,now,50);if err!=nil{return err};for _,event:=range events{if err=publishCoverImportEvent(ctx,stream,subject,event);err!=nil{_ = repo.RecordModerationOutboxFailure(ctx,event.EventID,boundedPublishError(err),time.Now().UTC().Add(5*time.Second).Format(time.RFC3339Nano));return err};if err=repo.MarkModerationOutboxPublished(ctx,event.EventID,time.Now().UTC().Format(time.RFC3339Nano));err!=nil{return err}};return nil }
func publishCoverImportEvent(ctx context.Context,stream nats.JetStreamContext,subject string,event repositories.PendingCoverImportOutboxEvent) error { message:=nats.NewMsg(subject);message.Header.Set(nats.MsgIdHdr,event.EventID);message.Data=[]byte(event.Payload);_,err:=stream.PublishMsg(message,nats.Context(ctx));return err }
