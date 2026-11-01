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
	if len(argv) == 0 {
		return nil, fmt.Errorf("spawn: 空 argv")
	}
	run := &db.Run{
		RunID:      randHex(8),
		TicketID:   ticketID,
		Role:       role,
		Shell:      shell,
		RunToken:   randHex(16),
		SocketPath: filepath.Join(g.cfg.RunDir, "seats", ""),
		LogPath:    filepath.Join(g.cfg.RunDir, "logs", ""),
	}
	run.SocketPath += run.RunID + ".sock"
	run.LogPath += run.RunID + ".log"

	logf, err := os.OpenFile(run.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("spawn log: %w", err)
	}
	defer logf.Close()

	if err := g.database.CreateRun(ctx, run); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	cmd.Env = append([]string{}, env...)
	cmd.Env = append(cmd.Env,
		"META_RUN_ID="+run.RunID,
		"META_RUN_TOKEN="+run.RunToken,
		"META_SEAT_SOCK="+run.SocketPath,
		"META_GATEWAY_SOCK="+g.cfg.SocketPath,
		"META_TICKET_ID="+ticketID,
	)
	cmd.Stdout, cmd.Stderr = logf, logf

	if err := cmd.Start(); err != nil {
		_ = g.database.MarkDead(ctx, run.RunID)
		return nil, fmt.Errorf("spawn %s: %w", shell, err) // ①级：同步返回，调用方换壳
	}
	run.PID = cmd.Process.Pid
	_ = g.database.UpdatePID(ctx, run.RunID, run.PID)

	st := &runState{run: run, cmd: cmd.Process}
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
		_ = g.database.MarkExited(context.Background(), run.RunID, code)
		g.maybeDeclareDead(st)
	}()
	return run, nil
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
