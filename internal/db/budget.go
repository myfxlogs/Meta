package db

import "context"

// AddUsage 累计票上用量：used_total（票级累计，跨尝试单调递增，§5.5 A6）恒进；
// used（当次尝试）仅 countAttempt=true 才进——adjunct 席不是施工「尝试」，
// 不占尝试帽，否则裁决席用量能把施工席误判过 100% 硬停（§1.3）。
func (d *DB) AddUsage(ctx context.Context, ticketID string, delta int64, countAttempt bool) error {
	var usedDelta int64
	if countAttempt {
		usedDelta = delta
	}
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO budget (ticket_id, used, used_total, updated_at) VALUES (?,?,?,?)
		 ON CONFLICT(ticket_id) DO UPDATE SET
		   used=used+excluded.used, used_total=used_total+excluded.used_total, updated_at=excluded.updated_at`,
		cargs(ticketID, usedDelta, delta, d.now().UnixNano())...)
	return err
}

// EnsureBudgetDefaults 预算缺省落种（D-9/D-27 配置化）：行缺省才写——
// 显式 ticket new --attempt-cap/--hard-cap 或 SetBudget 的行不覆盖。
func (d *DB) EnsureBudgetDefaults(ctx context.Context, ticketID string, cap, hardCap int64) error {
	if cap <= 0 && hardCap <= 0 {
		return nil
	}
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO budget (ticket_id, cap, hard_cap, used, used_total, updated_at)
		 VALUES (?,?,?,0,0,?)
		 ON CONFLICT(ticket_id) DO NOTHING`,
		cargs(ticketID, cap, hardCap, d.now().UnixNano())...)
	return err
}

// IncrementDefer 延期计数 +1（stall 裁决 defer 径），返回新值。
func (d *DB) IncrementDefer(ctx context.Context, ticketID string) (int, error) {
	var n int
	err := d.db.QueryRowContext(ctx,
		`UPDATE ticket SET defer_count=defer_count+1, updated_at=? WHERE id=?
		 RETURNING defer_count`, cargs(d.now().UnixNano(), ticketID)...).Scan(&n)
	return n, err
}

// Usage 取用量（审计面/测试）。
func (d *DB) Usage(ctx context.Context, ticketID string) (used, usedTotal int64, err error) {
	err = d.db.QueryRowContext(ctx,
		`SELECT used, used_total FROM budget WHERE ticket_id=?`, ticketID).Scan(&used, &usedTotal)
	return
}
