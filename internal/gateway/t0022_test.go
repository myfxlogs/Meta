package gateway

// 票 0022 锚 A1–A4：审计重派壳偏置（D-46）——同壳尾部连败 ≥N 本轮跳过，
// 只跳过不标壳坏；剔空照旧 shellsExhausted。对抗锚：删 skipLosingAuditShells
// 接线/剔除行或翻 ExitCode!=0 判定 → A1 红。

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"meta/internal/db"
)

// auditBiasGateway 阈值 2 的调度测试网关＋单调注入钟：seat_run started_at
// 严格保序（连败计数赖新→旧扫序，同刻序不定）。
func auditBiasGateway(t *testing.T) *Gateway {
	t.Helper()
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.AuditShellSkipStreak = 2
	})
	var tick int64
	base := time.Now()
	g.DB().SetClock(func() time.Time {
		tick++
		return base.Add(time.Duration(tick))
	})
	return g
}

// seedSeatRun 造一条完结的 seat_run 行（连败账输入）：started_at 取注入钟，
// 调用序即新→旧序；dead=判死，code=退码（成功行：dead=1 code=0——正常退也判死）。
func seedSeatRun(t *testing.T, g *Gateway, ticketID, role, shell string, code int, dead bool) {
	t.Helper()
	ctx := context.Background()
	id := fmt.Sprintf("seed-%s-%d", ticketID, time.Now().UnixNano())
	r := &db.Run{
		RunID:      id,
		TicketID:   ticketID,
		Role:       role,
		Shell:      shell,
		RunToken:   id + "-tok",
		Provenance: "test",
	}
	if err := g.DB().CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := g.DB().MarkExited(ctx, id, code); err != nil {
		t.Fatal(err)
	}
	if dead {
		if err := g.DB().MarkDead(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
}

// auditDispatch 派审计席（prefer 可选）；spec 用 worker 席——连上即领票立住。
func auditDispatch(t *testing.T, g *Gateway, ticketID, prefer string) {
	t.Helper()
	spec := dispatchSpec(t)
	spec.PreferShell = prefer
	if err := g.DispatchSeat(context.Background(), ticketID, "审计", spec); err != nil {
		t.Fatal(err)
	}
}

// lastRun 票上最新一枚 run 的登记表标签。
func lastRun(t *testing.T, g *Gateway, ticketID string) *db.Run {
	t.Helper()
	runs, err := g.DB().RunsForTicket(context.Background(), ticketID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("runs 取不到/空: %v %d", err, len(runs))
	}
	return runs[len(runs)-1]
}

// eventKindCount ticket_event 里某 kind 的行数（偏置落账断言）。
func eventKindCount(t *testing.T, g *Gateway, ticketID, kind string) int {
	t.Helper()
	evs, err := g.DB().Events(context.Background(), ticketID)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range evs {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// TestAuditShellBias_Skip A1：devin 审计连败 2（最新两枚）→prefer 提首仍被剔，
// claude 顶上立住；audit_shell_skip 事件 emit＋ticket_event 落账，note 含壳名。
func TestAuditShellBias_Skip(t *testing.T) {
	g := auditBiasGateway(t)
	mkTicketAt(t, g, "t-a1", db.PhaseReview)
	seedSeatRun(t, g, "t-a1", "审计", "devin/swe-2-max", 1, true)
	seedSeatRun(t, g, "t-a1", "审计", "devin/swe-2-max", 1, true)

	auditDispatch(t, g, "t-a1", "devin")
	e := waitEvent(t, g, "audit_shell_skip", "t-a1", 8*time.Second)
	if !strings.Contains(e.Note, "devin") {
		t.Fatalf("skip note 应含壳名: %q", e.Note)
	}
	waitEvent(t, g, "seat_up", "t-a1", 10*time.Second)
	if got := lastRun(t, g, "t-a1").Shell; got != "claude" {
		t.Fatalf("devin 应被跳过、claude 顶上: 末 run=%s", got)
	}
	if eventKindCount(t, g, "t-a1", "audit_shell_skip") != 1 {
		t.Fatal("ticket_event 应落 1 行 audit_shell_skip")
	}
}

// TestAuditShellBias_Threshold A2 阈值边界：同壳仅 1 连败→不跳照试；
// 连败 2 中间夹 ExitCode==0 成功行→断计不跳。
func TestAuditShellBias_Threshold(t *testing.T) {
	g := auditBiasGateway(t)

	// 仅 1 败：devin 不跳。
	mkTicketAt(t, g, "t-a2a", db.PhaseReview)
	seedSeatRun(t, g, "t-a2a", "审计", "devin/swe-2-max", 1, true)
	auditDispatch(t, g, "t-a2a", "devin")
	waitEvent(t, g, "seat_up", "t-a2a", 10*time.Second)
	if got := lastRun(t, g, "t-a2a").Shell; got != "devin/swe-2-max" {
		t.Fatalf("1 败不该跳 devin: 末 run=%s", got)
	}

	// 中间夹成功行：尾部连败断计为 1，devin 不跳。
	mkTicketAt(t, g, "t-a2b", db.PhaseReview)
	seedSeatRun(t, g, "t-a2b", "审计", "devin/swe-2-max", 1, true) // 旧败
	seedSeatRun(t, g, "t-a2b", "审计", "devin/swe-2-max", 0, true) // 夹成功行
	seedSeatRun(t, g, "t-a2b", "审计", "devin/swe-2-max", 1, true) // 新败
	auditDispatch(t, g, "t-a2b", "devin")
	waitEvent(t, g, "seat_up", "t-a2b", 10*time.Second)
	if got := lastRun(t, g, "t-a2b").Shell; got != "devin/swe-2-max" {
		t.Fatalf("断计后不该跳 devin: 末 run=%s", got)
	}
	if eventKindCount(t, g, "t-a2b", "audit_shell_skip") != 0 {
		t.Fatal("断计票不该出 audit_shell_skip")
	}
}

// TestAuditShellBias_RoleIsolation A3：连败行 role=施工 →审计派发不计；
// 审计连败行在 →施工派发不生效（D-46 钦定面＝审计重派）。
func TestAuditShellBias_RoleIsolation(t *testing.T) {
	g := auditBiasGateway(t)
	ctx := context.Background()

	// 施工连败行不进审计偏置账。
	mkTicketAt(t, g, "t-a3a", db.PhaseReview)
	seedSeatRun(t, g, "t-a3a", "施工", "devin/swe-2-high", 1, true)
	seedSeatRun(t, g, "t-a3a", "施工", "devin/swe-2-high", 1, true)
	auditDispatch(t, g, "t-a3a", "devin")
	waitEvent(t, g, "seat_up", "t-a3a", 10*time.Second)
	if got := lastRun(t, g, "t-a3a").Shell; got != "devin/swe-2-max" {
		t.Fatalf("施工连败不该触发审计偏置: 末 run=%s", got)
	}

	// 审计连败行在，施工派发照样原序试 devin（偏置只挂审计径）。
	if err := g.DB().CreateTicket(ctx, "t-a3b", "隔离票b", nil); err != nil {
		t.Fatal(err)
	}
	seedSeatRun(t, g, "t-a3b", "审计", "devin/swe-2-max", 1, true)
	seedSeatRun(t, g, "t-a3b", "审计", "devin/swe-2-max", 1, true)
	if err := g.DispatchSeat(ctx, "t-a3b", "施工", dispatchSpec(t)); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, g, "seat_up", "t-a3b", 10*time.Second)
	if got := lastRun(t, g, "t-a3b").Shell; got != "devin/swe-2-high" {
		t.Fatalf("施工派发不该被审计偏置影响: 末 run=%s", got)
	}
	if eventKindCount(t, g, "t-a3b", "audit_shell_skip") != 0 {
		t.Fatal("施工派发不该出 audit_shell_skip")
	}
}

// TestAuditShellBias_Exhausted A4：全部审计候选壳连败达标→剔空→
// shellsExhausted 径（事件在），零新 run。
func TestAuditShellBias_Exhausted(t *testing.T) {
	g := auditBiasGateway(t)
	mkTicketAt(t, g, "t-a4", db.PhaseReview)
	for i := 0; i < 2; i++ { // 两壳各 2 连败（claude 块在上/devin 块在下——交叉断计语义同达）
		seedSeatRun(t, g, "t-a4", "审计", "claude", 1, true)
	}
	for i := 0; i < 2; i++ {
		seedSeatRun(t, g, "t-a4", "审计", "devin/swe-2-max", 1, true)
	}
	auditDispatch(t, g, "t-a4", "")
	// T26 病5：首尽 degraded_retry 清 failSet 重试一轮——剔除账在 seat_run
	// 不在 failSet，第二轮仍剔空→shells_degraded；两壳×两轮＝4 行 skip。
	waitEvent(t, g, "shells_degraded", "t-a4", 8*time.Second)
	if n := eventKindCount(t, g, "t-a4", "audit_shell_skip"); n != 4 {
		t.Fatalf("两壳两轮各应落 2 行 audit_shell_skip: %d", n)
	}
	runs, _ := g.DB().RunsForTicket(context.Background(), "t-a4")
	if len(runs) != 4 {
		t.Fatalf("剔空后不该有新 spawn: runs=%d", len(runs))
	}
}
