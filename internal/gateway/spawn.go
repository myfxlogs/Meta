package gateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"meta/internal/db"
)

// wipeSecretsDir 凭据目录收尸（T18 真只读副作用）：先逐件 chmod 回
// 0700 再 RemoveAll——0555/0444 目录文件 unlinkat 即 EACCES。
func (g *Gateway) wipeSecretsDir(runID string) {
	dir := filepath.Join(g.cfg.RunDir, "secrets", runID)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil {
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
	_ = os.RemoveAll(dir)
}

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

	// 0034-p3 连坐豁免：席包 systemd-run scope——逃出 meta-gateway
	// cgroup，deploy restart 不再满门杀（adjunct 免连坐＋他票席豁免）；
	// scope 失则直跑（旧径）。
	if p, err := exec.LookPath("systemd-run"); err == nil {
		argv = append([]string{p, "--user", "--scope", "--quiet", "--collect",
			"--unit=meta-seat-" + run.RunID, "--"}, argv...)
	}
	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...)
	// T33① 基座环境缺省补齐：systemd-run --user 连用户管理器全靠
	// XDG_RUNTIME_DIR/DBUS_SESSION_BUS_ADDRESS——metaseat 网关 environ
	// 零 XDG（实证）时注入件亦无，scope 包装 argv 起跑前死（0049
	// exit=-1 三连实证）。已有键不动（显式配置优先）。
	cmd.Env = seatBaseEnv(env)
	cmd.Env = append(cmd.Env,
		"META_RUN_ID="+run.RunID,
		"META_RUN_TOKEN="+run.RunToken,
		"META_SEAT_SOCK="+run.SocketPath,
		"META_GATEWAY_SOCK="+g.cfg.SocketPath,
		"META_TICKET_ID="+run.TicketID,
	)
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// T31② 死必有日志：spawn 头行先落——cli-drive 秒退/被杀零字节
	// 实证（三任死全空档死因黑箱）；头行保非空＋argv 凭证现形。
	fmt.Fprintf(logf, "seat spawn argv=%v ticket=%s role=%s\n", argv, run.TicketID, run.Role)

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

	g.wg.Add(1)
	go func() { // SIGCHLD 源：父进程 Wait 返回＝子死
		defer g.wg.Done()
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
		// T31② exit 落行：死因留档（被杀/秒退可查——日志留档不动）。
		if f, err := os.OpenFile(run.LogPath, os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			fmt.Fprintf(f, "seat exit code=%d\n", code)
			f.Close()
		}
		g.maybeDeclareDead(st)
	}()
	return nil
}

// seatBaseEnv 席位基座环境：PATH/HOME 缺则沿网关值（席与网关同 uid），
// XDG_RUNTIME_DIR 缺则按 uid 拼 /run/user/<uid>——运行目录存在才补
// （容器/无登录会话不强补坏路径）；DBUS 缺则按 XDG 标准布局 unix:path=
// $XDG_RUNTIME_DIR/bus 拼。注入件已带键一律不动。
func seatBaseEnv(env []string) []string {
	return seatBaseEnvAt(env, fmt.Sprintf("/run/user/%d", os.Getuid()))
}

func seatBaseEnvAt(env []string, xdgDir string) []string {
	has := func(k string) (string, bool) {
		for _, kv := range env {
			if strings.HasPrefix(kv, k+"=") {
				return kv[len(k)+1:], true
			}
		}
		return "", false
	}
	out := append([]string{}, env...)
	for _, k := range []string{"PATH", "HOME"} {
		if _, ok := has(k); !ok {
			if v := os.Getenv(k); v != "" {
				out = append(out, k+"="+v)
			}
		}
	}
	xdg, hasX := has("XDG_RUNTIME_DIR")
	if !hasX {
		if fi, err := os.Stat(xdgDir); err != nil || !fi.IsDir() {
			return out
		}
		xdg = xdgDir
		out = append(out, "XDG_RUNTIME_DIR="+xdg)
	}
	if _, ok := has("DBUS_SESSION_BUS_ADDRESS"); !ok {
		out = append(out, "DBUS_SESSION_BUS_ADDRESS=unix:path="+xdg+"/bus")
	}
	return out
}

