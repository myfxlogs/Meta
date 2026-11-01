package gateway

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// ---------- 部署闸测试基建（票0010 锚 2-10） ----------

// seatDrive 代驾流：attach 真席位 socket，用活 run 的 token 直接驱帧。
type seatDrive struct {
	stream pbv1.SeatGateway_AttachClient
}

func driveSeat(t *testing.T, g *Gateway, sockDir string, run *db.Run) *seatDrive {
	t.Helper()
	_, c := dialSeatGateway(t, filepath.Join(sockDir, "run", "meta.sock"))
	stream, err := c.Attach(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{
		Auth: &pbv1.RunAuth{RunToken: run.RunToken}}}); err != nil {
		t.Fatal(err)
	}
	return &seatDrive{stream: stream}
}

func (d *seatDrive) send(t *testing.T, m *pbv1.SeatMsg) {
	t.Helper()
	if err := d.stream.Send(m); err != nil {
		t.Fatal(err)
	}
}

// exec 发一步并等 exec_resp（跳过其它帧；有界）。
func (d *seatDrive) exec(t *testing.T, step, ref string) *pbv1.ExecResp {
	t.Helper()
	d.send(t, &pbv1.SeatMsg{Body: &pbv1.SeatMsg_ExecReq{
		ExecReq: &pbv1.ExecReq{Step: step, Ref: ref}}})
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		m, err := d.stream.Recv()
		if err != nil {
			t.Fatalf("等 exec_resp 断流: %v", err)
		}
		if r := m.GetExecResp(); r != nil && r.Step == step {
			return r
		}
	}
	t.Fatalf("等 exec_resp[%s] 超时", step)
	return nil
}

func (d *seatDrive) claim(t *testing.T, tid string) {
	t.Helper()
	d.send(t, &pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: tid}}})
}

func (d *seatDrive) deliver(t *testing.T, note string) {
	t.Helper()
	d.send(t, &pbv1.SeatMsg{Body: &pbv1.SeatMsg_Deliver{
		Deliver: &pbv1.Deliver{Ref: "t/x-branch", Note: note}}})
}

// waitRun 等有 role 的非 adjunct run 落库（派发是异步径，有界重试）。
func waitRun(t *testing.T, g *Gateway, tid, role string) *db.Run {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		runs, _ := g.DB().RunsForTicket(context.Background(), tid)
		for _, r := range runs {
			if r.Role == role && !r.Adjunct {
				return r
			}
		}
		<-time.After(20 * time.Millisecond)
	}
	t.Fatalf("等 run role=%s 超时", role)
	return nil
}

// deploySpec CTO 部署派发参数：idle 壳不自动连——测试代驾帧。
func deploySpec() SeatSpec {
	return SeatSpec{PromptFile: "/dev/null", WorkDir: "/tmp",
		Env: []string{"GO_HELPER_SEAT=1", "SEAT_MODE=idle", "PATH=" + os.Getenv("PATH")}}
}

// stubProject 伪配方：各步 stdout 回显步名（tail 断言源）；deploy 步
// git init+产 artifact.txt（diff 文件清单断言源）。
func stubProject(id string) db.Project {
	return db.Project{ID: id, RepoURL: "", DefaultBranch: "master",
		IntegrateCmd: "echo integrate-ok",
		CheckCmd:     "echo check-ok",
		DeployCmd:    "echo deploy-ok; git init -q .; echo x > artifact.txt",
		ProbeCmd:     "echo probe-ok",
		RollbackCmd:  "echo rollback-ok",
		SandboxSpec: `run_as: "seatrun"
step_run_as { key: "integrate" value: "owner" }
step_run_as { key: "deploy" value: "owner" }
step_run_as { key: "probe" value: "owner" }
step_run_as { key: "rollback" value: "owner" }`,
	}
}

