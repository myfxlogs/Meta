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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// adjunctSpec adjunct 派发 spec（必给 prompt_file；SEAT_MODE 经 Env 注入）。
func adjunctSpec(t *testing.T, mode string, extra ...string) SeatSpec {
	t.Helper()
	pf := filepath.Join(t.TempDir(), "prompt.md")
	if err := os.WriteFile(pf, []byte("# fixture 提示\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := []string{"GO_HELPER_SEAT=1", "SEAT_MODE=" + mode,
		"META_ADOPT_GRACE=30", "PATH=" + os.Getenv("PATH")}
	return SeatSpec{PromptFile: pf, WorkDir: "/tmp",
		Env: append(env, extra...), Adjunct: true}
}

func waitRuns(t *testing.T, g *Gateway, tid string, n int, timeout time.Duration) []*db.Run {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rs, _ := g.DB().RunsForTicket(ctx, tid)
		if len(rs) >= n {
			return rs
		}
		<-time.After(30 * time.Millisecond)
	}
	rs, _ := g.DB().RunsForTicket(ctx, tid)
	t.Fatalf("票 %s runs=%d 预期≥%d", tid, len(rs), n)
	return nil
}

func hasEvent(t *testing.T, g *Gateway, tid, kind string) *db.Event {
	t.Helper()
	evs, _ := g.DB().Events(context.Background(), tid)
	for _, e := range evs {
		if e.Kind == kind {
			return e
		}
	}
	return nil
}

// TestAdjunct_KillRequeue 锚 5：SEAT_MODE=rule 发 kill_requeue→施工 run dead、
// 票回 已派、failSet 含其壳键、相位不被 adjunct claim/deliver 污染。
func TestAdjunct_KillRequeue(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	ctx := context.Background()
	mkTicketAt(t, g, "ta1", db.PhaseAssigned)
	worker := spawnSeat(t, g, "ta1", "施工", "worker")
	waitPhase(t, g, "ta1", db.PhaseWorking, 5*time.Second)

	// adjunct 闸：无 prompt_file 拒；同角色活 adjunct 去重。
	if err := g.DispatchSeat(ctx, "ta1", "CTO", SeatSpec{Adjunct: true, WorkDir: "/tmp"}); err == nil {
		t.Fatal("adjunct 无 prompt_file 应拒")
	}
	spec := adjunctSpec(t, "rule", "SEAT_RULE=kill_requeue:空转确认")
	if err := g.DispatchSeat(ctx, "ta1", "CTO", spec); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "adjunct_claim", "ta1", 10*time.Second)
	if err := g.DispatchSeat(ctx, "ta1", "CTO", spec); err == nil {
		t.Fatal("活 adjunct 在应去重拒")
	}

	// 裁决落地：施工 run dead＋票回已派＋failSet 含壳键＋ruling 行。
	waitPhase(t, g, "ta1", db.PhaseAssigned, 10*time.Second)
	st, _ := g.DB().RunByToken(ctx, worker.RunToken)
	if st == nil || !st.Dead {
		t.Fatal("kill_requeue 未杀施工 run")
	}
	wantKey := db.Shell{Name: "testshell", Role: "施工"}.ShellKey()
	g.mu.Lock()
	inSet := g.failSet["ta1"][wantKey]
	g.mu.Unlock()
	if !inSet {
		t.Fatalf("failSet 缺壳键 %s", wantKey)
	}
	if hasEvent(t, g, "ta1", "ruling") == nil {
		t.Fatal("ruling 行未落账")
	}
}

