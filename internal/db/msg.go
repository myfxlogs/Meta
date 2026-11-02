package db

import (
	"context"
	"time"
)

// Msg 票内留言行（ticket_msg，票0030 续传）：run_id 空＝未投递，
// 非空＝已投递给该 run（直投 feed 或注入前缀带走）。
type Msg struct {
	ID        int64
	TicketID  string
	Actor     string
	Text      string
	RunID     string
	CreatedAt time.Time
}

// AppendMsg 落一条票内留言，返回行 id。
func (d *DB) AppendMsg(ctx context.Context, ticketID, actor, text string) (int64, error) {
	res, err := d.db.ExecContext(ctx,
		`INSERT INTO ticket_msg (ticket_id, actor, text, created_at) VALUES (?,?,?,?)`,
		cargs(ticketID, actor, text, d.now().UnixNano())...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// MsgsForTicket 取票内留言（注入前缀/续传提示输入）。pendingOnly=true
// 只取未投递行（run_id=''）。
func (d *DB) MsgsForTicket(ctx context.Context, ticketID string, pendingOnly bool) ([]*Msg, error) {
	q := `SELECT id, ticket_id, actor, text, run_id, created_at FROM ticket_msg WHERE ticket_id=?`
	if pendingOnly {
		q += ` AND run_id=''`
	}
	rows, err := d.db.QueryContext(ctx, q+` ORDER BY id`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Msg
	for rows.Next() {
		var m Msg
		var created int64
		if err := rows.Scan(&m.ID, &m.TicketID, &m.Actor, &m.Text, &m.RunID, &created); err != nil {
			return nil, err
		}
		m.CreatedAt = time.Unix(0, created)
		out = append(out, &m)
	}
	return out, rows.Err()
}

// MarkMsgDelivered 盖投递戳（feed 直投成功或消息随提示文件带走）。
func (d *DB) MarkMsgDelivered(ctx context.Context, id int64, runID string) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE ticket_msg SET run_id=? WHERE id=?`, cargs(runID, id)...)
	return err
}
