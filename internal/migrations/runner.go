package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

//go:embed sql/*.sql
var migrationFiles embed.FS

const migrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    checksum TEXT NOT NULL,
    applied_at TEXT NOT NULL
)`

// Run applique dans l'ordre toutes les migrations encore absentes. Une
// migration déjà appliquée ne peut pas être modifiée silencieusement : son
// checksum est vérifié à chaque démarrage.
func Run(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, migrationsTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "sql")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		if err := apply(ctx, db, entry.Name()); err != nil {
			return err
		}
	}

	return nil
}

func apply(ctx context.Context, db *sql.DB, name string) error {
	content, err := migrationFiles.ReadFile("sql/" + name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	version := strings.SplitN(name, "_", 2)[0]
	sum := sha256.Sum256(content)
	checksum := hex.EncodeToString(sum[:])

	var storedChecksum string
	err = db.QueryRowContext(ctx,
		"SELECT checksum FROM schema_migrations WHERE version = ?", version,
	).Scan(&storedChecksum)
	if err == nil {
		if storedChecksum != checksum {
			return fmt.Errorf("migration %s checksum mismatch", name)
		}
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("check migration %s: %w", name, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	if _, err = tx.ExecContext(ctx,
		"INSERT INTO schema_migrations(version, name, checksum, applied_at) VALUES (?, ?, ?, ?)",
		version, name, checksum, time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}

	return nil
}
