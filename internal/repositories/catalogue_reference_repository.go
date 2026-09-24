package repositories

import (
 "context"
 "database/sql"
 "defta-librairie/internal/models"
 "errors"
 "fmt"
)

var (
 ErrCatalogueReferenceNotFound = errors.New("catalogue reference not found")
 ErrCatalogueReferenceConflict = errors.New("catalogue reference conflict")
)

type CatalogueReferenceRepository struct{ db *sql.DB }
func NewCatalogueReferenceRepository(db *sql.DB) *CatalogueReferenceRepository { return &CatalogueReferenceRepository{db: db} }

func referenceTable(kind string) (string, string, error) {
 switch kind {
 case "category": return "categories", "categoriename", nil
 case "publisher": return "publishers", "fullname", nil
 default: return "", "", errors.New("invalid catalogue reference kind")
 }
}
func (r *CatalogueReferenceRepository) List(ctx context.Context, kind string) ([]models.CatalogueReference, error) {
 table,name,err:=referenceTable(kind); if err!=nil{return nil,err}
 rows,err:=r.db.QueryContext(ctx,fmt.Sprintf("SELECT id, code, %s, ar, fr, en, active, COALESCE(created_at,''), COALESCE(updated_at,'') FROM %s ORDER BY code",name,table))
 if err!=nil{return nil,fmt.Errorf("list catalogue references: %w",err)}; defer rows.Close()
 result:=make([]models.CatalogueReference,0)
 for rows.Next(){var v models.CatalogueReference; if err=rows.Scan(&v.ID,&v.Code,&v.Name,&v.Arabic,&v.French,&v.English,&v.Active,&v.CreatedAt,&v.UpdatedAt);err!=nil{return nil,err};result=append(result,v)}
 return result,rows.Err()
}


func (r *CatalogueReferenceRepository) Create(ctx context.Context, kind string, value models.CatalogueReference, actorID, auditID string) error {
	table, name, err := referenceTable(kind); if err != nil { return err }
	tx, err := r.db.BeginTx(ctx, nil); if err != nil { return err }; defer tx.Rollback()
	_, err = tx.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s(code, %s, ar, fr, en, active, created_at, updated_at) VALUES(?, ?, ?, ?, ?, 1, ?, ?)", table, name), value.Code, value.Name, value.Arabic, value.French, value.English, value.CreatedAt, value.UpdatedAt)
	if isUniqueViolation(err) { return ErrCatalogueReferenceConflict }; if err != nil { return err }
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at) VALUES (?, ?, ?, ?, ?, ?, 1, ?)", auditID, actorID, "CREATE_"+kind, "CATALOGUE_REFERENCE", value.Code, value.Name, value.CreatedAt); err != nil { return err }
	return tx.Commit()
}


func (r *CatalogueReferenceRepository) SetActive(ctx context.Context, kind string, id int64, active bool, actorID, auditID, now string) error {
 table,_,err:=referenceTable(kind);if err!=nil{return err}
 tx,err:=r.db.BeginTx(ctx,nil);if err!=nil{return err};defer tx.Rollback()
 result,err:=tx.ExecContext(ctx,fmt.Sprintf("UPDATE %s SET active=?, updated_at=? WHERE id=?",table),active,now,id)
 if err!=nil{return err};n,err:=result.RowsAffected();if err!=nil{return err};if n!=1{return ErrCatalogueReferenceNotFound}
 if _,err=tx.ExecContext(ctx,"INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,?,?,?,?,1,?)",auditID,actorID,"SET_"+kind+"_ACTIVE","CATALOGUE_REFERENCE",fmt.Sprint(id),fmt.Sprint(active),now);err!=nil{return err}
 return tx.Commit()
}


func (r *CatalogueReferenceRepository) Update(ctx context.Context, kind string, id int64, value models.CatalogueReference, actorID, auditID string) error {
 table,name,err:=referenceTable(kind);if err!=nil{return err}
 tx,err:=r.db.BeginTx(ctx,nil);if err!=nil{return err};defer tx.Rollback()
 result,err:=tx.ExecContext(ctx,fmt.Sprintf("UPDATE %s SET code=?, %s=?, ar=?, fr=?, en=?, updated_at=? WHERE id=?",table,name),value.Code,value.Name,value.Arabic,value.French,value.English,value.UpdatedAt,id)
 if isUniqueViolation(err){return ErrCatalogueReferenceConflict};if err!=nil{return err};n,err:=result.RowsAffected();if err!=nil{return err};if n!=1{return ErrCatalogueReferenceNotFound}
 if _,err=tx.ExecContext(ctx,"INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,?,?,?,?,1,?)",auditID,actorID,"UPDATE_"+kind,"CATALOGUE_REFERENCE",fmt.Sprint(id),value.Name,value.UpdatedAt);err!=nil{return err}
 return tx.Commit()
}
