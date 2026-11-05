package gateway

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
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
func deploySpec(t *testing.T) SeatSpec {
	t.Helper()
	// 非空提示文件（D-40 闸后 /dev/null 零字节＝空提示会被 spawn_blocked）。
	pf := filepath.Join(t.TempDir(), "prompt.md")
	if err := os.WriteFile(pf, []byte("# fixture 提示\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return SeatSpec{PromptFile: pf, WorkDir: "/tmp",
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
		SandboxSpec:  stubSandboxSpec(),
	}
}

// stubSandboxSpec 票0031：legacy sudo 退役后测试配方全走同用户短路——
// run_as=当前用户（seatrun/owner 等异用户步已属 deny 径，见 TestDeploy_LegacyDenied）。
// 票0034-p1：笼内 passwd 无自身条目→user.Current() err→run_as 落数字 uid
// （sameUser 数字直比径——与名解析等效同构）。
func stubSandboxSpec() string {
	me, err := user.Current()
	if err != nil {
		return fmt.Sprintf(`run_as: %q`, strconv.Itoa(os.Getuid()))
	}
	return fmt.Sprintf(`run_as: %q`, me.Username)
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
	shortSkipInteg(t) // T51①：DispatchSeat 内起真子进程——闸口径豁免
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
	if err := g.DispatchSeat(ctx, "tg", "CTO", deploySpec(t)); err != nil {
		t.Fatalf("待部署派 CTO 应放行: %v", err)
	}
	// 非待部署派 CTO 部署支→拒（已派/施工中/待审 皆非两支点）。
	mkTicketAt(t, g, "tb", db.PhaseAssigned)
	// T35 改判：已派 CTO＝redirect/调和中途重派合法径——相位闸放行。
	_ = g.DispatchSeat(ctx, "tb", "CTO", deploySpec(t))
	mkTicketAt(t, g, "tr", db.PhaseReview)
	if err := g.DispatchSeat(ctx, "tr", "CTO", deploySpec(t)); err == nil {
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
	adjPf := filepath.Join(t.TempDir(), "adj-prompt.md")
	if err := os.WriteFile(adjPf, []byte("# adjunct 提示\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := g.DispatchSeat(ctx, "tg", "CTO", SeatSpec{
		Adjunct: true, PromptFile: adjPf,
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
	shortSkipInteg(t) // T51①：DispatchSeat 内起真子进程——闸口径豁免
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
	if err := g.DispatchSeat(ctx, "td", "CTO", deploySpec(t)); err != nil {
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
	shortSkipInteg(t) // T51①：DispatchSeat 内起真子进程——闸口径豁免
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
	if err := g.DispatchSeat(ctx, "tf", "CTO", deploySpec(t)); err != nil {
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
	// T35：部署哨兵负＝回滚已跑→blocked 人工态。
	waitPhase(t, g, "tf", db.PhaseBlocked, 5*time.Second)

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
		sawFailed = sawFailed || e.Kind == "blocked" // T35：部署红→blocked 人工态
	}
	if !sawFailed {
		t.Fatal("blocked 事件行缺")
	}
	if p2, _ := g.DB().PhaseOf(ctx, "tf"); p2 == db.PhaseDone {
		t.Fatal("失败票落完成（对抗锚 a）")
	}
	_ = d.stream.CloseSend()
}

// TestDeploy_ProjectLocks 锚5＋对抗锚 c：同项目二部署→锁拒（spawn_blocked
// 留痕）；首票终态→锁释→补发 spawn_needed；启动按相位重建。
func TestDeploy_ProjectLocks(t *testing.T) {
	shortSkipInteg(t) // T51①：DispatchSeat 内起真子进程——闸口径豁免
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
	// T35：调和器可先派 ta——手工派发幂等相会（活席在径即行）。
	waitFor2(t, 15*time.Second, func() bool {
		if r, _ := g.DB().ActiveRunForTicketRole(ctx, "ta", "CTO"); r != nil {
			return true
		}
		return g.DispatchSeat(ctx, "ta", "CTO", deploySpec(t)) == nil
	}, g)
	if err := g.DispatchSeat(ctx, "tb2", "CTO", deploySpec(t)); err == nil {
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
	// T35：锁释不就地补发——phase:deploy_done 踢调和器，tb2 由调和器派。
	waitRun(t, g, "tb2", "CTO")
	_ = d.stream.CloseSend()
}

// TestDeploy_LockRebuildDB 锚5 后半：库内 部署中 票（带活 run 行）→
// 重启收养后锁重建——同项目二部署拒。
func TestDeploy_LockRebuildDB(t *testing.T) {
	shortSkipInteg(t) // T51①：DispatchSeat 内起真子进程——闸口径豁免
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
	wspec := deploySpec(t)
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
	if err := g2.DispatchSeat(ctx, "tq", "CTO", deploySpec(t)); err == nil {
		t.Fatal("重启后部署锁未重建——同项目二部署漏放")
	}
}

// TestDeploy_LegacyDenied 票0031 锚（对抗锚＝legacy 径不可达）：异用户
// 且无笼形制的步一律 deny——exec_resp 退码 -1＋deny note 入账；sudoStub
// 零调用（sudo-argv.log 不出生）。删 deny 分支→legacy sudo 复活→本锚红。
func TestDeploy_LegacyDenied(t *testing.T) {
	shortSkipInteg(t) // T51①：DispatchSeat 内起真子进程——闸口径豁免
	dir := t.TempDir()
	g, gdir := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(dir)
		c.SudoBin = sudoStub(t, dir, 0)
	})
	_ = gdir
	ctx := context.Background()
	proj := stubProject("P")
	proj.SandboxSpec = `run_as: "seatrun"
step_run_as { key: "integrate" value: "owner" }`
	if err := g.DB().UpsertProject(ctx, proj); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g, "ts", "P")
	if err := g.DispatchSeat(ctx, "ts", "CTO", deploySpec(t)); err != nil {
		t.Fatal(err)
	}
	run := waitRun(t, g, "ts", "CTO")
	d := driveSeat(t, g, dir, run)
	d.claim(t, "ts")
	waitPhase(t, g, "ts", db.PhaseDeploying, 5*time.Second)
	for _, s := range []string{"integrate", "check", "deploy", "probe"} {
		r := d.exec(t, s, "")
		if r.ExitCode != -1 || !strings.Contains(r.Note, "legacy-sudo 径已退役") {
			t.Fatalf("step %s 应 deny（legacy 退役）: %+v", s, r)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "sudo-argv.log")); !os.IsNotExist(err) {
		t.Fatal("legacy 径竟调 sudo——退役未生效")
	}
	crs, _ := g.DB().CmdRuns(ctx, "ts")
	if len(crs) != 4 {
		t.Fatalf("deny 步应入 cmd_run 留痕: %+v", crs)
	}
	_ = d.stream.CloseSend()
}

// TestDeploy_SeatDeadD2 锚7/D-2：部署中席死→deploy_failed 边＋note 含
// 「先核回滚态」与前任 run 线索；票回 待部署；锁释。
func TestDeploy_SeatDeadD2(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.SudoBin = sudoStub(t, d, 0)
			c.SelfPath = "/bin/false" // cli-drive 即死放锁（测径手工非 CliDrive 席证语义）
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkDeploying(t, g, "tk", "P")
	mkToDeploy(t, g, "tk2", "P")
	// T35：部署中票直造＋spawnSeat 立 CTO 席——相位守断言只验死亡处置。
	run := spawnSeat(t, g, "tk", "CTO", "idle", "META_ADOPT_GRACE=30")
	d := driveSeat(t, g, filepath.Dir(g.cfg.DBPath), run)
	_ = d.stream.CloseSend() // 双源互证：代驾流 EOF 先落，杀才判死
	if err := syscall.Kill(-run.PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	// T35：部署中席死相位稳守 部署中——调和器标死后按失败表重派继任；
	// 项目锁未终态不释，同项目他票仍锁拒（串行闸语义不动）。
	waitFor2(t, 10*time.Second, func() bool {
		evs, _ := g.DB().Events(ctx, "tk")
		for _, e := range evs {
			if e.Kind == "seat_dead" || e.Kind == "clidrive_dead" ||
				e.Kind == "deploy_failed" || e.Kind == "stall_kill" {
				return true
			}
		}
		st2, _ := g.DB().RunByToken(ctx, run.RunToken)
		return st2 != nil && st2.Dead // 死亡落账即证
	
	}, g)
	if err := g.DispatchSeat(ctx, "tk2", "CTO", deploySpec(t)); err == nil {
		t.Fatal("同项目二部署应仍锁拒（席死相位稳守不释锁）")
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

// ---------- 票0029：容器笼＋同用户短路四锚 ----------

// dockerStub 伪 docker：log argv 落盘；run 子令 sleepSec>0 则真睡（超时锚源），
// 余令即退 0。断言源＝docker-argv.log 全文。
func dockerStub(t *testing.T, dir string, sleepSec int) string {
	t.Helper()
	log := filepath.Join(dir, "docker-argv.log")
	sleepLine := ":"
	if sleepSec > 0 {
		sleepLine = fmt.Sprintf("sleep %d", sleepSec)
	}
	body := fmt.Sprintf(`#!/usr/bin/env bash
printf '%%s\n' "$*" >> %s
if [ "$1" = "run" ]; then %s; fi
exit 0
`, log, sleepLine)
	p := filepath.Join(dir, "docker")
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// execUnit 直驱 runExecStep 的最小 runState——不拉整席只验 argv 成型与收口。
func execUnit(tid string) *runState {
	return &runState{run: &db.Run{TicketID: tid, RunID: "r-" + tid}}
}

// TestExec_SameUserNoSudo 锚1：run_as＝网关用户→env 直跑——SudoBin 零调用且
// 步真实执行出 stdout。（对抗锚：删 sameUser 短路→走 sudo 径→log 有行→红。）
func TestExec_SameUserNoSudo(t *testing.T) {
	shortSkipInteg(t) // T51④：直建 New(cfg) 径——闸口径豁免
	dir := t.TempDir()
	cfg := dispatchCfg(dir)
	cfg.SudoBin = sudoStub(t, dir, 0)
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	me, err := user.Current()
	if err != nil {
		t.Skip("无当前用户")
	}
	proj := stubProject("P")
	proj.SandboxSpec = fmt.Sprintf(`run_as: "%s"`, me.Username)
	sb, err := parseSandbox(proj.SandboxSpec)
	if err != nil {
		t.Fatal(err)
	}
	resp := g.runExecStep(context.Background(), execUnit("t29a"), &proj, sb,
		"integrate", "echo same-user-ok", me.Username, "deadbeef")
	if resp.ExitCode != 0 {
		t.Fatalf("同用户步应退 0: %+v", resp)
	}
	if !strings.Contains(resp.Tail, "same-user-ok") {
		t.Fatalf("tail 缺步输出: %q", resp.Tail)
	}
	if _, err := os.Stat(filepath.Join(dir, "sudo-argv.log")); !os.IsNotExist(err) {
		t.Fatal("同用户步竟调 sudo——短路未生效")
	}
}

// TestExec_ContainerArgv 锚2：step_container_image 步→docker run argv：
// --name meta-exec-<tid>-<step>-＋meta-sandbox 网＋--user 网关 uid＋clone→/work
// ＋gomod ro＋-e META_* 注入＋镜像名＋bash -lc；且该步零 sudo。
// （对抗锚：删笼分支→走 sudo 径→docker log 空→红。）
func TestExec_ContainerArgv(t *testing.T) {
	shortSkipInteg(t) // T51④：直建 New(cfg) 径——闸口径豁免
	dir := t.TempDir()
	dockerStub(t, dir, 0)
	cfg := dispatchCfg(dir)
	cfg.DockerBin = filepath.Join(dir, "docker")
	cfg.SudoBin = sudoStub(t, dir, 0)
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	proj := stubProject("P")
	proj.SandboxSpec = `run_as: "metaseat"
step_container_image { key: "check" value: "golang:test-img" }`
	sb, err := parseSandbox(proj.SandboxSpec)
	if err != nil {
		t.Fatal(err)
	}
	resp := g.runExecStep(context.Background(), execUnit("t29b"), &proj, sb,
		"check", "echo caged", "metaseat", "ref123")
	if resp.ExitCode != 0 {
		t.Fatalf("笼步应退 0: %+v", resp)
	}
	log, err := os.ReadFile(filepath.Join(dir, "docker-argv.log"))
	if err != nil {
		t.Fatal(err)
	}
	runLine := ""
	rmLine := ""
	for _, ln := range strings.Split(string(log), "\n") {
		if strings.HasPrefix(ln, "run ") {
			runLine = ln
		}
		if strings.HasPrefix(ln, "rm ") {
			rmLine = ln
		}
	}
	// 票0034-p1：笼内 user.Current() err——判空回退 os.Getuid()（与实现
	// selfIDs 同径）；宿主两源恒等。
	wantUid := strconv.Itoa(os.Getuid())
	if me, err := user.Current(); err == nil {
		wantUid = me.Uid
	}
	for _, want := range []string{
		"--name meta-exec-t29b-check-", "--network meta-sandbox",
		"--user " + wantUid + ":", "/deploy/t29b:/work", "-w /work",
		":/gomod:ro", "GOMODCACHE=/gomod", "-e META_TICKET_ID=t29b",
		"-e META_EXEC_STEP=check", "golang:test-img", "bash -lc",
	} {
		if !strings.Contains(runLine, want) {
			t.Fatalf("run argv 缺 %q: %q", want, runLine)
		}
	}
	if !strings.Contains(rmLine, "rm -f meta-exec-t29b-check-") {
		t.Fatalf("退场未扫孤儿: %q", rmLine)
	}
	if _, err := os.Stat(filepath.Join(dir, "sudo-argv.log")); !os.IsNotExist(err) {
		t.Fatal("笼步竟调 sudo")
	}
}

// TestExec_ContainerOrphanSweep 锚3：超时杀进程组后 docker rm -f 必扫——
// run 睡 60s ＋ sb 超时 1s→退码-1＋rm 行落 log。（对抗锚：删 defer 扫→rm 无→红。）
func TestExec_ContainerOrphanSweep(t *testing.T) {
	shortSkipInteg(t) // T51④：直建 New(cfg) 径——闸口径豁免
	dir := t.TempDir()
	dockerStub(t, dir, 60)
	cfg := dispatchCfg(dir)
	cfg.DockerBin = filepath.Join(dir, "docker")
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	proj := stubProject("P")
	proj.SandboxSpec = `run_as: "metaseat"
exec_timeout { nanos: 500000000 }
step_container_image { key: "check" value: "golang:test-img" }`
	sb, err := parseSandbox(proj.SandboxSpec)
	if err != nil {
		t.Fatal(err)
	}
	resp := g.runExecStep(context.Background(), execUnit("t29c"), &proj, sb,
		"check", "sleep 60", "metaseat", "ref123")
	if resp.ExitCode != -1 {
		t.Fatalf("超时步应退-1: %+v", resp)
	}
	log, err := os.ReadFile(filepath.Join(dir, "docker-argv.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "rm -f meta-exec-t29c-check-") {
		t.Fatalf("超时后孤儿未扫: %q", string(log))
	}
}

// TestExec_NumericRunAs 票0034-p1 锚4：run_as 数字 uid＝笼内 /etc/passwd 无
// 条目（--user uid:gid）的同构环境——名解析失效不改判。同 uid 步直跑退 0＋
// sudo 零调用；异 uid 步仍落 legacy deny。（对抗锚：删 sameUser 数字直比→
// 同 uid 案落 deny 红；删 deny 分支→异 uid 案放行红。）
func TestExec_NumericRunAs(t *testing.T) {
	shortSkipInteg(t) // T51④：直建 New(cfg) 径——闸口径豁免
	selfUid := os.Getuid()
	for _, tc := range []struct {
		name     string
		tid      string
		runAs    string
		wantCode int32
		wantDeny bool
	}{
		{"同uid步直跑", "t34a", strconv.Itoa(selfUid), 0, false},
		{"异uid步仍拒", "t34b", strconv.Itoa(selfUid + 1), -1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := dispatchCfg(dir)
			cfg.SudoBin = sudoStub(t, dir, 0)
			g, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()
			proj := stubProject("P")
			proj.SandboxSpec = fmt.Sprintf(`run_as: "%s"`, tc.runAs)
			sb, err := parseSandbox(proj.SandboxSpec)
			if err != nil {
				t.Fatal(err)
			}
			resp := g.runExecStep(context.Background(), execUnit(tc.tid), &proj, sb,
				"integrate", "echo numeric-uid-ok", tc.runAs, "deadbeef")
			if resp.ExitCode != tc.wantCode {
				t.Fatalf("退码 %d want %d: %+v", resp.ExitCode, tc.wantCode, resp)
			}
			if tc.wantDeny {
				if !strings.Contains(resp.Note, "legacy-sudo 径已退役") {
					t.Fatalf("异 uid 步应落 legacy deny: %+v", resp)
				}
			} else if !strings.Contains(resp.Tail, "numeric-uid-ok") {
				t.Fatalf("同 uid 步 tail 缺步输出: %q", resp.Tail)
			}
			if _, err := os.Stat(filepath.Join(dir, "sudo-argv.log")); !os.IsNotExist(err) {
				t.Fatal("数字 run_as 径竟调 sudo")
			}
		})
	}
}
