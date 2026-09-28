package services

import (
	"context"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/moderation"
	"defta-librairie/internal/repositories"
	"errors"
	"fmt"
	"io"
	"time"
)

type coverImportSourceReader interface { OpenSource(context.Context, string) (io.ReadCloser, error) }

// CoverImportModerationService only records a moderation decision. It never
// creates or promotes a book cover; SAFE is queued for OCR and human review.
type CoverImportModerationService struct { repository *repositories.CoverImportRepository; sources coverImportSourceReader; client moderation.Client; newID func()(string,error); now func()time.Time }
func NewCoverImportModerationService(repository *repositories.CoverImportRepository,sources coverImportSourceReader,client moderation.Client)*CoverImportModerationService{return &CoverImportModerationService{repository:repository,sources:sources,client:client,newID:identity.NewID,now:time.Now}}
func(s *CoverImportModerationService)Process(ctx context.Context,jobID string)error{
	if s==nil||s.repository==nil||s.sources==nil||s.client==nil||s.newID==nil||jobID==""{return ErrInvalidModerationWorker}
	job,err:=s.repository.ClaimForModeration(ctx,jobID,s.now().UTC().Format(time.RFC3339Nano));if err!=nil{return err}
	source,err:=s.sources.OpenSource(ctx,job.SourceObjectKey);if err!=nil{return s.fail(ctx,job.ID,"SOURCE_UNAVAILABLE",err)};defer source.Close()
	image,err:=io.ReadAll(io.LimitReader(source,job.SourceSize+1));if err!=nil||int64(len(image))!=job.SourceSize{if err==nil{err=errors.New("unexpected quarantined source size")};return s.fail(ctx,job.ID,"SOURCE_INVALID",err)}
	result,err:=s.client.Moderate(ctx,job.SourceContentType,image);if err!=nil{return s.fail(ctx,job.ID,"MODERATOR_UNAVAILABLE",err)}
	auditID,err:=s.newID();if err!=nil{return s.fail(ctx,job.ID,"WORKER_ID_FAILURE",err)}
	eventID:="";if result.Class=="SAFE"{eventID,err=s.newID();if err!=nil{return s.fail(ctx,job.ID,"WORKER_ID_FAILURE",err)}}
	if err=s.repository.CompleteModeration(ctx,job,result.Class,result.ModelVersion,eventID,auditID,s.now().UTC().Format(time.RFC3339Nano));err!=nil{return err};return nil
}
func(s *CoverImportModerationService)fail(ctx context.Context,jobID,code string,cause error)error{auditID,err:=s.newID();if err==nil{err=s.repository.FailModeration(ctx,jobID,code,auditID,s.now().UTC().Format(time.RFC3339Nano))};if err!=nil{return errors.Join(fmt.Errorf("moderate cover import: %w",cause),err)};return fmt.Errorf("%w: %w",ErrBookSubmissionModerationFailed,cause)}
var _ coverImportSourceReader=(*covers.MinIOProcessingStore)(nil)
