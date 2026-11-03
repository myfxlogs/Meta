package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"meta/internal/db"
)

// ---------- 票0044 部署态判三补测试基建 ----------

// stubDeploySeams 桩缝替身（锚件「Probe/stamp/日志面经缝桩测」）：
// binVersion 表查、repoCodeHead 定值、deployProbe 定 err。
func stubDeploySeams(t *testing.T, vers map[string]string, head string, probeErr error) {
	t.Helper()
	bv, rh, dp := binVersion, repoCodeHead, deployProbe
	t.Cleanup(func() { binVersion, repoCodeHead, deployProbe = bv, rh, dp })
	binVersion = func(p string) (string, error) {
		if s, ok := vers[p]; ok {
			return s, nil
		}
		return "", fmt.Errorf("version 不可跑: %s", p)
	}
	repoCodeHead = func(string) (string, error) { return head, nil }
	deployProbe = func(string, time.Duration) error { return probeErr }
}

// writeSentinel 死席哨兵落盘（run/artifacts/<runid>/sentinel.log）。
func writeSentinel(t *testing.T, dir, runID, body string) {
	t.Helper()
	d := filepath.Join(dir, "run", "artifacts", runID)
	if err := os.MkdirAll(d, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "sentinel.log"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeDeployLog 造 deploy exec 日志（run/exec/<tid>-deploy-<name>.log——
// 名带序号字典序即时间序，后写者＝最新份）。
func writeDeployLog(t *testing.T, dir, tid, name, body string) {
	t.Helper()
	p := filepath.Join(dir, "run", "exec", fmt.Sprintf("%s-deploy-%s.log", tid, name))
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// deadSeatUnit 直驱 onRunDead 的死席 fixture——不拉整席只验席死判分支。
func deadSeatUnit(tid, runID string) *runState {
	return newRunState(&db.Run{TicketID: tid, RunID: runID, Role: "CTO", PID: 424242})
}

// mkDeploying 造部署中票（项目挂接）——死席判 fixture：mkToDeploy 只到
// 待部署，部署中相位须 mkTicketAt 直推。
func mkDeploying(t *testing.T, g *Gateway, id, project string) {
	t.Helper()
	mkTicketAt(t, g, id, db.PhaseDeploying)
	if err := g.DB().SetTicketProject(context.Background(), id, project); err != nil {
		t.Fatal(err)
	}
}

// deployEdgeSeen 读库事件史找目标边（返回 note 供留痕断言）。
func deployEdgeSeen(t *testing.T, g *Gateway, tid string, kind string) (bool, string) {
	t.Helper()
	evs, err := g.DB().Events(context.Background(), tid)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range evs {
		if e.Kind == kind {
			return true, e.Note
		}
	}
	return false, ""
}

// emitted 观察口事件是否已发（deploy_ok 等 emit-only 帧——缓冲道内直读）。
func emitted(g *Gateway, kind, tid string, dur time.Duration) bool {
	for _, e := range collectEvents(g, dur) {
		if e.Kind == kind && e.TicketID == tid {
			return true
		}
	}
	return false
}

// TestT44_AttachDrainDone 锚2 done 支＋锚6 排干行＋锚7 签名绝迹：
// 部署中席死＋sentinel.log 含 META_DEPLOY:done→deploy_done→完成＋
// deploy_ok 事件＋integrate/deploy 双锁释（同项目二票可派）；
// deploy_failed 绝迹。Probe 桩红——排干先于态判，socket 红不挡盘上哨兵。
func TestT44_AttachDrainDone(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.SudoBin = sudoStub(t, filepath.Dir(c.DBPath), 0)
			c.SelfPath = "/bin/false" // cli-drive 即死放锁（测径手工非 CliDrive 席证语义）
	})
	stubDeploySeams(t, nil, "", fmt.Errorf("probe 结构性红"))
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkDeploying(t, g, "tk44", "P")
	mkToDeploy(t, g, "tk44b", "P")
	// T35：部署中票立 CTO 席——spawnSeat 直立（调和器见对齐零 churn）。
	run := spawnSeat(t, g, "tk44", "CTO", "idle", "META_ADOPT_GRACE=30")
	d := driveSeat(t, g, dir, run)
	_ = d.stream.CloseSend() // 双源互证：代驾流 EOF 先落，杀才判死
	writeSentinel(t, dir, run.RunID, "META_EXEC:deploy e109692\nMETA_DEPLOY:done\n")
	if err := syscall.Kill(-run.PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, g, "tk44", db.PhaseDone, 10*time.Second)

	if ok, _ := deployEdgeSeen(t, g, "tk44", "deploy_done"); !ok {
		t.Fatal("deploy_done 边缺——排干未消费 done 哨兵（对抗锚）")
	}
	if ok, _ := deployEdgeSeen(t, g, "tk44", "blocked"); ok {
		t.Fatal("物理全绿仍出 deploy_failed——签名未绝迹（锚7）")
	}
	if !emitted(g, "deploy_ok", "tk44", 300*time.Millisecond) {
		t.Fatal("deploy_ok 事件缺（对抗锚：锁放行序）")
	}
	// T35：锁释后调和器自动派 tk44b——活席行即放锁证。
	waitFor2(t, 15*time.Second, func() bool {
		rs, _ := g.DB().RunsForTicket(ctx, "tk44b")
		return len(rs) > 0
	})
}

// TestT44_AttachDrainFailed 锚2 failed 支：sentinel.log 含
// META_DEPLOY:failed→deploy_failed＋note 记哨兵在盘——终哨兵定边，
// 态判桩全绿布景仍翻负（排干先于态判）。
func TestT44_AttachDrainFailed(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = dispatchCfg(filepath.Dir(c.DBPath)) })
	inst := filepath.Join(dir, "meta.inst")
	nb := filepath.Join(dir, "bin", "meta.new")
	stubDeploySeams(t, map[string]string{inst: "abc-deploy", nb: "abc-deploy"}, "abc", nil)
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkDeploying(t, g, "tf44", "P")
	rid := "r-tf44-dead"
	writeSentinel(t, dir, rid, "META_DEPLOY:failed\n")
	writeDeployLog(t, dir, "tf44", "1",
		fmt.Sprintf("upgrade: 已装 %s → %s（prev=true）\n", nb, inst))
	g.onRunDead(deadSeatUnit("tf44", rid))
	// T35：failed 哨兵＝回滚已跑→blocked 人工态（非回待部署）。
	waitPhase(t, g, "tf44", db.PhaseBlocked, 5*time.Second)
	ok, note := deployEdgeSeen(t, g, "tf44", "deploy_failed")
	if !ok {
		t.Fatal("failed 哨兵未记 deploy_failed 账")
	}
	if !strings.Contains(note, "哨兵在盘") {
		t.Fatalf("哨兵在盘未留痕: %q", note)
	}
}

