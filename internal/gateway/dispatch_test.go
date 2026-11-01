package gateway

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// dispatchCfg 真壳调度径测试配置：SeatArgv 替成 helper 席位进程。
func dispatchCfg(dir string) Config {
	return Config{
		DBPath:           filepath.Join(dir, "meta.db"),
		RunDir:           filepath.Join(dir, "run"),
		SocketPath:       filepath.Join(dir, "run", "meta.sock"),
		ProgressTimeout:  30 * time.Minute,
		AuthTimeout:      2 * time.Second,
		AdoptDialTimeout: 2 * time.Second,
		QuickExit:        3 * time.Second,
		Stagger:          20 * time.Millisecond,
		SeatArgv:         []string{os.Args[0], "-test.run=TestHelperProcess", "-test.v=false"},
	}
}

func dispatchSpec() SeatSpec {
	return SeatSpec{
		PromptFile: "/dev/null", WorkDir: "/tmp",
		Env: []string{"GO_HELPER_SEAT=1", "SEAT_MODE=worker", "META_ADOPT_GRACE=30",
			"PATH=" + os.Getenv("PATH")},
	}
}

// TestDispatch_SeatLifecycle 调度一镜到底：DispatchSeat 走相位→路由 spawn→
// attach+claim→施工中；杀席→failSet 换壳→下一壳顶上（①级换壳的机器径）。
func TestDispatch_SeatLifecycle(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		dir := filepath.Dir(c.DBPath)
		*c = dispatchCfg(dir)
	})
	ctx := context.Background()
	if err := g.DB().CreateTicket(ctx, "td", "dispatch 票", nil); err != nil {
		t.Fatal(err)
	}
	if err := g.DispatchSeat(ctx, "td", "施工", dispatchSpec()); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, g, "td", db.PhaseWorking, 10*time.Second)

	runs, _ := g.DB().RunsForTicket(ctx, "td")
	if len(runs) != 1 || runs[0].Shell != "devin/swe-2-high" {
		t.Fatalf("首尝试应为 devin/swe-2-high: %+v", runs[0].Shell)
	}
	pid := runs[0].PID

	// 杀席位进程组——①级中途死：回队→failSet 排除 devin→claude 顶上。
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "requeue", "td", 8*time.Second)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		runs, _ = g.DB().RunsForTicket(ctx, "td")
		if len(runs) >= 2 {
			break
		}
		<-time.After(50 * time.Millisecond)
	}
	if len(runs) < 2 {
		evs, _ := g.DB().Events(ctx, "td")
		t.Fatalf("未自动换壳重起，runs=%d 事件=%v", len(runs), evs)
	}
	if runs[1].Shell != "claude" {
		t.Fatalf("第二尝试应换 claude: %s", runs[1].Shell)
	}
	waitPhase(t, g, "td", db.PhaseWorking, 10*time.Second)
}

// TestDispatch_FrozenRefuses 冻票拒派＋排队 spawn 判弃（§5.5 D8）。
func TestDispatch_FrozenRefuses(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		dir := filepath.Dir(c.DBPath)
		*c = dispatchCfg(dir)
	})
	ctx := context.Background()
	mkTicketAt(t, g, "tf", db.PhaseAssigned)
	if err := g.DB().SetBudgetFrozen(ctx, "tf", true); err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	g.frozen["tf"] = true
	g.mu.Unlock()
	if err := g.DispatchSeat(ctx, "tf", "施工", dispatchSpec()); err == nil {
		t.Fatal("冻票派发应被拒")
	}
	// 排队 spawn 事件在消费端判弃：手动入队，观察口不许出 spawn_needed。
	g.enqueueSpawn("tf", "施工")
	for _, e := range collectEvents(g, 300*time.Millisecond) {
		if e.Kind == "spawn_needed" && e.TicketID == "tf" {
			t.Fatal("冻票漏出 spawn_needed")
		}
	}
}