// TestAdjunct_Defer 锚 5：defer→闹钟后移≈ProgressTimeout＋defer_count=1；
// 第三次 defer→defer_limit 行且 count 不再增。
func TestAdjunct_Defer(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.ProgressTimeout = 30 * time.Minute
	})
	ctx := context.Background()
	mkTicketAt(t, g, "ta2", db.PhaseAssigned)
	spawnSeat(t, g, "ta2", "施工", "worker")
	waitPhase(t, g, "ta2", db.PhaseWorking, 5*time.Second)

	if err := g.DispatchSeat(ctx, "ta2", "CTO",
		adjunctSpec(t, "rule", "SEAT_RULE=defer:真在干活", "SEAT_RULE_COUNT=3")); err != nil {
		t.Fatal(err)
	}
	// 3 帧同流发：1→count=1 闹钟后移；2→count=2；3→帽尽 defer_limit 不增。
	waitFor2(t, 10*time.Second, func() bool {
		tk, _ := g.DB().GetTicket(ctx, "ta2")
		return tk.DeferCount >= 2
	})
	waitFor2(t, 5*time.Second, func() bool {
		return hasEvent(t, g, "ta2", "defer_limit") != nil
	})
	tk, _ := g.DB().GetTicket(ctx, "ta2")
	if tk.DeferCount != 2 {
		t.Fatalf("defer_count=%d 预期 2（帽尽不再增）", tk.DeferCount)
	}
	dl, ok := g.AlarmDeadline("ta2")
	if !ok || time.Until(dl) < 25*time.Minute || time.Until(dl) > 31*time.Minute {
		t.Fatalf("defer 未重挂全额闹钟: %v", dl)
	}
	if p, _ := g.DB().PhaseOf(ctx, "ta2"); p != db.PhaseWorking {
		t.Fatalf("adjunct 裁决动了相位: %s", p)
	}
}

// TestAdjunct_DeliverNoPhase 锚 5：adjunct deliver→仅落 adjunct_deliver 行＋
// Ack 照回，票相位不动（§1.3 码位表 deliver 行——裁决席不是相位步行席）。
func TestAdjunct_DeliverNoPhase(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	ctx := context.Background()
	mkTicketAt(t, g, "ta4", db.PhaseAssigned)
	spawnSeat(t, g, "ta4", "施工", "worker")
	waitPhase(t, g, "ta4", db.PhaseWorking, 5*time.Second)

	if err := g.DispatchSeat(ctx, "ta4", "CTO",
		adjunctSpec(t, "rule", "SEAT_RULE=defer:真在干活")); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "adjunct_claim", "ta4", 10*time.Second)
	var token string
	for _, r := range waitRuns(t, g, "ta4", 2, 10*time.Second) {
		if r.Role == "CTO" {
			token = r.RunToken
		}
	}
	if token == "" {
		t.Fatal("CTO adjunct run 未立")
	}
	// 第二流同 token 发 Deliver（裁决席产物出口）：adjunct 支只记行不转相位。
	_, c := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))
	s, _ := c.Attach(ctx)
	s.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: token}}})
	s.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Deliver{Deliver: &pbv1.Deliver{
		Ref: "r1", Note: "裁决产物"}}})
	m, err := s.Recv()
	if err != nil || m.GetAck() == nil || !m.GetAck().GetOk() {
		t.Fatalf("adjunct deliver 无 Ack: %v %+v", err, m)
	}
	if hasEvent(t, g, "ta4", "adjunct_deliver") == nil {
		t.Fatal("adjunct_deliver 行未落账")
	}
	if p, _ := g.DB().PhaseOf(ctx, "ta4"); p != db.PhaseWorking {
		t.Fatalf("adjunct deliver 动了相位: %s", p)
	}
	s.CloseSend()
}

// TestRule_NonAdjunctRejected 锚 5：非 adjunct 席发 rule→拒（断流）。
func TestRule_NonAdjunctRejected(t *testing.T) {	g, dir := newTestGateway(t, nil)
	mkTicketAt(t, g, "ta3", db.PhaseAssigned)
	run := spawnSeat(t, g, "ta3", "施工", "idle")
	_, c := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))
	stream, err := c.Attach(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: run.RunToken}}})
	stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: "ta3"}}})
	waitPhase(t, g, "ta3", db.PhaseWorking, 3*time.Second)
	if err := stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Rule{
		Rule: &pbv1.Rule{Action: "defer", Note: "x"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("非 adjunct rule 未被拒: %v", err)
	}
}