// TestT44_AttachStateOK 锚3 兜底绿径：席死＋sentinel.log 无 META_DEPLOY
// 行＋态判三要件齐（exec 日志已装行/装着戳==期戳/Probe 桩绿）→
// deploy_done。已装行 newBin 走相对径——repo_url 解析锚同时验证。
func TestT44_AttachStateOK(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = dispatchCfg(filepath.Dir(c.DBPath)) })
	inst := filepath.Join(dir, "meta.inst")
	nb := filepath.Join(dir, "bin", "meta.new")
	stubDeploySeams(t, map[string]string{inst: "abc-deploy", nb: "abc-deploy"}, "abc", nil)
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, db.Project{
		ID: "P", RepoURL: dir, DefaultBranch: "master"}); err != nil {
		t.Fatal(err)
	}
	mkDeploying(t, g, "ts44", "P")
	writeSentinel(t, dir, "r-ts44-dead", "META_EXEC:deploy e109692\nMETA_DELIVER:t/x\n")
	writeDeployLog(t, dir, "ts44", "1",
		fmt.Sprintf("build ok\nupgrade: 已装 bin/meta.new → %s（prev=true）\n", inst))
	g.onRunDead(deadSeatUnit("ts44", "r-ts44-dead"))
	waitPhase(t, g, "ts44", db.PhaseDone, 5*time.Second)
	if ok, _ := deployEdgeSeen(t, g, "ts44", "deploy_done"); !ok {
		t.Fatal("态判绿径未出 deploy_done")
	}
	if !emitted(g, "deploy_ok", "ts44", 200*time.Millisecond) {
		t.Fatal("deploy_ok 事件缺")
	}
}

