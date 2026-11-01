package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Project 项目配方行（总设计 §11 子集——票0010 消费列）。
type Project struct {
	ID            string
	RepoURL       string // clone 源（本机路径或远端；空＝无仓址，cwd=干净空目录）
	DefaultBranch string
	IntegrateCmd  string // 集成交付支命令（integrate 步）
	CheckCmd      string
	DeployCmd     string
	ProbeCmd      string
	RollbackCmd   string
	SandboxSpec   string // prototext 序列化 SandboxSpec（run_as 缺省＋按步覆盖）
}

// CmdFor 步名→配方命令：配方外步名不得代跑（范围外禁线落点）。
func (p *Project) CmdFor(step string) (string, bool) {
	switch step {
	case "integrate":
		return p.IntegrateCmd, p.IntegrateCmd != ""
	case "check":
		return p.CheckCmd, p.CheckCmd != ""
	case "deploy":
		return p.DeployCmd, p.DeployCmd != ""
	case "probe":
		return p.ProbeCmd, p.ProbeCmd != ""
	case "rollback":
		return p.RollbackCmd, p.RollbackCmd != ""
	}
	return "", false
}

// UpsertProject 种/改项目配方行。
func (d *DB) UpsertProject(ctx context.Context, p Project) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO project (id, repo_url, default_branch, integrate_cmd, check_cmd, deploy_cmd, probe_cmd, rollback_cmd, sandbox_spec)
		 VALUES (?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET repo_url=excluded.repo_url, default_branch=excluded.default_branch,
		   integrate_cmd=excluded.integrate_cmd, check_cmd=excluded.check_cmd, deploy_cmd=excluded.deploy_cmd,
		   probe_cmd=excluded.probe_cmd, rollback_cmd=excluded.rollback_cmd, sandbox_spec=excluded.sandbox_spec`,
		cargs(p.ID, p.RepoURL, p.DefaultBranch, p.IntegrateCmd, p.CheckCmd,
			p.DeployCmd, p.ProbeCmd, p.RollbackCmd, p.SandboxSpec)...)
	return err
}

// GetProject 取项目配方；无行返 nil,nil。
func (d *DB) GetProject(ctx context.Context, id string) (*Project, error) {
	var p Project
	err := d.db.QueryRowContext(ctx,
		`SELECT id, repo_url, default_branch, integrate_cmd, check_cmd, deploy_cmd, probe_cmd, rollback_cmd, sandbox_spec
		 FROM project WHERE id=?`, id).
		Scan(&p.ID, &p.RepoURL, &p.DefaultBranch, &p.IntegrateCmd, &p.CheckCmd,
			&p.DeployCmd, &p.ProbeCmd, &p.RollbackCmd, &p.SandboxSpec)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// SeedProject 种行落点：同 id 行已存在即不动（DB 行是活态，同壳行种子）。
// 返回 true＝本行是刚落的。
func (d *DB) SeedProject(ctx context.Context, p Project) (bool, error) {
	cur, err := d.GetProject(ctx, p.ID)
	if err != nil {
		return false, err
	}
	if cur != nil {
		return false, nil
	}
	return true, d.UpsertProject(ctx, p)
}

// SetTicketProject 给票配项目列（票0010：project 派发/锁键与配方查表的来源）。
func (d *DB) SetTicketProject(ctx context.Context, id, project string) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE ticket SET project=?, updated_at=? WHERE id=?`,
		cargs(project, d.now().UnixNano(), id)...)
	return err
}

// TicketsByProjectPhase 同项目指定相位票集（项目锁释放后的 spawn 补发源）。
func (d *DB) TicketsByProjectPhase(ctx context.Context, project string, phases []Phase) ([]*Ticket, error) {
	if len(phases) == 0 {
		return nil, nil
	}
	ph := ""
	args := []any{project}
	for i, p := range phases {
		if i > 0 {
			ph += ","
		}
		ph += "?"
		args = append(args, string(p))
	}
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, title, phase, spec, project, dedupe_key, budget_frozen, reject_count, defer_count, created_at, updated_at
		 FROM ticket WHERE project=? AND phase IN (`+ph+`) ORDER BY created_at`, args...)
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
		if err := rows.Scan(&t.ID, &t.Title, &phase, &t.Spec, &t.Project, &t.DedupeKey, &frozen,
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

// CmdRun 一次代跑留痕（票0010 §10/B2：退码+输出路径+尾段+diff 文件清单）。
type CmdRun struct {
	ID        int64
	TicketID  string
	RunID     string // 发起 exec 的席位 run_id（部署席）
	Step      string
	ExitCode  int
	OutPath   string
	Tail      string
	DiffFiles string // 步后 clone 的 git status --porcelain 清单（透明非闸）
	CreatedAt time.Time
}

// InsertCmdRun 落一行代跑记录。
func (d *DB) InsertCmdRun(ctx context.Context, r *CmdRun) error {
	res, err := d.db.ExecContext(ctx,
		`INSERT INTO cmd_run (ticket_id, run_id, step, exit_code, out_path, tail, diff_files, created_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		cargs(r.TicketID, r.RunID, r.Step, r.ExitCode, r.OutPath, r.Tail, r.DiffFiles,
			d.now().UnixNano())...)
	if err != nil {
		return err
	}
	r.ID, err = res.LastInsertId()
	return err
}

// CmdRuns 票上全部代跑记录（按序——四步序断言与审计取证同径）。
func (d *DB) CmdRuns(ctx context.Context, ticketID string) ([]*CmdRun, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, ticket_id, run_id, step, exit_code, out_path, tail, diff_files, created_at
		 FROM cmd_run WHERE ticket_id=? ORDER BY id`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CmdRun
	for rows.Next() {
		var r CmdRun
		var created int64
		if err := rows.Scan(&r.ID, &r.TicketID, &r.RunID, &r.Step, &r.ExitCode,
			&r.OutPath, &r.Tail, &r.DiffFiles, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = time.Unix(0, created)
		out = append(out, &r)
	}
	return out, rows.Err()
}
