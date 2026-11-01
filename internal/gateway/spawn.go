package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"meta/internal/db"
)

// Spawn 拉起一个席位实例（父子进程）＋分配 per-seat UDS 路径（§5.1）。
// run_token 随机一次性，经 env 注入；spawn 失败是同步返回——调用方据此换壳。
// 席位约定环境变量：META_GATEWAY_SOCK / META_SEAT_SOCK / META_RUN_TOKEN /
// META_TICKET_ID / META_RUN_ID。
func (g *Gateway) Spawn(ctx context.Context, ticketID, role, shell string, argv []string, env []string) (*db.Run, error) {
	run := g.newRun(ticketID, role, shell)
	if err := g.startRun(ctx, run, argv, env); err != nil {
		return nil, err
	}
	return run, nil
}

// newRun 分配 run 身份与落点（socket/log 路径、token）。
func (g *Gateway) newRun(ticketID, role, shell string) *db.Run {
	id := randHex(8)
	return &db.Run{
		RunID:      id,
		TicketID:   ticketID,
		Role:       role,
		Shell:      shell,
		RunToken:   randHex(16),
		SocketPath: filepath.Join(g.cfg.RunDir, "seats", id+".sock"),
		LogPath:    filepath.Join(g.cfg.RunDir, "logs", id+".log"),
	}
}

// startRun 登记 run 行并前台 exec（B5：禁 daemonize——SIGCHLD/EOF 双源靠它）。
// 子进程独立进程组（Setpgid）——killRun 负 pid 整组杀，外壳+壳子进程同灭。
func (g *Gateway) startRun(ctx context.Context, run *db.Run, argv []string, env []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("spawn: 空 argv")
	}
	logf, err := os.OpenFile(run.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("spawn log: %w", err)
	}
	defer logf.Close()

	if err := g.database.CreateRun(ctx, run); err != nil {
		return err
	}

	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	cmd.Env = append([]string{}, env...)
	cmd.Env = append(cmd.Env,
		"META_RUN_ID="+run.RunID,
		"META_RUN_TOKEN="+run.RunToken,
		"META_SEAT_SOCK="+run.SocketPath,
		"META_GATEWAY_SOCK="+g.cfg.SocketPath,
		"META_TICKET_ID="+run.TicketID,
	)
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		_ = g.database.MarkDead(ctx, run.RunID)
		return fmt.Errorf("spawn %s: %w", run.Shell, err) // ①级：同步返回，调用方换壳
	}
	run.PID = cmd.Process.Pid
	_ = g.database.UpdatePID(ctx, run.RunID, run.PID)

	st := newRunState(run)
	st.cmd = cmd.Process
	g.mu.Lock()
	g.runs[run.RunID] = st
	g.mu.Unlock()

	go func() { // SIGCHLD 源：父进程 Wait 返回＝子死
		err := cmd.Wait()
		code := -1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err == nil {
			code = 0
		}
		st.mu.Lock()
		st.exited, st.exitCode = true, code
		st.mu.Unlock()
		close(st.exitCh)
		_ = g.database.MarkExited(context.Background(), run.RunID, code)
		g.maybeDeclareDead(st)
	}()
	return nil
}

// processAlive kill(pid,0) 一次性核实——只在 EOF/对账等事件点上调用，非探活。
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