// TestT44_AttachStateRed 锚3 反锚：席死＋无终哨兵＋三要件任一翻→
// deploy_failed（原 note 文案不变「先核回滚态」——D-2 真败径不退）。
func TestT44_AttachStateRed(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = dispatchCfg(filepath.Dir(c.DBPath)) })
	inst := filepath.Join(dir, "meta.inst")
	nb := filepath.Join(dir, "bin", "meta.new")
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, db.Project{
		ID: "P", RepoURL: dir, DefaultBranch: "master"}); err != nil {
		t.Fatal(err)
	}
	goodLine := fmt.Sprintf("upgrade: 已装 bin/meta.new → %s（prev=true）\n", inst)

	cases := []struct {
		name string
		log  string // exec 日志体（空＝不写）
		vers map[string]string
		head string
		perr error
	}{
		{name: "无已装证据", log: "", vers: map[string]string{inst: "abc-deploy"}, head: "abc"},
		{name: "戳不符", log: goodLine,
			vers: map[string]string{inst: "zzz-deploy", nb: "abc-deploy"}, head: "abc"},
		{name: "Probe红", log: goodLine,
			vers: map[string]string{inst: "abc-deploy", nb: "abc-deploy"}, head: "abc",
			perr: fmt.Errorf("socket 死")},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubDeploySeams(t, tc.vers, tc.head, tc.perr)
			tid := fmt.Sprintf("tr%d", i)
			mkDeploying(t, g, tid, "P")
			if tc.log != "" {
				writeDeployLog(t, dir, tid, "1", tc.log)
			}
			g.onRunDead(deadSeatUnit(tid, "r-"+tid))
			// T35：相位稳守 部署中＋deploy_failed 账（失败表 rollback≤1
			// 由调和器判——非回待部署）。
			waitPhase(t, g, tid, db.PhaseDeploying, 5*time.Second)
			ok, note := deployEdgeSeen(t, g, tid, "deploy_failed")
			if !ok {
				t.Fatal("要件翻未记 deploy_failed 账")
			}
			if !strings.Contains(note, "先核回滚态") {
				t.Fatalf("note 缺先核回滚态: %q", note)
			}
		})
	}
}

// TestT44_ReconcileDrainDone 锚4：PhaseDeploying 死 run＋sentinel.log
// 含 done→对账判 deploy_done（boot 序 Probe 桩红不挡——排干先于态判）。
func TestT44_ReconcileDrainDone(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = dispatchCfg(filepath.Dir(c.DBPath)) })
	stubDeploySeams(t, nil, "", fmt.Errorf("probe 结构性红"))
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkDeploying(t, g, "tr44", "P")
	rid := "r-tr44-dead"
	if err := g.DB().CreateRun(ctx, &db.Run{
		RunID: rid, TicketID: "tr44", Role: "CTO", PID: 424243,
		RunToken:   rid + "-tok",
		SocketPath: filepath.Join(dir, "run", "seats", rid+".sock")}); err != nil {
		t.Fatal(err)
	}
	writeSentinel(t, dir, rid, "META_DEPLOY:done\n")
	g.reconcile(ctx)
	waitPhase(t, g, "tr44", db.PhaseDone, 5*time.Second)
	if ok, _ := deployEdgeSeen(t, g, "tr44", "deploy_done"); !ok {
		t.Fatal("对账排干未出 deploy_done")
	}
	if ok, _ := deployEdgeSeen(t, g, "tr44", "blocked"); ok {
		t.Fatal("排干有证仍翻负")
	}
}

