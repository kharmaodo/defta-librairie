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
