package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Migrate(ctx context.Context, db *DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY,checksum TEXT NOT NULL,applied_at INTEGER NOT NULL) STRICT`)
	if err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		body, _ := migrations.ReadFile("migrations/" + entry.Name())
		sum := sha256.Sum256(body)
		hash := hex.EncodeToString(sum[:])
		var previous string
		err := db.QueryRowContext(ctx, `select checksum from schema_migrations where version=?`, entry.Name()).Scan(&previous)
		if err == nil {
			if previous != hash {
				return fmt.Errorf("migration checksum mismatch: %s", entry.Name())
			}
			continue
		}
		if err != sql.ErrNoRows {
			return err
		}
		if err = db.Write(ctx, func(tx *Tx) error {
			if _, e := tx.ExecContext(ctx, string(body)); e != nil {
				return e
			}
			_, e := tx.ExecContext(ctx, `insert into schema_migrations values(?,?,?)`, entry.Name(), hash, time.Now())
			return e
		}); err != nil {
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}