// TestT44_ReconcileDrainFailed 锚4 反径：对账死席＋sentinel failed→
// deploy_failed（哨兵在盘留痕）；无终哨兵＋态判无据→deploy_failed。
func TestT44_ReconcileDrainFailed(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { *c = dispatchCfg(filepath.Dir(c.DBPath)) })
	stubDeploySeams(t, nil, "", fmt.Errorf("probe 结构性红"))
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ tid, sentinel string }{
		{"tr45", "META_DEPLOY:failed\n"},
		{"tr46", "META_EXEC:probe ok\n"}, // 无 META_DEPLOY 终哨兵
	} {
		mkDeploying(t, g, tc.tid, "P")
		rid := "r-" + tc.tid + "-dead"
		if err := g.DB().CreateRun(ctx, &db.Run{
			RunID: rid, TicketID: tc.tid, Role: "CTO", PID: 424244,
			RunToken:   rid + "-tok",
			SocketPath: filepath.Join(dir, "run", "seats", rid+".sock")}); err != nil {
			t.Fatal(err)
		}
		writeSentinel(t, dir, rid, tc.sentinel)
	}
	g.reconcile(ctx)
	for _, tc := range []struct {
		tid       string
		wantPhase db.Phase
	}{
		{"tr45", db.PhaseBlocked}, {"tr46", db.PhaseBlocked},
	} {
		waitPhase(t, g, tc.tid, tc.wantPhase, 5*time.Second)
		if ok, _ := deployEdgeSeen(t, g, tc.tid, "deploy_failed"); !ok {
			t.Fatalf("%s 对账死席未记 deploy_failed 账", tc.tid)
		}
	}
}

// TestT44_DeployStateOKResourced 锚5：换源判据——cmd_run 零 deploy 行＋
// exec 日志已装行＋装着戳==期戳→true；删已装行/戳不符→false；cmd_run
// deploy exit=0 行在（旁证通道，META_INSTALL 定装着位）→证据件真。
func TestT44_DeployStateOKResourced(t *testing.T) {
	g, dir := newTestGateway(t, nil)
	ctx := context.Background()
	inst := filepath.Join(dir, "meta.inst")
	nb := filepath.Join(dir, "bin", "meta.new")
	if err := g.DB().UpsertProject(ctx, db.Project{
		ID: "P", RepoURL: dir, DefaultBranch: "master"}); err != nil {
		t.Fatal(err)
	}
	mkTicketAt(t, g, "tds", db.PhaseDeploying)
	if err := g.DB().SetTicketProject(ctx, "tds", "P"); err != nil {
		t.Fatal(err)
	}
	stubDeploySeams(t, map[string]string{inst: "abc-deploy", nb: "abc-deploy"}, "abc", nil)

	// 零 cmd_run＋已装行＋戳对→true。
	writeDeployLog(t, dir, "tds", "1",
		fmt.Sprintf("upgrade: 已装 bin/meta.new → %s（prev=true）\n", inst))
	if !g.deployStateOK(ctx, "tds") {
		t.Fatal("已装行＋戳对＋Probe 绿未判成")
	}
	// 删已装行（新份顶替——glob 取最新）→false。
	writeDeployLog(t, dir, "tds", "2", "upgrade: 预检失败——未装\n")
	if g.deployStateOK(ctx, "tds") {
		t.Fatal("无已装行竟判成")
	}
	// 戳不符→false。
	binVersion = func(p string) (string, error) {
		if p == nb {
			return "abc-deploy", nil
		}
		return "zzz-deploy", nil
	}
	writeDeployLog(t, dir, "tds", "3",
		fmt.Sprintf("upgrade: 已装 bin/meta.new → %s（prev=true）\n", inst))
	if g.deployStateOK(ctx, "tds") {
		t.Fatal("装戳≠期戳竟判成")
	}

	// cmd_run 旁证通道：零已装行＋deploy exit=0 行＋META_INSTALL 装着位
	// 戳对期戳（code-head 次选源）→true。
	mkTicketAt(t, g, "tds2", db.PhaseDeploying)
	if err := g.DB().SetTicketProject(ctx, "tds2", "P"); err != nil {
		t.Fatal(err)
	}
	stubDeploySeams(t, map[string]string{inst: "abc-deploy"}, "abc", nil)
	t.Setenv("META_INSTALL", inst)
	if err := g.DB().InsertCmdRun(ctx, &db.CmdRun{
		TicketID: "tds2", Step: "deploy", ExitCode: 0}); err != nil {
		t.Fatal(err)
	}
	if !g.deployStateOK(ctx, "tds2") {
		t.Fatal("cmd_run 旁证通道未判成")
	}
}
