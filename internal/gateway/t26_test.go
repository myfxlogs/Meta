package gateway

// T26 派发/部署径确定性收口——七病一病一锚。

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"meta/internal/db"
)

// 病1 B1 锁查占原子化：同项目两张待部署票并发 Dispatch——恰一席过闸，
// 另一席 spawn_blocked 拒（旧径闸间缝隙两骑并跑＝0045/0046 实证）。
// 对抗锚提名行：dispatchMu 临界区——删则并发双派可同过锁闸。
func TestT26_B1ConcurrentDispatch(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g, "ta26a", "P")
	mkToDeploy(t, g, "ta26b", "P")
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, tid := range []string{"ta26a", "ta26b"} {
		wg.Add(1)
		go func(i int, tid string) {
			defer wg.Done()
			errs[i] = g.DispatchSeat(ctx, tid, "CTO", deploySpec(t))
		}(i, tid)
	}
	wg.Wait()
	ok, blocked := 0, 0
	for _, e := range errs {
		if e == nil {
			ok++
		} else {
			blocked++
		}
	}
	if ok != 1 || blocked != 1 {
		t.Fatalf("并发双派应恰一过闸：ok=%d blocked=%d errs=%v", ok, blocked, errs)
	}
	// 空桶钉：project 挂不上（零登记）→拒派 fail-closed，不留空键锁桶。
	mkTicketAt(t, g, "ta26c", db.PhaseToDeploy)
	if err := g.DispatchSeat(ctx, "ta26c", "CTO", deploySpec(t)); err == nil {
		t.Fatal("空项目待部署票应拒派 fail-closed")
	}
	evs, _ := g.DB().Events(ctx, "ta26c")
	sawBlocked := false
	for _, e := range evs {
		sawBlocked = sawBlocked || e.Kind == "spawn_blocked"
	}
	if !sawBlocked {
		t.Fatal("空项目拒派缺 spawn_blocked 留痕")
	}
}

// 病2 spawn 必响：票不存在/登记表读败——旧径静默 return＝席位零起零报
// （0047 实证）。对抗锚提名行：spawnFail 调用点——删则静默回潮。
func TestT26_SpawnFailLoud(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	g.dispatchOnce(spawnReq{ticketID: "ghost", role: "施工"})
	waitEvent(t, g, "spawn_failed", "ghost", 3*time.Second)
	evs, _ := g.DB().Events(ctx2(t), "ghost")
	saw := false
	for _, e := range evs {
		saw = saw || e.Kind == "spawn_failed"
	}
	if !saw {
		t.Fatal("不存在票的派发缺 spawn_failed 落账")
	}
}

func ctx2(t *testing.T) context.Context { t.Helper(); return context.Background() }

// 病3 克隆并发互斥：同票同锁、异票异锁——同票并发 exec 不再撞同一
// deploy clone 目录（0046 integrate clone 红实证）。
// 对抗锚提名行：cloneLockFor map 键——翻成单锁则异票互斥过串。
func TestT26_CloneMutexPerTicket(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	if g.cloneLockFor("x1") != g.cloneLockFor("x1") {
		t.Fatal("同票应同锁")
	}
	if g.cloneLockFor("x1") == g.cloneLockFor("x2") {
		t.Fatal("异票应异锁")
	}
	// 并发串行实证：同票 32 并发 ensureDeployClone（repoURL 空=空目录径）
	// 全绿即互斥成立（无锁则目录建删交错——本径虽幂等，锁语义由上行断）。
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := g.ensureDeployClone(context.Background(), "x1", "", ""); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

// 病4 完成态判事件化：四步 cmd_run 齐绿即落 done——哨兵蒸发/席死径
// 绕开判定不再挡收口（0044/0043 未竟域）。缺一步不落。
// 对抗锚提名行：deployStepsDone 存在性判——翻成无判则半成品票误完成。
func TestT26_DeployStepsDone(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "td4", db.PhaseDeploying)
	steps := []string{"integrate", "check", "deploy", "probe"}
	for i, s := range steps[:3] { // 先三步——不齐不落
		if err := g.DB().InsertCmdRun(ctx, &db.CmdRun{
			TicketID: "td4", RunID: "r1", Step: s, ExitCode: 0}); err != nil {
			t.Fatal(err)
		}
		_ = i
	}
	g.maybeDeployDone(ctx, "td4")
	if p, _ := g.DB().PhaseOf(ctx, "td4"); p == db.PhaseDone {
		t.Fatal("三步账即落 done——缺步误收口")
	}
	if err := g.DB().InsertCmdRun(ctx, &db.CmdRun{
		TicketID: "td4", RunID: "r1", Step: steps[3], ExitCode: 0}); err != nil {
		t.Fatal(err)
	}
	g.maybeDeployDone(ctx, "td4")
	waitPhase(t, g, "td4", db.PhaseDone, 3*time.Second)
	// 幂等：再走不变。
	g.maybeDeployDone(ctx, "td4")
	if p, _ := g.DB().PhaseOf(ctx, "td4"); p != db.PhaseDone {
		t.Fatalf("幂等回落相位=%s", p)
	}
}