// TestThreeRejects 锚 6：连续 3 次 EdgeReject（经 transition()）→reject_limit
// emit＋事件行；1/2 次无。dispatch 边→phase:dispatch 门铃。
func TestThreeRejects(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "trj", db.PhaseWorking)
	rejectOnce := func() {
		for _, s := range []struct {
			to   db.Phase
			kind db.EdgeKind
		}{
			{db.PhaseReview, db.EdgeDeliver},
			{db.PhaseAuditing, db.EdgeAuditStart},
			{db.PhaseAssigned, db.EdgeReject},
		} {
			if err := g.transition(ctx, "trj", s.to, s.kind, "test", ""); err != nil {
				t.Fatalf("迁移→%s: %v", s.to, err)
			}
		}
		// 回 施工中 再驳
		if err := g.transition(ctx, "trj", db.PhaseWorking, db.EdgeClaim, "test", ""); err != nil {
			t.Fatal(err)
		}
	}
	rejectOnce()
	rejectOnce()
	for _, e := range collectEvents(g, 200*time.Millisecond) {
		if e.Kind == "reject_limit" {
			t.Fatal("第 2 次驳回即 emit reject_limit")
		}
	}
	rejectOnce()
	waitEvent(t, g, "reject_limit", "trj", 3*time.Second)
	if hasEvent(t, g, "trj", "reject_limit") == nil {
		t.Fatal("reject_limit 事件行未落账")
	}
	tk, _ := g.DB().GetTicket(ctx, "trj")
	if tk.RejectCount != 3 {
		t.Fatalf("reject_count=%d 预期 3", tk.RejectCount)
	}

	// dispatch 边门铃。
	if err := g.DB().CreateTicket(ctx, "tdp", "门铃票", nil); err != nil {
		t.Fatal(err)
	}
	if err := g.transition(ctx, "tdp", db.PhaseDesigning, db.EdgeDesignStart, "test", ""); err != nil {
		t.Fatal(err)
	}
	if err := g.transition(ctx, "tdp", db.PhaseAssigned, db.EdgeDispatch, "test", ""); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "phase:dispatch", "tdp", 3*time.Second)
}

// TestConfigSeeds 锚 7：fixture 配置→shell 表按行落种＋EnsureBudgetDefaults
// 写角色帽＋opencode 行 attempt_cap 覆盖生效。
func TestConfigSeeds(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.DefaultAttemptCap = 20_000_000
		c.TicketHardCap = 120_000_000
		c.RoleCaps = map[string]int64{"施工": 40_000_000}
	})
	ctx := context.Background()
	if err := g.DB().CreateTicket(ctx, "tc1", "配置票", nil); err != nil {
		t.Fatal(err)
	}
	if err := g.DispatchSeat(ctx, "tc1", "施工", dispatchSpec(t)); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "seat_up", "tc1", 10*time.Second)
	// 落种：登记行按 fixture 出现；角色帽落种。
	rows, err := g.DB().ListShells(ctx, "施工")
	if err != nil || len(rows) != 3 {
		t.Fatalf("登记行=%v %v（预期 3 行）", rows, err)
	}
	b, _ := g.DB().GetBudget(ctx, "tc1")
	if b.Cap != 40_000_000 || b.HardCap != 120_000_000 {
		t.Fatalf("EnsureBudgetDefaults 帽不符: %+v", b)
	}

	// opencode 行 attempt_cap 覆盖：prefer opencode→本尝试帽改 8M。
	if err := g.DB().CreateTicket(ctx, "tc2", "低帽票", nil); err != nil {
		t.Fatal(err)
	}
	spec := dispatchSpec(t)
	spec.PreferShell = "opencode"
	if err := g.DispatchSeat(ctx, "tc2", "施工", spec); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "seat_up", "tc2", 10*time.Second)
	b2, _ := g.DB().GetBudget(ctx, "tc2")
	if b2.Cap != 8_000_000 {
		t.Fatalf("opencode 行 attempt_cap 未覆盖: cap=%d", b2.Cap)
	}
}

