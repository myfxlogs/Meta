package db

import (
	"context"
)

// Subscription 巡检订阅行（票0030）：事件 kind→派发角色。
type Subscription struct {
	Kind    string
	Role    string
	Enabled bool
}

// SeedSubscription 种一行订阅（同 (kind,role) 行已存在不动——DB 行是活态，
// 与 shell/project 种子同语义）。
func (d *DB) SeedSubscription(ctx context.Context, kind, role string) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO subscription (kind, role, enabled, created_at) VALUES (?,?,1,?)`,
		cargs(kind, role, d.now().UnixNano())...)
	return err
}

// ListSubscriptions 读全部启用订阅（patrol 匹配输入；禁行不返——
// 关闸＝删 enabled 即停，不删行留账）。
func (d *DB) ListSubscriptions(ctx context.Context) ([]*Subscription, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT kind, role, enabled FROM subscription ORDER BY kind, role`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Subscription
	for rows.Next() {
		var s Subscription
		var en int
		if err := rows.Scan(&s.Kind, &s.Role, &en); err != nil {
			return nil, err
		}
		s.Enabled = en != 0
		out = append(out, &s)
	}
	return out, rows.Err()
}
