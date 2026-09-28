//go:build fts5

package main

import (
	"context"
	"database/sql"
	"defta-librairie/internal/config"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/ocr"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

type coverImportOCRMessage struct { JobID string `json:"jobId"` }

func runCoverImportOCRWorker(ctx context.Context,cfg *config.Config,db *sql.DB,logger *slog.Logger) {
	opts:=[]nats.Option{nats.Name("defta-librairie-cover-import-ocr-worker")};if cfg.NATSUser!=""||cfg.NATSPassword!=""{opts=append(opts,nats.UserInfo(cfg.NATSUser,cfg.NATSPassword))}
	connection,err:=nats.Connect(cfg.NATSURL,opts...);if err!=nil{logger.Error("cover_import_ocr_nats_unavailable","error",err);return};defer connection.Close();stream,err:=connection.JetStream();if err!=nil{logger.Error("cover_import_ocr_jetstream_unavailable","error",err);return}
	if _,err=stream.ConsumerInfo(cfg.NATSImportStream,cfg.NATSOCRConsumer);err!=nil { if _,err=stream.AddConsumer(cfg.NATSImportStream,&nats.ConsumerConfig{Durable:cfg.NATSOCRConsumer,FilterSubject:cfg.NATSOCRSubject,AckPolicy:nats.AckExplicitPolicy,AckWait:cfg.OCRTimeout+10*time.Second,MaxDeliver:cfg.CoverWorkerMaxDeliver});err!=nil{logger.Error("cover_import_ocr_create_consumer_failed","error",err);return} }
	sub,err:=stream.PullSubscribe(cfg.NATSOCRSubject,cfg.NATSOCRConsumer,nats.BindStream(cfg.NATSImportStream),nats.ManualAck());if err!=nil{logger.Error("cover_import_ocr_subscribe_failed","error",err);return}
	store,err:=covers.NewMinIOProcessingStore(cfg.MinIOEndpoint,cfg.MinIOAccessKey,cfg.MinIOSecretKey,cfg.MinIOBucketCovers,cfg.MinIOUseSSL);if err!=nil{logger.Error("cover_import_ocr_store_invalid","error",err);return}
	worker:=services.NewCoverImportOCRService(repositories.NewCoverImportRepository(db),store,ocr.NewTesseract(cfg.OCREngine,cfg.OCRLanguage,cfg.OCRTimeout),cfg.OCREngine,cfg.OCRLanguage)
	for { messages,err:=sub.Fetch(1,nats.MaxWait(time.Second));if errors.Is(err,nats.ErrTimeout){if ctx.Err()!=nil{return};continue};if err!=nil{logger.Warn("cover_import_ocr_fetch_failed","error",err);continue};for _,message:=range messages{var event coverImportOCRMessage;if err=json.Unmarshal(message.Data,&event);err!=nil||event.JobID==""{_ = message.Term();continue};err=worker.Process(ctx,event.JobID);if err==nil||errors.Is(err,repositories.ErrCoverImportJobNotFound)||errors.Is(err,repositories.ErrCoverImportJobState)||errors.Is(err,services.ErrCoverImportOCRFailed){_ = message.Ack();continue};_ = message.NakWithDelay(5*time.Second)} }
}