// TestAuditGate 锚 11：待审票派审计→claim→audit_start＋相位审计中；他相位
// 拒；同角色活席再派拒；usage 双计。
func TestAuditGate(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	ctx := context.Background()
	mkTicketAt(t, g, "tg1", db.PhaseReview)

	spec := dispatchSpec(t)
	spec.Env = []string{"GO_HELPER_SEAT=1", "SEAT_MODE=idle",
		"META_ADOPT_GRACE=30", "PATH=" + os.Getenv("PATH")} // 测试代驾
	if err := g.DispatchSeat(ctx, "tg1", "审计", spec); err != nil {
		t.Fatal(err)
	}
	// 相位仍待审（派发不步行）；run 已生。
	waitRuns(t, g, "tg1", 1, 10*time.Second)
	if p, _ := g.DB().PhaseOf(ctx, "tg1"); p != db.PhaseReview {
		t.Fatalf("审计派发动了相位: %s", p)
	}
	// 同角色活席再派→拒。
	if err := g.DispatchSeat(ctx, "tg1", "审计", spec); err == nil {
		t.Fatal("活审计席在再派应拒")
	}
	// 他相位派审计→拒。
	for i, ph := range []db.Phase{db.PhaseAssigned, db.PhaseWorking} {
		tid := fmt.Sprintf("tgx%d", i)
		mkTicketAt(t, g, tid, ph)
		if err := g.DispatchSeat(ctx, tid, "审计", spec); err == nil {
			t.Fatalf("相位 %s 派审计应拒（case %d）", ph, i)
		}
	}

	run := waitRuns(t, g, "tg1", 1, 5*time.Second)[0]
	_, c := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))
	stream, _ := c.Attach(context.Background())
	stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: run.RunToken}}})
	stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: "tg1"}}})
	waitEvent(t, g, "phase:audit_start", "tg1", 5*time.Second)
	waitPhase(t, g, "tg1", db.PhaseAuditing, 3*time.Second)
	if hasEvent(t, g, "tg1", "audit_start") == nil {
		t.Fatal("audit_start 事件行未落账")
	}
	// usage 双计（相位步行席是真尝试）。
	stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Usage{Usage: &pbv1.Usage{TokensUsed: 10}}})
	waitFor2(t, 3*time.Second, func() bool {
		b, _ := g.DB().GetBudget(ctx, "tg1")
		return b.Used == 10 && b.UsedTotal == 10
	})
	stream.CloseSend()
}

