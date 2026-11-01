package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Ticket 票行。
type Ticket struct {
	ID           string
	Title        string
	Phase        Phase
	Project      string
	DedupeKey    string
	BudgetFrozen bool
	RejectCount  int
	DeferCount   int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Event 票事件行——事件就是调用本身，ticket_event 是持久日志（§3.3）。
type Event struct {
	ID         int64
	TicketID   string
	Kind       string
	FromPhase  Phase
	ToPhase    Phase
	Actor      string
	Note       string
	Provenance string
	CreatedAt  time.Time
}

// CreateTicket 立票（初始相位 已立）。
func (d *DB) CreateTicket(ctx context.Context, id, title string, spec []byte) error {
	now := d.now().UnixNano()
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO ticket (id, title, phase, spec, created_at, updated_at) VALUES (?,?,?,?,?,?)`,
		id, title, string(PhaseCreated), spec, now, now)
	if err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	return d.appendEvent(ctx, id, "created", "", PhaseCreated, "gateway", "", "")
}

// GetTicket 取票。
func (d *DB) GetTicket(ctx context.Context, id string) (*Ticket, error) {
	var t Ticket
	var phase string
	var created, updated int64
	var frozen int
	err := d.db.QueryRowContext(ctx,
		`SELECT id, title, phase, project, dedupe_key, budget_frozen, reject_count, defer_count, created_at, updated_at
		 FROM ticket WHERE id=?`, id).
		Scan(&t.ID, &t.Title, &phase, &t.Project, &t.DedupeKey, &frozen,
			&t.RejectCount, &t.DeferCount, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Phase = Phase(phase)
	t.BudgetFrozen = frozen != 0
	t.CreatedAt = time.Unix(0, created)
	t.UpdatedAt = time.Unix(0, updated)
	return &t, nil
}

// PhaseOf 取当前相位。
func (d *DB) PhaseOf(ctx context.Context, id string) (Phase, error) {
	var p string
	err := d.db.QueryRowContext(ctx, `SELECT phase FROM ticket WHERE id=?`, id).Scan(&p)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("ticket %s 不存在", id)
	}
	return Phase(p), err
}

// Transition 相位迁移：边表核对 → 单事务内改相位＋记 ticket_event。
// 越边即拒（ErrIllegalEdge）。驳回累 reject_count；Yield/驳回理由入事件（判决史）。
func (d *DB) Transition(ctx context.Context, id string, to Phase, kind EdgeKind, actor, note string) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var from string
	if err := tx.QueryRowContext(ctx, `SELECT phase FROM ticket WHERE id=?`, id).Scan(&from); err != nil {
		return fmt.Errorf("transition read: %w", err)
	}
	if !Can(Phase(from), to, kind) {
		return &ErrIllegalEdge{From: Phase(from), To: to, Kind: kind}
	}
	now := d.now().UnixNano()
	if _, err := tx.ExecContext(ctx,
		`UPDATE ticket SET phase=?, updated_at=?,
		 reject_count = reject_count + ? WHERE id=?`,
		string(to), now, boolInt(kind == EdgeReject), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO ticket_event (ticket_id, kind, from_phase, to_phase, actor, note, created_at)
		 VALUES (?,?,?,?,?,?,?)`,
		id, string(kind), from, string(to), actor, note, now); err != nil {
		return err
	}
	return tx.Commit()
}

// NonTerminalTickets 全部非终态票（启动对账输入）。
func (d *DB) NonTerminalTickets(ctx context.Context) ([]*Ticket, error) {
	ph := ""
	args := []any{}
	for i, p := range Terminal {
		if i > 0 {
			ph += ","
		}
		ph += "?"
		args = append(args, string(p))
	}
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, title, phase, project, dedupe_key, budget_frozen, reject_count, defer_count, created_at, updated_at
		 FROM ticket WHERE phase NOT IN (`+ph+`) ORDER BY created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Ticket
	for rows.Next() {
		var t Ticket
		var phase string
		var created, updated int64
		var frozen int
		if err := rows.Scan(&t.ID, &t.Title, &phase, &t.Project, &t.DedupeKey, &frozen,
			&t.RejectCount, &t.DeferCount, &created, &updated); err != nil {
			return nil, err
		}
		t.Phase = Phase(phase)
		t.BudgetFrozen = frozen != 0
		t.CreatedAt = time.Unix(0, created)
		t.UpdatedAt = time.Unix(0, updated)
		out = append(out, &t)
	}
	return out, rows.Err()
}

// AppendEvent 记一条票事件（进度帧/闹钟事件/对账事件共用此径）。
func (d *DB) AppendEvent(ctx context.Context, ticketID, kind string, e Event) error {
	return d.appendEvent(ctx, ticketID, kind, e.FromPhase, e.ToPhase, e.Actor, e.Note, e.Provenance)
}

func (d *DB) appendEvent(ctx context.Context, ticketID, kind string, from, to Phase, actor, note, provenance string) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO ticket_event (ticket_id, kind, from_phase, to_phase, actor, note, provenance, created_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		ticketID, kind, string(from), string(to), actor, note, provenance, d.now().UnixNano())
	return err
}

// Events 取票的全部事件（判决史/输出流审计径）。
func (d *DB) Events(ctx context.Context, ticketID string) ([]*Event, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, ticket_id, kind, from_phase, to_phase, actor, note, provenance, created_at
		 FROM ticket_event WHERE ticket_id=? ORDER BY id`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// LastActivityAt 票上最近一次活性事件的墙上时间——闹钟 deadline 由此重算（§5.3 A4）。
// 活性事件＝进度帧或领票；都没有则回退票 updated_at。
func (d *DB) LastActivityAt(ctx context.Context, ticketID string) (time.Time, error) {
	var ts sql.NullInt64
	if err := d.db.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM ticket_event
		 WHERE ticket_id=? AND kind IN ('progress','claim')`, ticketID).Scan(&ts); err != nil {
		return time.Time{}, err
	}
	if ts.Valid {
		return time.Unix(0, ts.Int64), nil
	}
	var upd int64
	if err := d.db.QueryRowContext(ctx,
		`SELECT updated_at FROM ticket WHERE id=?`, ticketID).Scan(&upd); err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, upd), nil
}

func scanEvents(rows *sql.Rows) ([]*Event, error) {
	var out []*Event
	for rows.Next() {
		var e Event
		var from, to string
		var created int64
		if err := rows.Scan(&e.ID, &e.TicketID, &e.Kind, &from, &to,
			&e.Actor, &e.Note, &e.Provenance, &created); err != nil {
			return nil, err
		}
		e.FromPhase, e.ToPhase = Phase(from), Phase(to)
		e.CreatedAt = time.Unix(0, created)
		out = append(out, &e)
	}
	return out, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
