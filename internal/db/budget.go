package db

import "context"

// AddUsage 累计票上用量：used（当次尝试）＋used_total（票级累计，跨尝试单调
// 递增，§5.5 A6）。闸在阶段 0 不启用——只累计，不判顶。
func (d *DB) AddUsage(ctx context.Context, ticketID string, delta int64) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO budget (ticket_id, used, used_total, updated_at) VALUES (?,?,?,?)
		 ON CONFLICT(ticket_id) DO UPDATE SET
		   used=used+excluded.used, used_total=used_total+excluded.used_total, updated_at=excluded.updated_at`,
		ticketID, delta, delta, d.now().UnixNano())
	return err
}

// Usage 取用量（审计面/测试）。
func (d *DB) Usage(ctx context.Context, ticketID string) (used, usedTotal int64, err error) {
	err = d.db.QueryRowContext(ctx,
		`SELECT used, used_total FROM budget WHERE ticket_id=?`, ticketID).Scan(&used, &usedTotal)
	return
}
