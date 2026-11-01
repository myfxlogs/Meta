// Package db 是 meta 网关的 SQLite 嵌入层：九表 schema、WAL、
// 票相位边表机、VACUUM INTO 备份。网关是唯一写者（§3.3）。
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB 包装单写者连接。所有状态变更经网关 gRPC 进来，事件就是调用本身。
type DB struct {
	db *sql.DB
	// now 可注入（测试用），缺省 time.Now。
	now func() time.Time
}

// Open 打开（不存在则建）库文件，开 WAL，跑迁移。
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	sq, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	// 单写者进程↔嵌入式单写者库：一条连接，天然串行化写径。
	sq.SetMaxOpenConns(1)
	if _, err := sq.Exec(schema); err != nil {
		sq.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	// 幂等补列迁移：旧库 seat_run 可能缺 adjunct（票0004 增列）与
	// provenance（票0011 增列——D9 来源标盖戳位）。
	if err := ensureColumn(sq, "seat_run", "adjunct", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		sq.Close()
		return nil, fmt.Errorf("migrate adjunct: %w", err)
	}
	// 幂等补列迁移：旧库 project 可能缺 integrate_cmd（票0010 增列）。
	if err := ensureColumn(sq, "project", "integrate_cmd", "TEXT NOT NULL DEFAULT ''"); err != nil {
		sq.Close()
		return nil, fmt.Errorf("migrate integrate_cmd: %w", err)
	}
	if err := ensureColumn(sq, "seat_run", "provenance", "TEXT NOT NULL DEFAULT ''"); err != nil {
		sq.Close()
		return nil, fmt.Errorf("migrate provenance: %w", err)
	}
	for _, f := range []string{path, path + "-wal", path + "-shm"} {
		_ = os.Chmod(f, 0o600) // 库含密文与票真源，仅属主可读写
	}
	return &DB{db: sq, now: time.Now}, nil
}

// SetClock 注入时钟（测试用）。
func (d *DB) SetClock(now func() time.Time) { d.now = now }

// Close 关库。
func (d *DB) Close() error { return d.db.Close() }

// Backup VACUUM INTO 热备份——WAL 库上直接拷文件会撕裂，必须走 VACUUM（§3.2 B7）。
func (d *DB) Backup(dest string) error {
	if _, err := d.db.ExecContext(context.Background(), `VACUUM INTO ?`, dest); err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}
	return nil
}

// TableNames 列出现有表名（迁移烟测用）。
func (d *DB) TableNames() ([]string, error) {
	rows, err := d.db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// JournalMode 返回当前 journal_mode（WAL 烟测用）。
func (d *DB) JournalMode() (string, error) {
	var m string
	if err := d.db.QueryRow(`PRAGMA journal_mode`).Scan(&m); err != nil {
		return "", err
	}
	return m, nil
}

// ensureColumn 幂等 ALTER：列缺才 ADD COLUMN（sqlite 无 IF NOT EXISTS 列语法）。
func ensureColumn(sq *sql.DB, table, col, decl string) error {
	rows, err := sq.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == col {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = sq.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + col + ` ` + decl)
	return err
}
