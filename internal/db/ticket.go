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
	Spec         []byte // 序列化 pbv1.TicketSpec（票面八栏，可空）
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
	Role       string // actor 为 run_id 时经 seat_run 映射带出（决策41），否则空
	Note       string
	Provenance string
	CreatedAt  time.Time
}

// CreateTicket 立票（初始相位 已立）。
func (d *DB) CreateTicket(ctx context.Context, id, title string, spec []byte) error {
	return d.CreateTicketProv(ctx, id, title, spec, "")
}

// CreateTicketProv 立票＋created 事件盖 provenance（D9：CEO/admin 帧链
// 立票必须带戳——「业主指示」自此机检可查）。
func (d *DB) CreateTicketProv(ctx context.Context, id, title string, spec []byte, provenance string) error {
	now := d.now().UnixNano()
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO ticket (id, title, phase, spec, created_at, updated_at) VALUES (?,?,?,?,?,?)`,
		id, title, string(PhaseCreated), spec, now, now)
	if err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	return d.appendEvent(ctx, id, "created", "", PhaseCreated, "gateway", "", provenance)
}

// GetTicket 取票。
func (d *DB) GetTicket(ctx context.Context, id string) (*Ticket, error) {
	var t Ticket
	var phase string
	var created, updated int64
	var frozen int
	err := d.db.QueryRowContext(ctx,
		`SELECT id, title, phase, spec, project, dedupe_key, budget_frozen, reject_count, defer_count, created_at, updated_at
		 FROM ticket WHERE id=?`, id).
		Scan(&t.ID, &t.Title, &phase, &t.Spec, &t.Project, &t.DedupeKey, &frozen,
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
// 返回迁移事件行 id（播报员水位游标以 ticket_event.id 为全序基准）。
func (d *DB) Transition(ctx context.Context, id string, to Phase, kind EdgeKind, actor, note string) (int64, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var from string
	if err := tx.QueryRowContext(ctx, `SELECT phase FROM ticket WHERE id=?`, id).Scan(&from); err != nil {
		return 0, fmt.Errorf("transition read: %w", err)
	}
	if !Can(Phase(from), to, kind) {
		return 0, &ErrIllegalEdge{From: Phase(from), To: to, Kind: kind}
	}
	now := d.now().UnixNano()
	if _, err := tx.ExecContext(ctx,
		`UPDATE ticket SET phase=?, updated_at=?,
		 reject_count = reject_count + ? WHERE id=?`,
		string(to), now, boolInt(kind == EdgeReject), id); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO ticket_event (ticket_id, kind, from_phase, to_phase, actor, note, created_at)
		 VALUES (?,?,?,?,?,?,?)`,
		id, string(kind), from, string(to), actor, note, now)
	if err != nil {
		return 0, err
	}
	rowID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return rowID, nil
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
		`SELECT e.id, e.ticket_id, e.kind, e.from_phase, e.to_phase, e.actor, e.note, e.provenance, e.created_at,
		        COALESCE(r.role,'')
		 FROM ticket_event e LEFT JOIN seat_run r ON r.run_id = e.actor
		 WHERE e.ticket_id=? ORDER BY e.id`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// EventsAfter 水位游标回读（D-25）：跨票全序 id>sinceID 的事件页——
// 播报员断线补报的唯一径（同码径覆盖启动对账与流断重连）。
func (d *DB) EventsAfter(ctx context.Context, sinceID int64, limit int) ([]*Event, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT e.id, e.ticket_id, e.kind, e.from_phase, e.to_phase, e.actor, e.note, e.provenance, e.created_at,
		        COALESCE(r.role,'')
		 FROM ticket_event e LEFT JOIN seat_run r ON r.run_id = e.actor
		 WHERE e.id>? ORDER BY e.id LIMIT ?`, sinceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// LatestEventKind 票上最新一条 ticket_event 的 kind（req_pending 泊车标/
// 对账抑制判据——无行回空串）。
func (d *DB) LatestEventKind(ctx context.Context, ticketID string) (string, error) {
	var k string
	err := d.db.QueryRowContext(ctx,
		`SELECT kind FROM ticket_event WHERE ticket_id=? ORDER BY id DESC LIMIT 1`,
		ticketID).Scan(&k)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return k, err
}

// UpdateSpec 回填票面 spec（席位流 admin fill_spec 帧落点；BLOB 直写）。
func (d *DB) UpdateSpec(ctx context.Context, ticketID string, spec []byte) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE ticket SET spec=?, updated_at=? WHERE id=?`,
		spec, d.now().UnixNano(), ticketID)
	return err
}

// TicketsByProjectPhase 项目×相位查票（design 锁释放后补发 spawn_needed 的
// 查表径；project 空串票不入锁族）。
func (d *DB) TicketsByProjectPhase(ctx context.Context, project string, phase Phase) ([]string, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT id FROM ticket WHERE project=? AND phase=? ORDER BY created_at`, project, string(phase))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// LastActivityAt 票上最近一次活性事件的墙上时间——闹钟 deadline 由此重算（§5.3 A4）。
// 活性事件＝进度帧/用量帧/领票；都没有则回退票 updated_at。
func (d *DB) LastActivityAt(ctx context.Context, ticketID string) (time.Time, error) {
	var ts sql.NullInt64
	if err := d.db.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM ticket_event
		 WHERE ticket_id=? AND kind IN ('progress','usage','claim')`, ticketID).Scan(&ts); err != nil {
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
			&e.Actor, &e.Note, &e.Provenance, &created, &e.Role); err != nil {
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