// TestVerdictFlow 锚 12：判决回流——reject→已派＋reject_count=1＋施工重派
// 新 run；pass→待部署；非法 verdict→InvalidArgument 不落相位；杀审计席→
// 待审（audit_dead，不计驳回）＋重派审计。
func TestVerdictFlow(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	ctx := context.Background()

	// —— reject 径：施工→deliver→待审→审计 verdict=reject →已派+重派 ——
	if err := g.DB().CreateTicket(ctx, "tv1", "判决票", nil); err != nil {
		t.Fatal(err)
	}
	specW := dispatchSpec(t)
	if err := g.DispatchSeat(ctx, "tv1", "施工", specW); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, g, "tv1", db.PhaseWorking, 10*time.Second)
	wrun := waitRuns(t, g, "tv1", 1, 5*time.Second)[0]
	_, c := dialSeatGateway(t, filepath.Join(dir, "run", "meta.sock"))
	s1, _ := c.Attach(context.Background())
	s1.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: wrun.RunToken}}})
	s1.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{Claim: &pbv1.Claim{TicketId: "tv1"}}})
	s1.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Deliver{Deliver: &pbv1.Deliver{Ref: "c1"}}})
	waitPhase(t, g, "tv1", db.PhaseReview, 5*time.Second)
	s1.CloseSend()
	syscall.Kill(-wrun.PID, syscall.SIGKILL)

	specA := dispatchSpec(t)
	specA.Env = []string{"GO_HELPER_SEAT=1", "SEAT_MODE=verdict", "SEAT_VERDICT=reject",
		"SEAT_REF=docs/x/判.md", "META_ADOPT_GRACE=30", "PATH=" + os.Getenv("PATH")}
	if err := g.DispatchSeat(ctx, "tv1", "审计", specA); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, g, "tv1", db.PhaseAssigned, 15*time.Second)
	tk, _ := g.DB().GetTicket(ctx, "tv1")
	if tk.RejectCount != 1 {
		t.Fatalf("reject_count=%d 预期 1", tk.RejectCount)
	}
	if hasEvent(t, g, "tv1", "reject") == nil {
		t.Fatal("reject 事件行未落账")
	}
	// 重派发生：新 run role=施工（径①内 enqueueSpawn）。
	waitFor2(t, 15*time.Second, func() bool {
		rs, _ := g.DB().RunsForTicket(ctx, "tv1")
		for _, r := range rs {
			if r.Role == "施工" && r.RunID != wrun.RunID && !r.Dead {
				return true
			}
		}
		return false
	})

	// —— pass 径：另一票 →待部署 ——
	mkTicketAt(t, g, "tv2", db.PhaseReview)
	specA2 := dispatchSpec(t)
	specA2.Env = []string{"GO_HELPER_SEAT=1", "SEAT_MODE=verdict", "SEAT_VERDICT=pass",
		"SEAT_REF=docs/x/判2.md", "META_ADOPT_GRACE=30", "PATH=" + os.Getenv("PATH")}
	if err := g.DispatchSeat(ctx, "tv2", "审计", specA2); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, g, "tv2", db.PhaseToDeploy, 15*time.Second)
	if hasEvent(t, g, "tv2", "audit_pass") == nil {
		t.Fatal("audit_pass 事件行未落账")
	}

	// —— 非法 verdict＋杀审计席 ——
	mkTicketAt(t, g, "tv3", db.PhaseReview)
	specA3 := dispatchSpec(t)
	specA3.Env = []string{"GO_HELPER_SEAT=1", "SEAT_MODE=worker",
		"META_ADOPT_GRACE=30", "PATH=" + os.Getenv("PATH")}
	if err := g.DispatchSeat(ctx, "tv3", "审计", specA3); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, g, "tv3", db.PhaseAuditing, 15*time.Second)
	arun := waitRuns(t, g, "tv3", 1, 5*time.Second)[0]
	s3, _ := c.Attach(context.Background())
	s3.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{Auth: &pbv1.RunAuth{RunToken: arun.RunToken}}})
	if err := s3.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Deliver{
		Deliver: &pbv1.Deliver{Verdict: "bogus"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s3.Recv(); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("非法 verdict 未拒: %v", err)
	}
	if p, _ := g.DB().PhaseOf(ctx, "tv3"); p != db.PhaseAuditing {
		t.Fatalf("非法 verdict 动了相位: %s", p)
	}
	// 杀审计席→待审（audit_dead，reject_count 不变）＋重派审计 run。
	syscall.Kill(-arun.PID, syscall.SIGKILL)
	waitPhase(t, g, "tv3", db.PhaseReview, 10*time.Second)
	if hasEvent(t, g, "tv3", "audit_dead") == nil {
		t.Fatal("audit_dead 事件行未落账")
	}
	tk3, _ := g.DB().GetTicket(ctx, "tv3")
	if tk3.RejectCount != 0 {
		t.Fatalf("审计席死计了驳回: %d", tk3.RejectCount)
	}
	waitFor2(t, 15*time.Second, func() bool {
		rs, _ := g.DB().RunsForTicket(ctx, "tv3")
		for _, r := range rs {
			if r.Role == "审计" && r.RunID != arun.RunID && !r.Dead {
				return true
			}
		}
		return false
	})
}

