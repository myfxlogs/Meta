package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Run 一次席位实例运行（seat_run 行）。
type Run struct {
	RunID        string
	SeatID       int64
	TicketID     string
	Role         string
	Shell        string
	PID          int
	RunToken     string
	SocketPath   string
	LogPath      string
	StartedAt    time.Time
	AttachedAt   time.Time
	AdoptedAt    time.Time
	ExitedAt     time.Time
	ExitCode     int
	StreamClosed bool
	Dead         bool
	Adjunct      bool   // 辅助席（裁决/诊断）：不占相位、不收养（票0004 §1.3）
	Provenance   string // spawn 方盖戳（D9）：terminal|feishu:<id>|system_event|auto
}

// CreateRun 登记一次 spawn。
func (d *DB) CreateRun(ctx context.Context, r *Run) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO seat_run (run_id, ticket_id, role, shell, pid, run_token, socket_path, log_path, started_at, adjunct, provenance)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		r.RunID, r.TicketID, r.Role, r.Shell, r.PID, r.RunToken, r.SocketPath, r.LogPath,
		d.now().UnixNano(), boolInt(r.Adjunct), r.Provenance)
	return err
}

// runCols seat_run 查询列序（scanRun/RunsForTicket 共用， adjunct/provenance 恒在末位）。
const runCols = `run_id, seat_id, ticket_id, role, shell, pid, run_token, socket_path, log_path,
       started_at, attached_at, adopted_at, exited_at, exit_code, stream_closed, dead, adjunct, provenance`

// RunByToken run_token 查 run——Attach 首帧验证径。
func (d *DB) RunByToken(ctx context.Context, token string) (*Run, error) {
	return d.scanRun(d.db.QueryRowContext(ctx,
		`SELECT `+runCols+` FROM seat_run WHERE run_token=?`, token))
}

// RunByID 按 run_id 查 run——板变化帧按事件 RunID 重读现态（幂等投影径）。
func (d *DB) RunByID(ctx context.Context, runID string) (*Run, error) {
	return d.scanRun(d.db.QueryRowContext(ctx,
		`SELECT `+runCols+` FROM seat_run WHERE run_id=?`, runID))
}

// ActiveRuns 全部未判死 run（板全态帧的活席输入）。
func (d *DB) ActiveRuns(ctx context.Context) ([]*Run, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT `+runCols+` FROM seat_run WHERE dead=0 ORDER BY started_at`)
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
			&exited, &r.ExitCode, &closed, &dead, &adjunct, &r.Provenance); err != nil {
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

// ActiveRunForTicket 票上未判死的最新 non-adjunct run（启动对账/杀谁 用——
// §1.3：adjunct 不被收养，也不在 stall 裁决的猎杀集里）。
func (d *DB) ActiveRunForTicket(ctx context.Context, ticketID string) (*Run, error) {
	return d.scanRun(d.db.QueryRowContext(ctx,
		`SELECT `+runCols+` FROM seat_run
		 WHERE ticket_id=? AND dead=0 AND adjunct=0 ORDER BY started_at DESC LIMIT 1`, ticketID))
}

// ActiveRunForTicketRole 票上同角色的活 non-adjunct run（§15.1 同角色活席去重闸）。
func (d *DB) ActiveRunForTicketRole(ctx context.Context, ticketID, role string) (*Run, error) {
	return d.scanRun(d.db.QueryRowContext(ctx,
		`SELECT `+runCols+` FROM seat_run
		 WHERE ticket_id=? AND dead=0 AND adjunct=0 AND role=? ORDER BY started_at DESC LIMIT 1`, ticketID, role))
}

// ActiveAdjunctForTicketRole 票上同角色的活 adjunct run（§1.3 adjunct 去重闸——防裁决席堆叠）。
func (d *DB) ActiveAdjunctForTicketRole(ctx context.Context, ticketID, role string) (*Run, error) {
	return d.scanRun(d.db.QueryRowContext(ctx,
		`SELECT `+runCols+` FROM seat_run
		 WHERE ticket_id=? AND dead=0 AND adjunct=1 AND role=? ORDER BY started_at DESC LIMIT 1`, ticketID, role))
}

// ActiveRunForRole 某角色未判死的最新 run（CEO 唤起去重闸——与票无关，
// CEO run 的 ticket_id 恒空串）。
func (d *DB) ActiveRunForRole(ctx context.Context, role string) (*Run, error) {
	return d.scanRun(d.db.QueryRowContext(ctx,
		`SELECT `+runCols+` FROM seat_run
		 WHERE role=? AND dead=0 ORDER BY started_at DESC LIMIT 1`, role))
}

func (d *DB) scanRun(row *sql.Row) (*Run, error) {
	var r Run
	var started, attached, adopted, exited int64
	var closed, dead, adjunct int
	err := row.Scan(&r.RunID, &r.SeatID, &r.TicketID, &r.Role, &r.Shell, &r.PID,
		&r.RunToken, &r.SocketPath, &r.LogPath, &started, &attached, &adopted,
		&exited, &r.ExitCode, &closed, &dead, &adjunct, &r.Provenance)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
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
	return &r, nil
}

// MarkAttached 记席位连上时刻。
func (d *DB) MarkAttached(ctx context.Context, runID string) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE seat_run SET attached_at=? WHERE run_id=?`, d.now().UnixNano(), runID)
	return err
}

// MarkAdopted 记收养成功时刻。
func (d *DB) MarkAdopted(ctx context.Context, runID string) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE seat_run SET adopted_at=? WHERE run_id=?`, d.now().UnixNano(), runID)
	return err
}

// MarkExited 记进程退出（SIGCHLD 源）。
func (d *DB) MarkExited(ctx context.Context, runID string, code int) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE seat_run SET exited_at=?, exit_code=? WHERE run_id=?`,
		d.now().UnixNano(), code, runID)
	return err
}

// MarkStreamClosed 记流 EOF（socket 源）。
func (d *DB) MarkStreamClosed(ctx context.Context, runID string) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE seat_run SET stream_closed=1 WHERE run_id=?`, runID)
	return err
}

// MarkDead 双源互证后判死。
func (d *DB) MarkDead(ctx context.Context, runID string) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE seat_run SET dead=1 WHERE run_id=?`, runID)
	return err
}

// UpdatePID 记子进程 pid（spawn 成功后回写）。
func (d *DB) UpdatePID(ctx context.Context, runID string, pid int) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE seat_run SET pid=? WHERE run_id=?`, pid, runID)
	return err
}