// sudoStub 伪 sudo：log argv 落盘后剥 -n/-u/-- 前缀真跑命令；exit!=0 则剥壳即退。
func sudoStub(t *testing.T, dir string, code int) string {
	t.Helper()
	log := filepath.Join(dir, "sudo-argv.log")
	var fail string
	if code != 0 {
		fail = fmt.Sprintf("exit %d\n", code)
	}
	body := fmt.Sprintf(`#!/usr/bin/env bash
printf '%%s\n' "$*" >> %s
%swhile [ $# -gt 0 ]; do
  case "$1" in -n) shift;; -u) shift 2;; --) shift; break;; *) shift;; esac
done
exec "$@"
`, log, fail)
	p := filepath.Join(dir, "sudo")
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func mkToDeploy(t *testing.T, g *Gateway, id, project string) {
	t.Helper()
	mkTicketAt(t, g, id, db.PhaseToDeploy)
	if err := g.DB().SetTicketProject(context.Background(), id, project); err != nil {
		t.Fatal(err)
	}
}

// TestDeploy_Gates 锚2：部署支步行＋三重闸——非待部署派 CTO 拒；非部署中/adjunct
// exec 拒；claim→deploy_start→部署中。（对抗锚 d：相位判等翻＝本表红。）
func TestDeploy_Gates(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.SudoBin = sudoStub(t, d, 0)
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g, "tg", "P")
	if err := g.DispatchSeat(ctx, "tg", "CTO", deploySpec()); err != nil {
		t.Fatalf("待部署派 CTO 应放行: %v", err)
	}
	// 非待部署派 CTO 部署支→拒（已派/施工中/待审 皆非两支点）。
	mkTicketAt(t, g, "tb", db.PhaseAssigned)
	if err := g.DispatchSeat(ctx, "tb", "CTO", deploySpec()); err == nil {
		t.Fatal("已派票派 CTO 应拒")
	}
	mkTicketAt(t, g, "tr", db.PhaseReview)
	if err := g.DispatchSeat(ctx, "tr", "CTO", deploySpec()); err == nil {
		t.Fatal("待审票派 CTO 应拒")
	}

	run := waitRun(t, g, "tg", "CTO")
	d := driveSeat(t, g, dir, run)
	// 未 claim＝相位仍 待部署：exec 三重闸之相位闸拒（非零码+note）。
	r := d.exec(t, "check", "")
	if r.ExitCode >= 0 || !strings.Contains(r.Note, "部署中") {
		t.Fatalf("非部署中 exec 应拒: %+v", r)
	}
	// adjunct 席发 exec→拒：在部署中票上起 CTO adjunct。
	if err := g.DispatchSeat(ctx, "tg", "CTO", SeatSpec{
		Adjunct: true, PromptFile: "/dev/null",
		Env: []string{"GO_HELPER_SEAT=1", "SEAT_MODE=hold", "PATH=" + os.Getenv("PATH")}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	var adj *db.Run
	for time.Now().Before(deadline) {
		runs, _ := g.DB().RunsForTicket(ctx, "tg")
		for _, rr := range runs {
			if rr.Adjunct {
				adj = rr
			}
		}
		if adj != nil {
			break
		}
		<-time.After(20 * time.Millisecond)
	}
	if adj == nil {
		t.Fatal("adjunct run 未起")
	}
	da := driveSeat(t, g, dir, adj)
	ra := da.exec(t, "check", "")
	if ra.ExitCode >= 0 {
		t.Fatalf("adjunct exec 应拒: %+v", ra)
	}
	// 正席 claim→deploy_start→部署中。
	d.claim(t, "tg")
	waitPhase(t, g, "tg", db.PhaseDeploying, 5*time.Second)
	_ = d.stream.CloseSend()
}

// TestDeploy_FourGreen 锚3：伪配方四步全绿→exec 按序→deploy_done→完成；
// cmd_run 四行序断言＋退码/输出路径/tail/diff 清单。
func TestDeploy_FourGreen(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.SudoBin = sudoStub(t, d, 0)
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g, "td", "P")
	if err := g.DispatchSeat(ctx, "td", "CTO", deploySpec()); err != nil {
		t.Fatal(err)
	}
	run := waitRun(t, g, "td", "CTO")
	d := driveSeat(t, g, dir, run)
	d.claim(t, "td")
	waitPhase(t, g, "td", db.PhaseDeploying, 5*time.Second)
	for _, step := range []string{"integrate", "check", "deploy", "probe"} {
		if r := d.exec(t, step, "t/0010-x"); r.ExitCode != 0 {
			t.Fatalf("step %s 退码 %d note=%s", step, r.ExitCode, r.Note)
		}
	}
	d.deliver(t, "META_DEPLOY:done")
	waitPhase(t, g, "td", db.PhaseDone, 5*time.Second)

	runs, err := g.DB().CmdRuns(ctx, "td")
	if err != nil || len(runs) != 4 {
		t.Fatalf("cmd_run 应 4 行: %d %v", len(runs), err)
	}
	want := []string{"integrate", "check", "deploy", "probe"}
	for i, r := range runs {
		if r.Step != want[i] || r.ExitCode != 0 || r.OutPath == "" || r.Tail == "" {
			t.Fatalf("cmd_run[%d]=%+v", i, r)
		}
	}
	if !strings.Contains(runs[2].DiffFiles, "artifact.txt") {
		t.Fatalf("deploy 步 diff 清单应含 artifact.txt: %q", runs[2].DiffFiles)
	}
	evs, _ := g.DB().Events(ctx, "td")
	var sawExec, sawDone bool
	for _, e := range evs {
		sawExec = sawExec || e.Kind == "exec"
		sawDone = sawDone || e.Kind == string(db.EdgeDeployDone)
	}
	if !sawExec || !sawDone {
		t.Fatal("exec/deploy_done 事件行缺")
	}
	_ = d.stream.CloseSend()
}

// TestDeploy_RedRollback 锚4＋对抗锚 a/b：check 红→rollback 执行痕→
// deploy_failed→待部署（非完成）。
func TestDeploy_RedRollback(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.SudoBin = sudoStub(t, d, 0)
	})
	ctx := context.Background()
	p := stubProject("P")
	p.CheckCmd = "echo red >&2; exit 1"
	if err := g.DB().UpsertProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g, "tf", "P")
	if err := g.DispatchSeat(ctx, "tf", "CTO", deploySpec()); err != nil {
		t.Fatal(err)
	}
	run := waitRun(t, g, "tf", "CTO")
	d := driveSeat(t, g, dir, run)
	d.claim(t, "tf")
	waitPhase(t, g, "tf", db.PhaseDeploying, 5*time.Second)
	if r := d.exec(t, "integrate", ""); r.ExitCode != 0 {
		t.Fatalf("integrate 应绿: %+v", r)
	}
	if r := d.exec(t, "check", ""); r.ExitCode != 1 {
		t.Fatalf("check 应红=1: %+v", r)
	}
	if r := d.exec(t, "rollback", ""); r.ExitCode != 0 {
		t.Fatalf("rollback 应跑: %+v", r)
	}
	d.deliver(t, "META_DEPLOY:failed")
	waitPhase(t, g, "tf", db.PhaseToDeploy, 5*time.Second)

	runs, _ := g.DB().CmdRuns(ctx, "tf")
	var sawRollback, sawDeploy bool
	for _, r := range runs {
		sawRollback = sawRollback || r.Step == "rollback"
		sawDeploy = sawDeploy || r.Step == "deploy"
	}
	if !sawRollback {
		t.Fatal("红后 rollback 步无执行痕（对抗锚 b）")
	}
	if sawDeploy {
		t.Fatal("check 红后不该跑 deploy 步")
	}
	evs, _ := g.DB().Events(ctx, "tf")
	var sawFailed bool
	for _, e := range evs {
		sawFailed = sawFailed || e.Kind == string(db.EdgeDeployFailed)
	}
	if !sawFailed {
		t.Fatal("deploy_failed 事件行缺")
	}
	if p2, _ := g.DB().PhaseOf(ctx, "tf"); p2 == db.PhaseDone {
		t.Fatal("失败票落完成（对抗锚 a）")
	}
	_ = d.stream.CloseSend()
}

