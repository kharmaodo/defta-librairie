package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"errors"
	"fmt"
)

var (
	ErrCoverImportNotFound = errors.New("cover import not found")
	ErrCoverImportQuota = errors.New("cover import quota exceeded")
	ErrInvalidCoverImport = errors.New("invalid cover import")
)

type PendingCoverImport struct { ID, LibraryID, ActorUserID, IdempotencyKey string; Jobs []PendingCoverImportJob }
type PendingCoverImportJob struct { ID, SourceObjectKey, SourceContentType, SourceFormat, InputSHA256, EventID, Payload string; SourceWidth, SourceHeight int; SourceSize int64; ExpiresAt string }

type CoverImportRepository struct { db *sql.DB }
func NewCoverImportRepository(db *sql.DB) *CoverImportRepository { return &CoverImportRepository{db: db} }

// CreatePending writes the batch, all jobs, outbox messages and audit record in one transaction.
func (r *CoverImportRepository) CreatePending(ctx context.Context, value PendingCoverImport, auditID, now, weeklySince string) error {
	if value.ID=="" || value.LibraryID=="" || value.ActorUserID=="" || value.IdempotencyKey=="" || len(value.Jobs)<1 || len(value.Jobs)>100 { return ErrInvalidCoverImport }
	tx, err := r.db.BeginTx(ctx,nil); if err != nil { return fmt.Errorf("begin cover import: %w",err) }; defer tx.Rollback()
	var concurrent, weekly int
	if err=tx.QueryRowContext(ctx,`SELECT COUNT(*) FROM cover_imports WHERE library_id=? AND status IN ('PENDING','PROCESSING')`,value.LibraryID).Scan(&concurrent);err!=nil{return fmt.Errorf("count active imports: %w",err)}
	if concurrent>=2{return ErrCoverImportQuota}
	if err=tx.QueryRowContext(ctx,`SELECT COUNT(*) FROM cover_import_jobs WHERE library_id=? AND created_at>=?`,value.LibraryID,weeklySince).Scan(&weekly);err!=nil{return fmt.Errorf("count weekly imports: %w",err)}
	if weekly+len(value.Jobs)>500{return ErrCoverImportQuota}
	if _,err=tx.ExecContext(ctx,`INSERT INTO cover_imports(id,library_id,actor_user_id,idempotency_key,status,total_files,created_at,updated_at) VALUES(?,?,?,?,'PENDING',?,?,?)`,value.ID,value.LibraryID,value.ActorUserID,value.IdempotencyKey,len(value.Jobs),now,now);err!=nil{return fmt.Errorf("insert cover import: %w",err)}
	for _, job := range value.Jobs {
		if job.ID==""||job.EventID==""||job.SourceObjectKey==""||len(job.InputSHA256)!=64||job.SourceWidth<1||job.SourceHeight<1||job.SourceSize<1||job.ExpiresAt=="" {return ErrInvalidCoverImport}
		if _,err=tx.ExecContext(ctx,`INSERT INTO cover_import_jobs(id,import_id,library_id,actor_user_id,source_object_key,source_content_type,source_format,source_width,source_height,source_size,input_sha256,status,expires_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,'PENDING_SCAN',?,?,?)`,job.ID,value.ID,value.LibraryID,value.ActorUserID,job.SourceObjectKey,job.SourceContentType,job.SourceFormat,job.SourceWidth,job.SourceHeight,job.SourceSize,job.InputSHA256,job.ExpiresAt,now,now);err!=nil{return fmt.Errorf("insert cover import job: %w",err)}
		if _,err=tx.ExecContext(ctx,`INSERT INTO cover_import_outbox(event_id,job_id,library_id,event_type,schema_version,payload,attempts,available_at,created_at) VALUES(?,?,?,'cover.imports.moderate.v1',1,?,0,?,?)`,job.EventID,job.ID,value.LibraryID,job.Payload,now,now);err!=nil{return fmt.Errorf("insert cover import outbox: %w",err)}
	}
	if _,err=tx.ExecContext(ctx,`INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?, 'CREATE_COVER_IMPORT','COVER_IMPORT',?, ?,1,?)`,auditID,value.ActorUserID,value.ID,fmt.Sprintf(`{"files":%d}`,len(value.Jobs)),now);err!=nil{return fmt.Errorf("audit cover import: %w",err)}
	if err=tx.Commit();err!=nil{return fmt.Errorf("commit cover import: %w",err)};return nil
}

func (r *CoverImportRepository) FindByIdempotencyKey(ctx context.Context, libraryID, key string) (models.CoverImport, error) {
	if libraryID=="" || key=="" { return models.CoverImport{}, ErrInvalidCoverImport }
	var item models.CoverImport; var completed sql.NullString
	err:=r.db.QueryRowContext(ctx,`SELECT id,library_id,status,total_files,accepted_files,rejected_files,created_at,updated_at,completed_at FROM cover_imports WHERE library_id=? AND idempotency_key=?`,libraryID,key).Scan(&item.ID,&item.LibraryID,&item.Status,&item.TotalFiles,&item.AcceptedFiles,&item.RejectedFiles,&item.CreatedAt,&item.UpdatedAt,&completed)
	if errors.Is(err,sql.ErrNoRows){return models.CoverImport{},ErrCoverImportNotFound};if err!=nil{return models.CoverImport{},fmt.Errorf("find cover import idempotency key: %w",err)};if completed.Valid{item.CompletedAt=&completed.String};return item,nil
}

func (r *CoverImportRepository) List(ctx context.Context, libraryID string, limit int) ([]models.CoverImport,error) {
	if libraryID=="" {return nil,ErrInvalidCoverImport}; if limit<1 {limit=30};if limit>100 {limit=100}
	rows,err:=r.db.QueryContext(ctx,`SELECT id,library_id,status,total_files,accepted_files,rejected_files,created_at,updated_at,completed_at FROM cover_imports WHERE library_id=? ORDER BY created_at DESC LIMIT ?`,libraryID,limit);if err!=nil{return nil,fmt.Errorf("list cover imports: %w",err)};defer rows.Close()
	result:=[]models.CoverImport{};for rows.Next(){var item models.CoverImport;var completed sql.NullString;if err=rows.Scan(&item.ID,&item.LibraryID,&item.Status,&item.TotalFiles,&item.AcceptedFiles,&item.RejectedFiles,&item.CreatedAt,&item.UpdatedAt,&completed);err!=nil{return nil,fmt.Errorf("scan cover import: %w",err)};if completed.Valid{item.CompletedAt=&completed.String};result=append(result,item)};return result,rows.Err()
}