// TestBudget_ThreeGates 预算三闸＋冻结序＋冻结取消未决（验收锚4）。
// 对抗性锚提名行：attach.go 的 `b.Used*5 >= b.Cap*4`、`b.Used >= b.Cap`、
// `b.UsedTotal >= b.HardCap`——删/翻任一比较，本表必红。
func TestBudget_ThreeGates(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
		c.SeatArgv = []string{os.Args[0], "-test.run=TestHelperProcess", "-test.v=false"}
	})
	ctx := context.Background()
	mkTicketAt(t, g, "tb1", db.PhaseAssigned)
	if err := g.DB().SetBudgetCaps(ctx, "tb1", 100, 1000); err != nil {
		t.Fatal(err)
	}
	// idle 席不自己连——测试代它 Attach 推 Usage 帧。
	run := spawnSeat(t, g, "tb1", "施工", "idle")
	_, c := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))
	stream, err := c.Attach(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	send := func(m *pbv1.SeatMsg) {
		if err := stream.Send(m); err != nil {
			t.Fatal(err)
		}
	}
	send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: run.RunToken}}})
	send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: "tb1"}}})
	waitEvent(t, g, "phase:claim", "tb1", 5*time.Second)

	// 79<80%：不警；推到 80：警一次（边沿）。
	send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Usage{Usage: &pbv1.Usage{TokensUsed: 79}}})
	for _, e := range collectEvents(g, 200*time.Millisecond) {
		if e.Kind == "budget_warn" {
			t.Fatal("79<80% 误警")
		}
	}
	send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Usage{Usage: &pbv1.Usage{TokensUsed: 1}}})
	waitEvent(t, g, "budget_warn", "tb1", 3*time.Second)
	// 再到 100：硬停→杀席→回队不自动重派。
	send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Usage{Usage: &pbv1.Usage{TokensUsed: 20}}})
	waitEvent(t, g, "budget_stop", "tb1", 3*time.Second)
	stream.CloseSend() // 代驾流 EOF：双源互证第二源
	waitPhase(t, g, "tb1", db.PhaseAssigned, 5*time.Second)
	if p, _ := g.DB().PhaseOf(ctx, "tb1"); p != db.PhaseAssigned {
		t.Fatalf("硬停后相位=%s", p)
	}
	st, _ := g.DB().RunByToken(ctx, run.RunToken)
	if !st.Dead {
		t.Fatal("硬停未杀席")
	}

	// 票级硬顶：tb2 硬帽 150；两次用量过顶→先杀后冻。
	mkTicketAt(t, g, "tb2", db.PhaseAssigned)
	if err := g.DB().SetBudgetCaps(ctx, "tb2", 1000, 150); err != nil {
		t.Fatal(err)
	}
	run2 := spawnSeat(t, g, "tb2", "施工", "idle")
	stream2, err := c.Attach(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	send2 := func(m *pbv1.SeatMsg) {
		if err := stream2.Send(m); err != nil {
			t.Fatal(err)
		}
	}
	send2(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: run2.RunToken}}})
	send2(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: "tb2"}}})
	waitEvent(t, g, "phase:claim", "tb2", 5*time.Second)
	send2(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Usage{Usage: &pbv1.Usage{TokensUsed: 160}}})
	waitEvent(t, g, "budget_frozen", "tb2", 3*time.Second)
	stream2.CloseSend() // 双源互证第二源

	tk, _ := g.DB().GetTicket(ctx, "tb2")
	if tk == nil || !tk.BudgetFrozen {
		t.Fatal("budget_frozen 未置")
	}
	if _, ok := g.AlarmDeadline("tb2"); ok {
		t.Fatal("冻结未取消未决闹钟")
	}
	st2, _ := g.DB().RunByToken(ctx, run2.RunToken)
	deadline := time.Now().Add(3 * time.Second)
	for st2 != nil && !st2.Dead && time.Now().Before(deadline) {
		<-time.After(20 * time.Millisecond)
		st2, _ = g.DB().RunByToken(ctx, run2.RunToken)
	}
	if st2 == nil || !st2.Dead {
		t.Fatal("先杀后冻：席未被杀")
	}
	// 冻票相位回队后不自动重派、Dispatch 拒。
	if err := g.DispatchSeat(ctx, "tb2", "施工", dispatchSpec()); err == nil {
		t.Fatal("冻票派发应被拒")
	}
}