// TestDeploy_ProjectLocks 锚5＋对抗锚 c：同项目二部署→锁拒（spawn_blocked
// 留痕）；首票终态→锁释→补发 spawn_needed；启动按相位重建。
func TestDeploy_ProjectLocks(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.SudoBin = sudoStub(t, d, 0)
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g, "ta", "P")
	mkToDeploy(t, g, "tb2", "P")
	if err := g.DispatchSeat(ctx, "ta", "CTO", deploySpec()); err != nil {
		t.Fatal(err)
	}
	if err := g.DispatchSeat(ctx, "tb2", "CTO", deploySpec()); err == nil {
		t.Fatal("同项目二部署应锁拒（对抗锚 c）")
	}
	var blocked bool
	evs, _ := g.DB().Events(ctx, "tb2")
	for _, e := range evs {
		blocked = blocked || e.Kind == "spawn_blocked"
	}
	if !blocked {
		t.Fatal("锁拒缺 spawn_blocked 留痕")
	}
	run := waitRun(t, g, "ta", "CTO")
	d := driveSeat(t, g, dir, run)
	d.claim(t, "ta")
	waitPhase(t, g, "ta", db.PhaseDeploying, 5*time.Second)
	d.deliver(t, "META_DEPLOY:done")
	waitPhase(t, g, "ta", db.PhaseDone, 5*time.Second)
	waitEvent(t, g, "spawn_needed", "tb2", 5*time.Second)
	_ = d.stream.CloseSend()
}

