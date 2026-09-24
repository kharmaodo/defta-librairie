package repositories

import (
 "context"
 "database/sql"
 "fmt"
)

type BookTaxonomyRepository struct{ db *sql.DB }
func NewBookTaxonomyRepository(db *sql.DB)*BookTaxonomyRepository{return &BookTaxonomyRepository{db:db}}

func (r *BookTaxonomyRepository) ReplaceCategories(ctx context.Context,bookID int,categoryIDs []int,primaryID *int,now string)error{
 tx,err:=r.db.BeginTx(ctx,nil);if err!=nil{return err};defer tx.Rollback()
 if _,err=tx.ExecContext(ctx,"DELETE FROM book_categories WHERE book_id=?",bookID);err!=nil{return err}
 for _,id:=range categoryIDs{primary:=0;if primaryID!=nil&&id==*primaryID{primary=1};if _,err=tx.ExecContext(ctx,"INSERT INTO book_categories(book_id,category_id,is_primary,created_at) VALUES(?,?,?,?)",bookID,id,primary,now);err!=nil{return err}}
 return tx.Commit()
}
func (r *BookTaxonomyRepository) Categories(ctx context.Context,bookID int)([]int,error){
 rows,err:=r.db.QueryContext(ctx,"SELECT category_id FROM book_categories WHERE book_id=? ORDER BY is_primary DESC,category_id",bookID);if err!=nil{return nil,err};defer rows.Close();ids:=[]int{};for rows.Next(){var id int;if err=rows.Scan(&id);err!=nil{return nil,err};ids=append(ids,id)};return ids,rows.Err()
}
func (r *BookTaxonomyRepository) SetPublisher(ctx context.Context,bookID int,publisherID *int)error{
 var value interface{}=nil;if publisherID!=nil{value=*publisherID};result,err:=r.db.ExecContext(ctx,"UPDATE defta SET publisher_id=? WHERE id=?",value,bookID);if err!=nil{return err};n,err:=result.RowsAffected();if err!=nil{return err};if n!=1{return fmt.Errorf("book not found")};return nil
}