// TestBudget_AttemptReset 换壳重做的新尝试：尝试内用量从零起（上一尝试的
// used 不清零会让新尝试首帧即被 100% 硬停秒杀），80% 警按尝试各发一次；
// 票级 used_total 跨尝试单调不清零。（驳回1 R1 回归锚——删 ResetAttemptUsage
// 接线或删 warned 复位，本表必红。）
func TestBudget_AttemptReset(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		d := filepath.Dir(c.DBPath)
		*c = dispatchCfg(d)
	})
	ctx := context.Background()
	mkTicketAt(t, g, "tr", db.PhaseAssigned)
	if err := g.DB().SetBudgetCaps(ctx, "tr", 100, 1000); err != nil {
		t.Fatal(err)
	}
	// idle 席不自己连——测试代驾 Attach 推 Usage 帧。
	spec := SeatSpec{PromptFile: "/dev/null", WorkDir: "/tmp",
		Env: []string{"GO_HELPER_SEAT=1", "SEAT_MODE=idle", "META_ADOPT_GRACE=30",
			"PATH=" + os.Getenv("PATH")}}
	if err := g.DispatchSeat(ctx, "tr", "施工", spec); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "seat_up", "tr", 10*time.Second) // idle 席过秒退窗口即立住

	runs, _ := g.DB().RunsForTicket(ctx, "tr")
	if len(runs) != 1 {
		t.Fatalf("首尝试应有 1 run: %d", len(runs))
	}
	_, c := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))
	s1, err := c.Attach(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	send1 := func(m *pbv1.SeatMsg) {
		if err := s1.Send(m); err != nil {
			t.Fatal(err)
		}
	}
	send1(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: runs[0].RunToken}}})
	send1(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: "tr"}}})
	waitEvent(t, g, "phase:claim", "tr", 5*time.Second)

	// 尝试1：用量到 95%——80% 警一次（边沿）。
	send1(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Usage{Usage: &pbv1.Usage{TokensUsed: 95}}})
	waitEvent(t, g, "budget_warn", "tr", 3*time.Second)
	s1.CloseSend() // 代驾流 EOF：杀后双源互证第二源
	if err := syscall.Kill(-runs[0].PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}

	// ①级中途死→回队→failSet 排除 devin→claude 顶上（新尝试）。
	waitEvent(t, g, "seat_up", "tr", 15*time.Second)
	runs, _ = g.DB().RunsForTicket(ctx, "tr")
	if len(runs) != 2 || runs[1].Shell != "claude" {
		t.Fatalf("第二尝试应换 claude: runs=%d", len(runs))
	}

	// 新尝试 used 从零起：+10 不许碰旧账 95→105 秒杀（R1 病灶）。
	s2, err := c.Attach(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	send2 := func(m *pbv1.SeatMsg) {
		if err := s2.Send(m); err != nil {
			t.Fatal(err)
		}
	}
	send2(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: runs[1].RunToken}}})
	send2(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Usage{Usage: &pbv1.Usage{TokensUsed: 10}}})
	for _, e := range collectEvents(g, 400*time.Millisecond) {
		if e.Kind == "budget_stop" || e.Kind == "budget_frozen" {
			t.Fatalf("新尝试首帧被旧账秒杀: %v", e)
		}
	}
	if !processAlive(runs[1].PID) {
		t.Fatal("新尝试席位被误杀")
	}
	if b, _ := g.DB().GetBudget(ctx, "tr"); b.Used != 10 {
		t.Fatalf("尝试内 used 应为 10（清零后重计）: %d", b.Used)
	}

	// 80% 警按尝试各发一次：本尝试再到 80 必再警。
	send2(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Usage{Usage: &pbv1.Usage{TokensUsed: 70}}})
	waitEvent(t, g, "budget_warn", "tr", 3*time.Second)

	// 票级累计不清零：95+10+70=175。
	if b, _ := g.DB().GetBudget(ctx, "tr"); b.Used != 80 || b.UsedTotal != 175 {
		t.Fatalf("预算账不符: used=%d used_total=%d（预期 80/175）", b.Used, b.UsedTotal)
	}
	s2.CloseSend()
}

// TestAlarm_UsageResets 用量帧重置闹钟（锚5 增量：Progress 之外的活性帧）。
func TestAlarm_UsageResets(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) { c.ProgressTimeout = 300 * time.Millisecond })
	mkTicketAt(t, g, "ta", db.PhaseAssigned)
	run := spawnSeat(t, g, "ta", "施工", "idle")
	_, c := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))
	stream, _ := c.Attach(context.Background())
	stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: run.RunToken}}})
	stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: "ta"}}})
	waitEvent(t, g, "phase:claim", "ta", 5*time.Second)
	d1, _ := g.AlarmDeadline("ta")
	<-time.After(120 * time.Millisecond)
	stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Usage{Usage: &pbv1.Usage{TokensUsed: 5}}})
	deadline := time.Now().Add(3 * time.Second)
	var d2 time.Time
	for time.Now().Before(deadline) {
		d2, _ = g.AlarmDeadline("ta")
		if d2.After(d1) {
			return // 用量帧重置生效
		}
		<-time.After(10 * time.Millisecond)
	}
	t.Fatalf("用量帧未重置闹钟: %v → %v", d1, d2)
}
