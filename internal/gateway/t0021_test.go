package gateway

// 票0021 验收锚：A4 设计席死归队补发 CTO-A＋A5 AdminGateway.Reconcile 手动对账径。

import (
	"context"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// TestDesignDeadRespawn 锚 A4：PhaseDesigning 票挂死席走 onRunDead→
// design_dead 边回 已立＋ticket_event 补发 spawn_needed note 含「CTO-A」。
// 对抗锚③：删 onRunDead PhaseDesigning 支→本表必红（票卡 设计中）。
func TestDesignDeadRespawn(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	mkTicketAt(t, g, "td", db.PhaseDesigning)
	run := spawnSeat(t, g, "td", "CTO", "hold", "META_ADOPT_GRACE=30")
	if err := syscall.Kill(-run.PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, g, "td", db.PhaseCreated, 10*time.Second)
	if hasEvent(t, g, "td", string(db.EdgeDesignDead)) == nil {
		t.Fatal("design_dead 事件行未落账")
	}
	ev := hasEvent(t, g, "td", "spawn_needed")
	if ev == nil || !strings.Contains(ev.Note, "CTO-A") {
		t.Fatalf("spawn_needed 补发行缺/非 CTO-A 分线: %+v", ev)
	}
}

// TestReconcileRPC 锚 A5：AdminGateway.Reconcile 手动对账径——对已立票
// 补发 spawn_needed（note 含 CTO-A）；二次调用幂等：活席票被收养不双派
// 不回队（相位不动、run 不死）。
func TestReconcileRPC(t *testing.T) {
	g, dir := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	ctx := context.Background()
	if err := g.DB().CreateTicket(ctx, "tr1", "已立票", nil); err != nil {
		t.Fatal(err)
	}
	// 活席票：施工中＋hold 席（收养口在听——对账应收养非回队）。
	mkTicketAt(t, g, "tr2", db.PhaseWorking)
	run := spawnSeat(t, g, "tr2", "施工", "hold", "META_ADOPT_GRACE=30")
	waitSeatUp(t, run)

	conn, err := grpc.NewClient("unix://"+filepath.Join(dir, "run", "meta.sock"),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := pbv1.NewAdminGatewayClient(conn)
	ack, err := c.Reconcile(ctx, &pbv1.Empty{})
	if err != nil || !ack.Ok {
		t.Fatalf("Reconcile RPC: %v %v", ack, err)
	}
	ev := hasEvent(t, g, "tr1", "spawn_needed")
	if ev == nil || !strings.Contains(ev.Note, "CTO-A") {
		t.Fatalf("已立票补发不符（应含 CTO-A）: %+v", ev)
	}
	// 活席被收养落痕。
	waitFor2(t, 5*time.Second, func() bool {
		st, _ := g.DB().RunByToken(ctx, run.RunToken)
		return st != nil && !st.AdoptedAt.IsZero()
	})
	// 二次调用幂等：Ack ok、相位不动、run 不死。
	ack2, err := c.Reconcile(ctx, &pbv1.Empty{})
	if err != nil || !ack2.Ok {
		t.Fatalf("二次 Reconcile: %v %v", ack2, err)
	}
	if p, _ := g.DB().PhaseOf(ctx, "tr2"); p != db.PhaseWorking {
		t.Fatalf("活席票相位被对账挪动: %s", p)
	}
	if st, _ := g.DB().RunByToken(ctx, run.RunToken); st == nil || st.Dead {
		t.Fatal("活席被二次对账误杀")
	}
}
