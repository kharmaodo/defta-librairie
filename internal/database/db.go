package database

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func Init(path string) error {
	var err error
	DB, err = sql.Open("sqlite3", path+"?_journal_mode=WAL&mode=rwc")
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		return err
	}

	log.Printf("Base SQLite connectée → %s", path)
	return nil
}

func Close() {
	if DB != nil {
		DB.Close()
		log.Println("Connexion SQLite fermée")
	}
}