package db

import (
	"context"
	"time"
)

// Memory ceo_memory 行（§3.2/§8：CEO 记忆在 meta 库）。
type Memory struct {
	ID         int64
	Kind       string
	Title      string
	Content    []byte
	EventRef   string // put 时机械盖 run_id——D-22「事件数 vs 写数」比对基准
	Provenance string // 盖自 run 行（D9）
	CreatedAt  time.Time
}

// PutMemory 落一条 CEO 记忆；eventRef/provenance 由网关盖（席不自报）。
func (d *DB) PutMemory(ctx context.Context, kind, title string, content []byte, eventRef, provenance string) (int64, error) {
	res, err := d.db.ExecContext(ctx,
		`INSERT INTO ceo_memory (kind, title, content, event_ref, provenance, created_at)
		 VALUES (?,?,?,?,?,?)`,
		kind, title, content, eventRef, provenance, d.now().UnixNano())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListMemory 按 kind 倒序读（kind 空＝全量）；limit<=0 回缺省 200。
func (d *DB) ListMemory(ctx context.Context, kind string, limit int) ([]*Memory, error) {
	if limit <= 0 {
		limit = 200
	}
	q := `SELECT id, kind, title, content, event_ref, provenance, created_at FROM ceo_memory`
	args := []any{}
	if kind != "" {
		q += ` WHERE kind=?`
		args = append(args, kind)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Memory
	for rows.Next() {
		var m Memory
		var created int64
		if err := rows.Scan(&m.ID, &m.Kind, &m.Title, &m.Content,
			&m.EventRef, &m.Provenance, &created); err != nil {
			return nil, err
		}
		m.CreatedAt = time.Unix(0, created)
		out = append(out, &m)
	}
	return out, rows.Err()
}

// CountMemoryByEventRef 某 run 的写入计数（D-22：CEO run 收口机械比对——
// 处理事件数 vs 其 put 写数，零写入且处理过事件→ceo_gap）。
func (d *DB) CountMemoryByEventRef(ctx context.Context, eventRef string) (int, error) {
	var n int
	err := d.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ceo_memory WHERE event_ref=?`, eventRef).Scan(&n)
	return n, err
}

// MemoryHas 幂等导入判据：kind+title 已有行（import-legacy 二跑零增靠它）。
func (d *DB) MemoryHas(ctx context.Context, kind, title string) (bool, error) {
	var n int
	err := d.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ceo_memory WHERE kind=? AND title=?`, kind, title).Scan(&n)
	return n > 0, err
}
