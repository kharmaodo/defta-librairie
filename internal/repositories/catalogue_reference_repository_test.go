package repositories

import (
 "context"
 "database/sql"
 "path/filepath"
 "testing"
 _ "github.com/mattn/go-sqlite3"
)

func TestCatalogueReferenceRepositoryListsSeedShape(t *testing.T) {
 db,err:=sql.Open("sqlite3",filepath.Join(t.TempDir(),"catalogue.db"));if err!=nil{t.Fatal(err)};defer db.Close()
 if _,err=db.Exec(`CREATE TABLE categories(id INTEGER PRIMARY KEY,code TEXT, categoriename TEXT,ar TEXT,fr TEXT,en TEXT,active INTEGER,created_at TEXT,updated_at TEXT); INSERT INTO categories VALUES(1,'fiqh','fqh','الفقه','Fiqh','Fiqh',1,'','');`);err!=nil{t.Fatal(err)}
 repo:=NewCatalogueReferenceRepository(db)
 values,err:=repo.List(context.Background(),"category");if err!=nil{t.Fatal(err)}
 if len(values)!=1||values[0].Code!="fiqh"||!values[0].Active{t.Fatalf("values=%+v",values)}
}
