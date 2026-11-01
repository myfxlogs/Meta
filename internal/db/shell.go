package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Shell 壳登记表一行（§7：壳×模型×适用角色×优先序×健康态）。
// Health ∈ ok|limited|dead|unknown；priority 小者优先。
type Shell struct {
	Name     string
	Model    string
	Role     string
	Priority int
	Health   string
}

// ShellKey 登记表行键。
func (s Shell) ShellKey() string { return s.Name + "/" + s.Model + "/" + s.Role }

// ListShells 列某角色全部登记行（路由输入）。
func (d *DB) ListShells(ctx context.Context, role string) ([]Shell, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT name, model, role, priority, health FROM shell WHERE role=? ORDER BY priority`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Shell
	for rows.Next() {
		var s Shell
		if err := rows.Scan(&s.Name, &s.Model, &s.Role, &s.Priority, &s.Health); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpsertShell 登记/更新一行（健康态不在此改——走 SetShellHealth）。
func (d *DB) UpsertShell(ctx context.Context, s Shell) error {
	h := s.Health
	if h == "" {
		h = "unknown"
	}
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO shell (name, model, role, priority, health) VALUES (?,?,?,?,?)
		 ON CONFLICT(name, model, role) DO UPDATE SET priority=excluded.priority`,
		s.Name, s.Model, s.Role, s.Priority, h)
	return err
}

// SetShellHealth 健康态迁移（路由懒探/标死的唯一改径）。
func (d *DB) SetShellHealth(ctx context.Context, s Shell, health string) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE shell SET health=? WHERE name=? AND model=? AND role=?`,
		health, s.Name, s.Model, s.Role)
	return err
}

// ShellHealth 取行当前健康态（无行→unknown）。
func (d *DB) ShellHealth(ctx context.Context, s Shell) (string, error) {
	var h string
	err := d.db.QueryRowContext(ctx,
		`SELECT health FROM shell WHERE name=? AND model=? AND role=?`,
		s.Name, s.Model, s.Role).Scan(&h)
	if errors.Is(err, sql.ErrNoRows) {
		return "unknown", nil
	}
	return h, err
}

// RunsForTicket 票的全部 seat_run（取证/管理面导出）。
func (d *DB) RunsForTicket(ctx context.Context, ticketID string) ([]*Run, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT `+runCols+` FROM seat_run WHERE ticket_id=? ORDER BY started_at`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Run
	for rows.Next() {
		var r Run
		var started, attached, adopted, exited int64
		var closed, dead, adjunct int
		if err := rows.Scan(&r.RunID, &r.SeatID, &r.TicketID, &r.Role, &r.Shell, &r.PID,
			&r.RunToken, &r.SocketPath, &r.LogPath, &started, &attached, &adopted,
			&exited, &r.ExitCode, &closed, &dead, &adjunct); err != nil {
			return nil, err
		}
		r.StartedAt = time.Unix(0, started)
		if attached != 0 {
			r.AttachedAt = time.Unix(0, attached)
		}
		if adopted != 0 {
			r.AdoptedAt = time.Unix(0, adopted)
		}
		if exited != 0 {
			r.ExitedAt = time.Unix(0, exited)
		}
		r.StreamClosed = closed != 0
		r.Dead = dead != 0
		r.Adjunct = adjunct != 0
		out = append(out, &r)
	}
	return out, rows.Err()
}

// Budget 票预算行（闸判定输入）。
type Budget struct {
	Cap       int64 // 每次尝试内上限（0=不设）
	Used      int64 // 本次尝试已计
	HardCap   int64 // 票级累计硬顶（0=不设）
	UsedTotal int64 // 票级累计（单调递增，追加=抬帽不清零）
}

// SetBudgetCaps 覆写两帽（追加预算＝抬帽不清零的落点；无行则建）。
func (d *DB) SetBudgetCaps(ctx context.Context, ticketID string, cap, hardCap int64) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO budget (ticket_id, cap, hard_cap, used, used_total, updated_at)
		 VALUES (?,?,?,0,0,?)
		 ON CONFLICT(ticket_id) DO UPDATE SET cap=excluded.cap, hard_cap=excluded.hard_cap, updated_at=excluded.updated_at`,
		ticketID, cap, hardCap, d.now().UnixNano())
	return err
}

// ResetAttemptUsage 新尝试开始：used 清零（used_total 不动——单调递增）。
func (d *DB) ResetAttemptUsage(ctx context.Context, ticketID string) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE budget SET used=0, updated_at=? WHERE ticket_id=?`,
		d.now().UnixNano(), ticketID)
	return err
}

// GetBudget 取预算行（无行→零值）。
func (d *DB) GetBudget(ctx context.Context, ticketID string) (Budget, error) {
	var b Budget
	err := d.db.QueryRowContext(ctx,
		`SELECT cap, used, hard_cap, used_total FROM budget WHERE ticket_id=?`, ticketID).
		Scan(&b.Cap, &b.Used, &b.HardCap, &b.UsedTotal)
	if errors.Is(err, sql.ErrNoRows) {
		return Budget{}, nil
	}
	return b, err
}

// SetBudgetFrozen 置/解冻票级冻结旗（§5.5 A6；相位不动）。
func (d *DB) SetBudgetFrozen(ctx context.Context, ticketID string, frozen bool) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE ticket SET budget_frozen=?, updated_at=? WHERE id=?`,
		boolInt(frozen), d.now().UnixNano(), ticketID)
	return err
}