// TestDeploy_LockRebuildDB 锚5 后半：库内 部署中 票（带活 run 行）→
// 重启收养后锁重建——同项目二部署拒。
func TestDeploy_LockRebuildDB(t *testing.T) {
	dir := t.TempDir()
	cfg := dispatchCfg(dir)
	cfg.SudoBin = sudoStub(t, dir, 0)
	ctx := context.Background()
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	go g.Serve()
	defer g.Close()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	// 造 部署中 票＋活 run（worker 席可被收养）。
	mkToDeploy(t, g, "tr", "P")
	wspec := deploySpec()
	wspec.Env = []string{"GO_HELPER_SEAT=1", "SEAT_MODE=worker", "META_ADOPT_GRACE=60",
		"PATH=" + os.Getenv("PATH")}
	if err := g.DispatchSeat(ctx, "tr", "CTO", wspec); err != nil {
		t.Fatal(err)
	}
	run := waitRun(t, g, "tr", "CTO")
	waitPhase(t, g, "tr", db.PhaseDeploying, 10*time.Second)
	pid := run.PID
	defer func() { _ = syscall.Kill(pid, syscall.SIGKILL) }()
	dbPath, sockPath := cfg.DBPath, cfg.SocketPath
	g.Close() // worker 存活进收养等待

	// 同库重起：收养→票仍 部署中→锁重建→同项目二部署拒。
	cfg2 := dispatchCfg(filepath.Dir(dbPath))
	cfg2.DBPath, cfg2.SocketPath = dbPath, sockPath
	cfg2.SudoBin = cfg.SudoBin
	g2, err := New(cfg2)
	if err != nil {
		t.Fatal(err)
	}
	go g2.Serve()
	defer g2.Close()
	waitPhase(t, g2, "tr", db.PhaseDeploying, 10*time.Second)
	mkToDeploy(t, g2, "tq", "P")
	if err := g2.DispatchSeat(ctx, "tq", "CTO", deploySpec()); err == nil {
		t.Fatal("重启后部署锁未重建——同项目二部署漏放")
	}
}

// TestDeploy_RunAs 锚6：stub sudo 捕 argv——check=-u seatrun（缺省），
// integrate/deploy/probe=-u owner（按步覆盖）；sudo -n 非零退→退码入 cmd_run
// ＋exec 事件，不静默降级。
func TestDeploy_RunAs(t *testing.T) {
	dir := t.TempDir()
	sudo := sudoStub(t, dir, 0)
	g, gdir := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(dir)
		c.SudoBin = sudo
	})
	_ = gdir
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g, "ts", "P")
	if err := g.DispatchSeat(ctx, "ts", "CTO", deploySpec()); err != nil {
		t.Fatal(err)
	}
	run := waitRun(t, g, "ts", "CTO")
	d := driveSeat(t, g, dir, run)
	d.claim(t, "ts")
	waitPhase(t, g, "ts", db.PhaseDeploying, 5*time.Second)
	for _, s := range []string{"integrate", "check", "deploy", "probe"} {
		if r := d.exec(t, s, ""); r.ExitCode != 0 {
			t.Fatalf("step %s: %+v", s, r)
		}
	}
	log, err := os.ReadFile(filepath.Join(dir, "sudo-argv.log"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"integrate": "owner", "check": "seatrun",
		"deploy": "owner", "probe": "owner"}
	for step, user := range want {
		line := ""
		for _, ln := range strings.Split(string(log), "\n") {
			if strings.Contains(ln, "META_EXEC_STEP="+step) && strings.Contains(ln, "bash") {
				line = ln
			}
		}
		if !strings.Contains(line, "-u "+user+" ") {
			t.Fatalf("step %s run_as 应 %s: %q", step, user, line)
		}
	}
	_ = d.stream.CloseSend()

	// sudo -n 失：非零退→退码入 cmd_run＋exec 事件＝fail loud。
	dir2 := t.TempDir()
	sudo2 := sudoStub(t, dir2, 3)
	g2, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(dir2)
		c.SudoBin = sudo2
	})
	if err := g2.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g2, "tx", "P")
	if err := g2.DispatchSeat(ctx, "tx", "CTO", deploySpec()); err != nil {
		t.Fatal(err)
	}
	run2 := waitRun(t, g2, "tx", "CTO")
	d2 := driveSeat(t, g2, dir2, run2)
	d2.claim(t, "tx")
	waitPhase(t, g2, "tx", db.PhaseDeploying, 5*time.Second)
	r2 := d2.exec(t, "check", "")
	if r2.ExitCode != 3 {
		t.Fatalf("sudo 非零退应透传=3: %+v", r2)
	}
	crs, _ := g2.DB().CmdRuns(ctx, "tx")
	if len(crs) != 1 || crs[0].ExitCode != 3 {
		t.Fatalf("cmd_run 未记 sudo 非零退: %+v", crs)
	}
	var sawExec bool
	evs, _ := g2.DB().Events(ctx, "tx")
	for _, e := range evs {
		sawExec = sawExec || e.Kind == "exec"
	}
	if !sawExec {
		t.Fatal("sudo 失无 exec 事件——静默降级")
	}
	_ = d2.stream.CloseSend()
}

