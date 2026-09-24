package repositories

import (
 "context"
 "database/sql"
 "path/filepath"
 "testing"
 _ "github.com/mattn/go-sqlite3"
)

func TestBookTaxonomyRepositoryReplacesCategories(t *testing.T){
 db,err:=sql.Open("sqlite3",filepath.Join(t.TempDir(),"taxonomy.db"));if err!=nil{t.Fatal(err)};defer db.Close()
 if _,err=db.Exec(`CREATE TABLE book_categories(book_id INTEGER,category_id INTEGER,is_primary INTEGER,created_at TEXT,PRIMARY KEY(book_id,category_id)); CREATE TABLE defta(id INTEGER PRIMARY KEY,publisher_id INTEGER); INSERT INTO defta(id) VALUES(1);`);err!=nil{t.Fatal(err)}
 r:=NewBookTaxonomyRepository(db);primary:=2
 if err=r.ReplaceCategories(context.Background(),1,[]int{2,3},&primary,"now");err!=nil{t.Fatal(err)}
 ids,err:=r.Categories(context.Background(),1);if err!=nil{t.Fatal(err)}
 if len(ids)!=2||ids[0]!=2||ids[1]!=3{t.Fatalf("ids=%v",ids)}
 if err=r.ReplaceCategories(context.Background(),1,[]int{4},nil,"later");err!=nil{t.Fatal(err)}
 ids,err=r.Categories(context.Background(),1);if err!=nil||len(ids)!=1||ids[0]!=4{t.Fatalf("ids=%v err=%v",ids,err)}
}