// processAlive kill(pid,0) 一次性核实——只在 EOF/对账等事件点上调用，非探活。
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err != nil && err != syscall.EPERM {
		return false
	}
	// T44：僵尸态判死——kill(pid,0) 对 zombie 恒活，调和器永判 actual 在
	// →保守跳过零动作（0052 施工中席死 24min 零动作实证根）。Linux
	// /proc/<pid>/stat state=='Z'＝已死待收，不属活席。
	if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
		st := string(b)
		if i := strings.LastIndex(st, ")"); i >= 0 && i+4 <= len(st) {
			if f := strings.Fields(st[i+2:]); len(f) > 0 && f[0] == "Z" {
				return false
			}
		}
	}
	return true
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// processAliveRun 核 PID 属本席进程（T32①）：kill0 之外验进程启动时刻
// ——PID 复用者启动早于席行（run.StartedAt 先于 cmd.Start 落账）即判非
// 本席，stale 行不再假在岗。proc 读不了回保守 kill0 口径（容器/权限角）。
// starttime=jiffies@boot；/proc/stat btime=开机历元；CLK_TCK=100
// （x86_64 恒定——arch 异则退回容忍口径仍不楔）。
func processAliveRun(r *db.Run) bool {
	if r == nil || r.PID <= 0 || !processAlive(r.PID) {
		return false
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", r.PID))
	if err != nil {
		return true
	}
	i := bytes.LastIndexByte(b, ')') // comm 可含空格/括号——从末 ')' 后切
	if i < 0 {
		return true
	}
	f := strings.Fields(string(b[i+1:]))
	if len(f) < 20 {
		return true
	}
	jif, err := strconv.ParseUint(f[19], 10, 64) // stat 第 22 字段=starttime
	if err != nil {
		return true
	}
	boot := bootEpoch()
	if boot == 0 {
		return true
	}
	// 双窗：本席进程起刻 ≈ StartedAt（spawn 滞后秒级）；早 2s 或晚 60s
	// 外＝PID 复用者（stale 行的 pid 被晚起/早起进程占——起刻必异）。
	procStart := time.Unix(boot+int64(jif/100), 0)
	return procStart.After(r.StartedAt.Add(-2*time.Second)) &&
		procStart.Before(r.StartedAt.Add(60*time.Second))
}

// bootEpoch /proc/stat btime 开机历元；读不了返 0（判径退回容忍口径）。
func bootEpoch() int64 {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(ln, "btime ") {
			v, _ := strconv.ParseInt(strings.TrimSpace(ln[6:]), 10, 64)
			return v
		}
	}
	return 0
}

// killRunGroup 席进程整组 SIGKILL——spawn 走 Setpgid，杀负 pid 连坐
// 壳的子进程；pid≤0（token run）或已退出吞错，幂等无害。
func killRunGroup(pid int) {
	if pid > 0 {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}

// cleanSeatResidue 死席残件连清（T27④，业下令「保持系统干净」）：
// 收尸径共用——seats/<runid>.sock 插座连删＋席位工作区收（git worktree
// 经 common-dir 归主仓移除；非 worktree 工作区不动——工作目录可能是
// 业主手工件，误删大于收益）。日志/artifacts 留档不动（审计史）。
func (g *Gateway) cleanSeatResidue(run *db.Run) {
	if run.SocketPath != "" {
		_ = os.Remove(run.SocketPath)
	}
	g.mu.Lock()
	wd := g.runWorkDir[run.RunID]
	g.mu.Unlock()
	if wd == "" {
		return
	}
	if fi, err := os.Stat(filepath.Join(wd, ".git")); err != nil || fi == nil {
		return // 非 git 工作区不动
	}
	common, err := exec.Command("git", "-C", wd, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return
	}
	gd := strings.TrimSpace(string(common))
	if !filepath.IsAbs(gd) {
		gd = filepath.Join(wd, gd)
	}
	if err := exec.Command("git", "--git-dir="+gd,
		"worktree", "remove", "--force", wd).Run(); err != nil {
		_ = g.database.AppendEvent(context.Background(), run.TicketID, "seat_residue", db.Event{
			Actor: "gateway", Note: "工作区收不净: " + wd})
	}
}
