package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	CoverImportMaxFiles = 100
	CoverImportMaxBytes int64 = 500 * 1024 * 1024
	CoverImportWeeklyLimit = 500
)

type CoverImportFile struct { DeclaredContentType string; Body io.Reader }

type CoverImportService struct { enabled bool; books *BookService; repository *repositories.CoverImportRepository; uploader *covers.SourceUploader; newID covers.IDGenerator; now func() time.Time }
func NewCoverImportService(enabled bool, books *BookService, repository *repositories.CoverImportRepository, uploader *covers.SourceUploader) *CoverImportService { return &CoverImportService{enabled:enabled,books:books,repository:repository,uploader:uploader,newID:identity.NewID,now:time.Now} }

func (s *CoverImportService) Create(ctx context.Context, claims *auth.Claims, requestedLibrary, idempotencyKey string, files []CoverImportFile) (models.CoverImport,error) {
	if !s.enabled||s.books==nil||s.repository==nil||s.uploader==nil||s.newID==nil{return models.CoverImport{},ErrCoversDisabled}
	if len(files)<1||len(files)>CoverImportMaxFiles{return models.CoverImport{},ErrInvalidBook}
	libraryID,err:=resolveBookScope(claims,requestedLibrary,true);if err!=nil{return models.CoverImport{},err};idempotencyKey=strings.TrimSpace(idempotencyKey);if idempotencyKey==""||len(idempotencyKey)>128{return models.CoverImport{},ErrInvalidBook}
	if err=s.books.ensureOwnerLibraryActive(ctx,claims,libraryID);err!=nil{return models.CoverImport{},err}
	if existing,findErr:=s.repository.FindByIdempotencyKey(ctx,libraryID,idempotencyKey);findErr==nil{return existing,nil}else if !errors.Is(findErr,repositories.ErrCoverImportNotFound){return models.CoverImport{},findErr}
	importID,err:=s.newID();if err!=nil{return models.CoverImport{},err}
	now:=s.now().UTC(); pending:=repositories.PendingCoverImport{ID:importID,LibraryID:libraryID,ActorUserID:claims.Subject,IdempotencyKey:idempotencyKey,Jobs:make([]repositories.PendingCoverImportJob,0,len(files))}; stored:=make([]covers.StoredSource,0,len(files))
	rollback:=func(cause error) error { for _,source:=range stored { if discardErr:=s.uploader.Discard(ctx,source);discardErr!=nil { cause=errors.Join(cause,fmt.Errorf("discard cover import source: %w",discardErr)) } };return fmt.Errorf("%w: %v",ErrCoverPersistence,cause) }
	for _,file:=range files {
		jobID,idErr:=s.newID();if idErr!=nil{return models.CoverImport{},rollback(idErr)}
		source,uploadErr:=s.uploader.UploadImport(ctx,libraryID,importID,jobID,file.DeclaredContentType,file.Body);if uploadErr!=nil{return models.CoverImport{},rollback(uploadErr)};stored=append(stored,source)
		eventID,idErr:=s.newID();if idErr!=nil{return models.CoverImport{},rollback(idErr)}
		payload,marshalErr:=json.Marshal(map[string]any{"schemaVersion":1,"eventId":eventID,"jobId":jobID,"importId":importID,"libraryId":libraryID,"sourceObjectKey":source.ObjectKey,"attempt":1});if marshalErr!=nil{return models.CoverImport{},rollback(marshalErr)}
		pending.Jobs=append(pending.Jobs,repositories.PendingCoverImportJob{ID:jobID,SourceObjectKey:source.ObjectKey,SourceContentType:source.ContentType,SourceFormat:source.Format,InputSHA256:source.SHA256,SourceWidth:source.Width,SourceHeight:source.Height,SourceSize:source.Size,EventID:eventID,Payload:string(payload),ExpiresAt:now.Add(30*24*time.Hour).Format(time.RFC3339Nano)})
	}
	auditID,err:=s.newID();if err!=nil{return models.CoverImport{},rollback(err)}
	if err=s.repository.CreatePending(ctx,pending,auditID,now.Format(time.RFC3339Nano),now.Add(-7*24*time.Hour).Format(time.RFC3339Nano));err!=nil{
		if existing,findErr:=s.repository.FindByIdempotencyKey(ctx,libraryID,idempotencyKey);findErr==nil{
			for _,source:=range stored { _=s.uploader.Discard(ctx,source) }
			return existing,nil
		}
		return models.CoverImport{},rollback(err)
	}
	return models.CoverImport{ID:importID,LibraryID:libraryID,Status:"PENDING",TotalFiles:len(files),CreatedAt:now.Format(time.RFC3339Nano),UpdatedAt:now.Format(time.RFC3339Nano)},nil
}

func (s *CoverImportService) List(ctx context.Context,claims *auth.Claims,requestedLibrary string,limit int)([]models.CoverImport,error){
	if !s.enabled||s.repository==nil{return nil,ErrCoversDisabled};libraryID,err:=resolveBookScope(claims,requestedLibrary,false);if err!=nil{return nil,err};if err=s.books.ensureOwnerLibraryActive(ctx,claims,libraryID);err!=nil{return nil,err};return s.repository.List(ctx,libraryID,limit)
}