// 病5 降级出路：CTO 待部署票候壳尽→clidrive_fallback＋spec 转 CliDrive＋
// failSet 清＋重入队（非悬死）；双锁仍持——本票续部署非放人插队。
// 对抗锚提名行：clidrive_fallback 分支——删则悬死回潮。
func TestT26_DegradedCliDriveFallback(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.SelfPath = "/bin/false" // CliDrive 席即退——只验接线不验活
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g, "td5", "P")
	sp := deploySpec(t)
	sp.CliDrive = false // 模拟 AI 席派发走到候壳尽的规格
	g.mu.Lock()
	g.specs[specKey{"td5", "CTO"}] = sp
	g.failSet["td5"] = map[string]bool{"devin/swe-2-max": true, "claude": true}
	g.mu.Unlock()
	g.shellsExhausted(ctx, spawnReq{ticketID: "td5", role: "CTO"})
	waitEvent(t, g, "clidrive_fallback", "td5", 3*time.Second)
	g.mu.Lock()
	got := g.specs[specKey{"td5", "CTO"}].CliDrive
	fs := len(g.failSet["td5"])
	g.mu.Unlock()
	if !got || fs != 0 {
		t.Fatalf("应转 CliDrive 且清 failSet: cli=%v fs=%d", got, fs)
	}
	// 入队的 CliDrive 派发跑通到 spawn 尝试（/bin/false 即退→连败账进 failSet）。
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		runs, _ := g.DB().RunsForTicket(ctx, "td5")
		if len(runs) > 0 && runs[0].Shell == "cli-drive" {
			return
		}
		<-time.After(30 * time.Millisecond)
	}
	t.Fatal("cli-drive 兜底席未起")
}

// 病6 接线：spec.CliDrive 不走壳路由/注入/D-40——spawn_failed 必响；
// 连败≥3 截断＋放还部署双锁（spawn_needed 重放不成死循环）。
// 对抗锚提名行：fails>=3 截断支与 releaseDeployLocks——删则死循环烧派。
func TestT26_CliDriveSpawnBound(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.SelfPath = "/bin/false" // CliDrive 席即退——逐次连败
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g, "td6", "P")
	spec := SeatSpec{CliDrive: true}
	req := spawnReq{ticketID: "td6", role: "CTO"}
	deadCount := func() int {
		n := 0
		runs, _ := g.DB().RunsForTicket(ctx, "td6")
		for _, r := range runs {
			if r.Shell == "cli-drive" && r.Dead {
				n++
			}
		}
		return n
	}
	for i := 0; i < 3; i++ { // 逐次连败进持久账
		g.cliDriveSpawn(ctx, req, spec)
		deadline := time.Now().Add(5 * time.Second)
		for deadCount() <= i && time.Now().Before(deadline) {
			<-time.After(30 * time.Millisecond)
		}
	}
	g.mu.Lock()
	g.projectLocks["P"] = map[string]bool{lockIntegrate: true, lockDeploy: true}
	g.mu.Unlock()
	g.cliDriveSpawn(ctx, req, spec) // 第 4 次：死账≥3 截断＋锁放还
	g.mu.Lock()
	held := g.projectLocks["P"]
	g.mu.Unlock()
	if held[lockIntegrate] || held[lockDeploy] {
		t.Fatalf("截断后锁应放还: %v", held)
	}
	runs, _ := g.DB().RunsForTicket(ctx, "td6")
	if len(runs) != 3 {
		t.Fatalf("截断后不该有新席: runs=%d", len(runs))
	}
	sawFail := false
	evs, _ := g.DB().Events(ctx, "td6")
	for _, e := range evs {
		sawFail = sawFail || e.Kind == "spawn_failed"
	}
	if !sawFail {
		t.Fatal("连败缺 spawn_failed 落账（必响锚）")
	}
}