// TestRetroVerdict 锚 13：待审票回翻——audit_start＋audit_pass 两行
// （actor=admin、note 含回翻与 ref）＋相位待部署；非待审/ref 空/二次→拒；
// reject→已派＋reject_count=1。
func TestRetroVerdict(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "tr1", db.PhaseReview)

	if ack, _ := g.RetroVerdict(ctx, &pbv1.RetroReq{TicketId: "tr1", Verdict: "pass", Ref: ""}); ack.Ok {
		t.Fatal("ref 空应拒")
	}
	ack, err := g.RetroVerdict(ctx, &pbv1.RetroReq{
		TicketId: "tr1", Verdict: "pass", Ref: "docs/实现/票-0001-判决/"})
	if err != nil || !ack.Ok {
		t.Fatalf("回翻失败: %v %+v", err, ack)
	}
	if p, _ := g.DB().PhaseOf(ctx, "tr1"); p != db.PhaseToDeploy {
		t.Fatalf("相位=%s 预期 待部署", p)
	}
	evs, _ := g.DB().Events(ctx, "tr1")
	var as, ap *db.Event
	for _, e := range evs {
		if e.Kind == "audit_start" {
			as = e
		}
		if e.Kind == "audit_pass" {
			ap = e
		}
	}
	if as == nil || ap == nil {
		t.Fatal("audit_start/audit_pass 行不齐")
	}
	if as.Actor != "admin" || !strings.Contains(as.Note, "回翻") ||
		!strings.Contains(as.Note, "docs/实现/票-0001-判决/") {
		t.Fatalf("回翻留痕不符: %+v", as)
	}
	// 幂等：二次调用相位已走→拒。
	if ack, _ := g.RetroVerdict(ctx, &pbv1.RetroReq{
		TicketId: "tr1", Verdict: "pass", Ref: "x"}); ack.Ok {
		t.Fatal("二次回翻应拒（幂等）")
	}
	// 非待审→拒。
	mkTicketAt(t, g, "tr2", db.PhaseAssigned)
	if ack, _ := g.RetroVerdict(ctx, &pbv1.RetroReq{
		TicketId: "tr2", Verdict: "pass", Ref: "x"}); ack.Ok {
		t.Fatal("非待审回翻应拒")
	}
	// reject 径：已派＋reject_count=1。
	mkTicketAt(t, g, "tr3", db.PhaseReview)
	ack, _ = g.RetroVerdict(ctx, &pbv1.RetroReq{
		TicketId: "tr3", Verdict: "reject", Ref: "docs/x/驳回.md"})
	if !ack.Ok {
		t.Fatalf("reject 回翻失败: %+v", ack)
	}
	if p, _ := g.DB().PhaseOf(ctx, "tr3"); p != db.PhaseAssigned {
		t.Fatalf("相位=%s 预期 已派", p)
	}
	tk, _ := g.DB().GetTicket(ctx, "tr3")
	if tk.RejectCount != 1 {
		t.Fatalf("reject_count=%d 预期 1", tk.RejectCount)
	}
}

// TestInject_PerRole 锚 8：注入落盘 <tid>-<role>.md；同票另一角色两文件并存。
func TestInject_PerRole(t *testing.T) {
	root := injectFixture(t)
	if err := os.WriteFile(filepath.Join(root, "roles", "审计.md"),
		[]byte("# 角色卡：审计\n\nfixture 审计关键行。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, dir := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.InjectRoot = root
	})
	ctx := context.Background()
	if err := g.DB().CreateTicket(ctx, "tdr", "双角色票", testSpec(t)); err != nil {
		t.Fatal(err)
	}
	spec := dispatchSpec(t)
	spec.PromptFile = "" // 注入径：无 --prompt-file 才走组装落盘
	if err := g.DispatchSeat(ctx, "tdr", "施工", spec); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, g, "tdr", db.PhaseWorking, 10*time.Second)
	waitFile(t, filepath.Join(dir, "run", "inject", "tdr-施工.md"), 3*time.Second)
	// 另一角色注入（直调组装径——审计派发闸限 待审，注入文件径独立）。
	tk, _ := g.DB().GetTicket(ctx, "tdr")
	p2, err := g.injectPrompt(ctx, "tdr", "审计", tk)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p2) != "tdr-审计.md" {
		t.Fatalf("落盘名未带角色: %s", p2)
	}
	for _, f := range []string{"tdr-施工.md", "tdr-审计.md"} {
		if _, err := os.Stat(filepath.Join(dir, "run", "inject", f)); err != nil {
			t.Fatalf("双文件未并存: %s", f)
		}
	}
}

func waitFor2(t *testing.T, d time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		<-time.After(15 * time.Millisecond)
	}
	t.Fatal("有界等待超时")
}
