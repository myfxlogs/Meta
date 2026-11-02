package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// FeishuCard 活票卡行（feishu_card 表，票0014/D-31/令#E14）——
// message_id↔票持久：卡状态在库不在 caster 内存（§1.7 决策38 同判推演），
// 网关重启后原地刷新同一张卡。
type FeishuCard struct {
	TicketID  string
	MessageID string
	ChatID    string
	UpdatedAt time.Time
}

// GetFeishuCard 取卡行；无行返回 nil,nil（与 GetTicket 同约）。
func (d *DB) GetFeishuCard(ctx context.Context, ticketID string) (*FeishuCard, error) {
	var c FeishuCard
	var updated int64
	err := d.db.QueryRowContext(ctx,
		`SELECT ticket_id, message_id, chat_id, updated_at
		 FROM feishu_card WHERE ticket_id=?`, ticketID).
		Scan(&c.TicketID, &c.MessageID, &c.ChatID, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.UpdatedAt = time.Unix(0, updated)
	return &c, nil
}

// SaveFeishuCard 落卡行（懒创建成功后 message_id 落库；幂等 upsert——
// 重建/复用同键覆盖，单票单卡钦定）。
func (d *DB) SaveFeishuCard(ctx context.Context, ticketID, messageID, chatID string) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO feishu_card (ticket_id, message_id, chat_id, updated_at)
		 VALUES (?,?,?,?)
		 ON CONFLICT(ticket_id) DO UPDATE SET
		   message_id=excluded.message_id,
		   chat_id=excluded.chat_id,
		   updated_at=excluded.updated_at`,
		ticketID, messageID, chatID, d.now().UnixNano())
	return err
}

// TouchFeishuCard PATCH 成功留痕（T9/D-63）：updated_at 拨今——卡龄追踪真源。
func (d *DB) TouchFeishuCard(ctx context.Context, ticketID string) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE feishu_card SET updated_at=? WHERE ticket_id=?`,
		d.now().UnixNano(), ticketID)
	return err
}
