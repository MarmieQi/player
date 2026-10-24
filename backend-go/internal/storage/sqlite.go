package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"golang.org/x/text/cases"
	"modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"player-stats-backend-go/internal/config"
	"strings"
	"sync"
	"time"
)

// DB owns an independent read pool and a single serialized writer. Business
// modules never receive the underlying writable sql.DB.
type DB struct {
	read, write *sql.DB
	zone        *time.Location
}
type Tx struct {
	tx   *sql.Tx
	zone *time.Location
}

var register sync.Once

func Fold(value string) string { return cases.Fold().String(value) }
func Open(ctx context.Context, cfg config.Config) (*DB, error) {
	register.Do(func() {
		sqlite.MustRegisterDeterministicScalarFunction("account_key", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if args[0] == nil {
				return nil, nil
			}
			return Fold(fmt.Sprint(args[0])), nil
		})
		sqlite.MustRegisterDeterministicScalarFunction("lower", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if args[0] == nil {
				return nil, nil
			}
			return Fold(fmt.Sprint(args[0])), nil
		})
	})
	path, err := filepath.Abs(cfg.DatabasePath)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	params := url.Values{"_pragma": {"foreign_keys(1)", "busy_timeout(5000)", "synchronous(FULL)"}, "_txlock": {"immediate"}}
	dsn := (&url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(path), "/")}).String() + "?" + params.Encode()
	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)
	fail := func(err error) (*DB, error) { writer.Close(); return nil, err }
	if err = writer.PingContext(ctx); err != nil {
		return fail(err)
	}
	var mode string
	if err = writer.QueryRowContext(ctx, `pragma journal_mode=WAL`).Scan(&mode); err != nil {
		return fail(err)
	}
	if strings.ToLower(mode) != "wal" {
		return fail(fmt.Errorf("SQLite WAL unavailable: %s", mode))
	}
	_ = os.Chmod(path, 0600)
	params.Del("_txlock")
	params.Add("_pragma", "query_only(1)")
	params.Set("mode", "ro")
	reader, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(path), "/")}).String()+"?"+params.Encode())
	if err != nil {
		return fail(err)
	}
	reader.SetMaxOpenConns(4)
	reader.SetMaxIdleConns(4)
	if err = reader.PingContext(ctx); err != nil {
		reader.Close()
		return fail(err)
	}
	zone := cfg.Location
	if zone == nil {
		zone = time.FixedZone("Asia/Shanghai", 8*3600)
	}
	return &DB{read: reader, write: writer, zone: zone}, nil
}

// WrapReadOnly is for SQL-driver contract tests, not application initialization.
func WrapReadOnly(db *sql.DB) *DB {
	return &DB{read: db, zone: time.FixedZone("Asia/Shanghai", 8*3600)}
}
func (d *DB) Close() error {
	err := d.read.Close()
	if d.write != nil {
		_, _ = d.write.Exec(`pragma wal_checkpoint(TRUNCATE)`)
		if e := d.write.Close(); err == nil {
			err = e
		}
	}
	return err
}
func (d *DB) Check(ctx context.Context) error {
	var result string
	if err := d.QueryRowContext(ctx, `pragma integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("SQLite integrity check: %s", result)
	}
	rows, err := d.QueryContext(ctx, `pragma foreign_key_check`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("SQLite foreign key violation")
	}
	return rows.Err()
}
func (d *DB) Checkpoint(ctx context.Context) error {
	var busy, pages, checkpointed int
	if err := d.write.QueryRowContext(ctx, `pragma wal_checkpoint(TRUNCATE)`).Scan(&busy, &pages, &checkpointed); err != nil {
		return err
	}
	if busy != 0 {
		return fmt.Errorf("SQLite checkpoint blocked: %d of %d WAL pages checkpointed", checkpointed, pages)
	}
	return nil
}
func (d *DB) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return d.write.ExecContext(ctx, q, encodeArgs(args)...)
}
func (d *DB) QueryContext(ctx context.Context, q string, args ...any) (*Rows, error) {
	r, e := d.read.QueryContext(ctx, q, encodeArgs(args)...)
	return &Rows{Rows: r, zone: d.zone}, e
}
func (d *DB) QueryRowContext(ctx context.Context, q string, args ...any) *Row {
	return &Row{row: d.read.QueryRowContext(ctx, q, encodeArgs(args)...), zone: d.zone}
}
func (d *DB) QueryRow(q string, args ...any) *Row {
	return d.QueryRowContext(context.Background(), q, args...)
}
func (d *DB) BeginTx(ctx context.Context, _ *sql.TxOptions) (*Tx, error) {
	tx, err := d.write.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{tx: tx, zone: d.zone}, nil
}
func (d *DB) Write(ctx context.Context, fn func(*Tx) error) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func (t *Tx) Commit() error   { return t.tx.Commit() }
func (t *Tx) Rollback() error { return t.tx.Rollback() }
func (t *Tx) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, q, encodeArgs(args)...)
}
func (t *Tx) QueryContext(ctx context.Context, q string, args ...any) (*Rows, error) {
	r, e := t.tx.QueryContext(ctx, q, encodeArgs(args)...)
	return &Rows{Rows: r, zone: t.zone}, e
}
func (t *Tx) QueryRowContext(ctx context.Context, q string, args ...any) *Row {
	return &Row{row: t.tx.QueryRowContext(ctx, q, encodeArgs(args)...), zone: t.zone}
}