// 病7 停滞警报修因＋僵尸席 CLI 兜底：
//
//	a) 未 attach 的僵尸席也挂停滞钟（旧径钟挂 attach 后——零活性帧永不报）；
//	b) CTO 部署票僵→处决＋stall_kill＋死判径补 spawn_needed（CliDrive 兜底链）；
//	c) 非 CTO 席只报不杀（§5.2 归 CTO 裁语义不退）。
//
// 对抗锚提名行：onSeatAlive armAlarm 行——删则 a 红；onStall 处决支——删则 b 红。
func TestT26_StallKillDeploySeat(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.ProgressTimeout = 200 * time.Millisecond
	})
	ctx := context.Background()
	// a) 施工僵尸席（idle＝永不 attach）：起席即挂钟→stall_suspect 必响。
	mkTicketAt(t, g, "tz", db.PhaseAssigned)
	if err := g.DispatchSeat(ctx, "tz", "施工", SeatSpec{
		PromptFile: deploySpec(t).PromptFile, WorkDir: "/tmp",
		Env: []string{"GO_HELPER_SEAT=1", "SEAT_MODE=idle", "PATH=" + os.Getenv("PATH")},
	}); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "stall_suspect", "tz", 8*time.Second)
	// c) 非 CTO 席只报不杀——stall_kill 不得落。
	for _, e := range collectEvents(g, 400*time.Millisecond) {
		if e.Kind == "stall_kill" && e.TicketID == "tz" {
			t.Fatal("非 CTO 席被处决——§5.2 只报不杀语义破")
		}
	}

	// b) CTO 待部署票僵尸席：处决→stall_kill→死判径补 spawn_needed。
	mkToDeploy(t, g, "tzd", "P2") // P2 未登记：锁径与本测无关——直接 Spawn 建席
	run, err := g.Spawn(ctx, "tzd", "CTO", "testshell",
		[]string{os.Args[0], "-test.run=TestHelperProcess", "-test.v=false"},
		append(os.Environ(), "GO_HELPER_SEAT=1", "SEAT_MODE=idle"))
	if err != nil {
		t.Fatal(err)
	}
	g.onStall("tzd")
	waitEvent(t, g, "stall_kill", "tzd", 5*time.Second)
	waitEvent(t, g, "spawn_needed", "tzd", 8*time.Second)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(run.PID) {
			break
		}
		<-time.After(30 * time.Millisecond)
	}
	if processAlive(run.PID) {
		t.Fatal("僵尸部署席未被处决")
	}
	// 处决 2 次后止杀：再响只报不杀（防循环）。
	for i := 0; i < 2; i++ {
		g.onStall("tzd")
	}
	nKill := 0
	evs, _ := g.DB().Events(ctx, "tzd")
	for _, e := range evs {
		if e.Kind == "stall_kill" {
			nKill++
		}
	}
	if nKill > 2 {
		t.Fatalf("处决应≤2 次止杀: %d", nKill)
	}
}
