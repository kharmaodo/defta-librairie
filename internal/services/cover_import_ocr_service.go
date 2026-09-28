package services

import (
	"context"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/ocr"
	"defta-librairie/internal/repositories"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var ErrCoverImportOCRFailed = errors.New("cover import OCR failed")

type coverImportOCRSourceReader interface { OpenSource(context.Context, string) (io.ReadCloser, error) }

type CoverImportOCRService struct {
	repository *repositories.CoverImportRepository
	sources    coverImportOCRSourceReader
	runner     ocr.Runner
	engine     string
	language   string
	newID      func() (string, error)
	now        func() time.Time
}

func NewCoverImportOCRService(repository *repositories.CoverImportRepository, sources coverImportOCRSourceReader, runner ocr.Runner, engine, language string) *CoverImportOCRService {
	return &CoverImportOCRService{repository:repository,sources:sources,runner:runner,engine:engine,language:language,newID:identity.NewID,now:time.Now}
}

func (s *CoverImportOCRService) Process(ctx context.Context, jobID string) error {
	if s==nil || s.repository==nil || s.sources==nil || s.runner==nil || s.newID==nil || jobID=="" || s.language!="ara" { return ErrCoverImportOCRFailed }
	now:=s.now().UTC().Format(time.RFC3339Nano)
	job,err:=s.repository.ClaimForOCR(ctx,jobID,now);if err!=nil{return err}
	source,err:=s.sources.OpenSource(ctx,job.SourceObjectKey);if err!=nil{return s.fail(ctx,job.ID,"SOURCE_UNAVAILABLE",err)};defer source.Close()
	image,err:=io.ReadAll(io.LimitReader(source,job.SourceSize+1));if err!=nil||int64(len(image))!=job.SourceSize{if err==nil{err=errors.New("unexpected quarantined source size")};return s.fail(ctx,job.ID,"SOURCE_INVALID",err)}
	raw,err:=s.runner.Run(ctx,image,job.SourceContentType);if err!=nil { code:="OCR_UNAVAILABLE";if errors.Is(err,ocr.ErrTimeout){code="OCR_TIMEOUT"};return s.fail(ctx,job.ID,code,err) }
	eventID,err:=s.newID();if err!=nil{return s.fail(ctx,job.ID,"WORKER_ID_FAILURE",err)}
	auditID,err:=s.newID();if err!=nil{return s.fail(ctx,job.ID,"WORKER_ID_FAILURE",err)}
	if err=s.repository.CompleteOCR(ctx,job,s.engine,"tesseract-5",s.language,raw,normalizeOCR(raw),eventID,auditID,s.now().UTC().Format(time.RFC3339Nano));err!=nil{return err};return nil
}

func normalizeOCR(raw string) string { return strings.Join(strings.Fields(raw), " ") }

func (s *CoverImportOCRService) fail(ctx context.Context, jobID, code string, cause error) error { auditID,err:=s.newID();if err==nil{err=s.repository.FailOCR(ctx,jobID,code,auditID,s.now().UTC().Format(time.RFC3339Nano))};if err!=nil{return errors.Join(fmt.Errorf("process cover import OCR: %w",cause),err)};return fmt.Errorf("%w: %w",ErrCoverImportOCRFailed,cause) }