// TestDeploy_SeatDeadD2 锚7/D-2：部署中席死→deploy_failed 边＋note 含
// 「先核回滚态」与前任 run 线索；票回 待部署；锁释。
func TestDeploy_SeatDeadD2(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.SudoBin = sudoStub(t, d, 0)
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g, "tk", "P")
	mkToDeploy(t, g, "tk2", "P")
	wspec := deploySpec()
	wspec.Env = []string{"GO_HELPER_SEAT=1", "SEAT_MODE=worker", "META_ADOPT_GRACE=30",
		"PATH=" + os.Getenv("PATH")}
	if err := g.DispatchSeat(ctx, "tk", "CTO", wspec); err != nil {
		t.Fatal(err)
	}
	run := waitRun(t, g, "tk", "CTO")
	waitPhase(t, g, "tk", db.PhaseDeploying, 10*time.Second)
	if err := syscall.Kill(-run.PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, g, "tk", db.PhaseToDeploy, 10*time.Second)
	evs, _ := g.DB().Events(ctx, "tk")
	var note string
	for _, e := range evs {
		if e.Kind == string(db.EdgeDeployFailed) {
			note = e.Note
		}
	}
	if !strings.Contains(note, "先核回滚态") || !strings.Contains(note, run.RunID) {
		t.Fatalf("D-2 note 缺线索: %q", note)
	}
	// 锁已释：同项目他票可再派（spawn_needed 补发行应有）。
	if err := g.DispatchSeat(ctx, "tk2", "CTO", deploySpec()); err != nil {
		t.Fatalf("锁未释: %v", err)
	}
}

// TestPing_Probe 锚8：AdminGateway.Ping 通＋meta-probe.sh 对活网关退 0。
func TestPing_Probe(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	_ = g
	// Ping RPC 等价调用断言回包。
	bin := filepath.Join(dir, "meta")
	if out, err := exec.Command("go", "build", "-o", bin, "meta/cmd/meta").CombinedOutput(); err != nil {
		t.Fatalf("build meta: %v %s", err, out)
	}
	sock := filepath.Join(dir, "run", "meta.sock")
	probe, _ := filepath.Abs("../../scripts/meta-probe.sh")
	env := append(os.Environ(),
		"META_BIN="+bin, "META_GATEWAY_SOCK="+sock)
	cmd := exec.Command("bash", probe)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("meta-probe.sh 对活网关应退 0: %v %s", err, out)
	}
	// 对死 socket 应非零退。
	cmd2 := exec.Command("bash", probe)
	cmd2.Env = append(os.Environ(),
		"META_BIN="+bin, "META_GATEWAY_SOCK="+filepath.Join(dir, "run", "dead.sock"),
		"META_PROBE_TIMEOUT=1s")
	if err := cmd2.Run(); err == nil {
		t.Fatal("死 sock 探针应非零退")
	}
}
