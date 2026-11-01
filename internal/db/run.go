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
}

// CreateRun 登记一次 spawn。
func (d *DB) CreateRun(ctx context.Context, r *Run) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO seat_run (run_id, ticket_id, role, shell, pid, run_token, socket_path, log_path, started_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		r.RunID, r.TicketID, r.Role, r.Shell, r.PID, r.RunToken, r.SocketPath, r.LogPath,
		d.now().UnixNano())
	return err
}

// RunByToken run_token 查 run——Attach 首帧验证径。
func (d *DB) RunByToken(ctx context.Context, token string) (*Run, error) {
	return d.scanRun(d.db.QueryRowContext(ctx,
		`SELECT run_id, seat_id, ticket_id, role, shell, pid, run_token, socket_path, log_path,
		        started_at, attached_at, adopted_at, exited_at, exit_code, stream_closed, dead
		 FROM seat_run WHERE run_token=?`, token))
}

// ActiveRunForTicket 票上未判死的最新 run（启动对账用）。
func (d *DB) ActiveRunForTicket(ctx context.Context, ticketID string) (*Run, error) {
	return d.scanRun(d.db.QueryRowContext(ctx,
		`SELECT run_id, seat_id, ticket_id, role, shell, pid, run_token, socket_path, log_path,
		        started_at, attached_at, adopted_at, exited_at, exit_code, stream_closed, dead
		 FROM seat_run WHERE ticket_id=? AND dead=0 ORDER BY started_at DESC LIMIT 1`, ticketID))
}

func (d *DB) scanRun(row *sql.Row) (*Run, error) {
	var r Run
	var started, attached, adopted, exited int64
	var closed, dead int
	err := row.Scan(&r.RunID, &r.SeatID, &r.TicketID, &r.Role, &r.Shell, &r.PID,
		&r.RunToken, &r.SocketPath, &r.LogPath, &started, &attached, &adopted,
		&exited, &r.ExitCode, &closed, &dead)
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
