package gateway

// 票0021 验收锚：A4 设计席死归队补发 CTO-A＋A5 AdminGateway.Reconcile 手动对账径。

import (
	"context"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// TestDesignDeadRespawn 锚 A4（T35 改义）：PhaseDesigning 票挂死席——相位
// 稳守 设计中＋seat_dead 账，调和器按失败表重派 CTO-A（非回队）。
func TestDesignDeadRespawn(t *testing.T) {
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "worker")
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "hold")
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.InjectRoot = injectFixture(t)
	})
	mkTicketAt(t, g, "td", db.PhaseDesigning)
	if err := g.DB().UpdateSpec(context.Background(), "td", testSpec(t)); err != nil {
		t.Fatal(err)
	}
	run := spawnSeat(t, g, "td", "CTO", "hold", "META_ADOPT_GRACE=30")
	waitSeatUp(t, run) // 等流建立双源齐备再杀——未 attach 死无 EOF 互证
	if err := syscall.Kill(-run.PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, g, "td", db.PhaseDesigning, 10*time.Second)
	waitFor2(t, 10*time.Second, func() bool {
		return hasEvent(t, g, "td", "seat_dead") != nil
	}, g)
	// 调和器重派：新 CTO run 出现（死席标死后 desired=CTO 起席）。
	waitFor2(t, 15*time.Second, func() bool {
		r, _ := g.DB().ActiveRunForTicketRole(context.Background(), "td", "CTO")
		return r != nil && r.RunID != run.RunID
	}, g)
}

// TestReconcileRPC 锚 A5（T35 改义）：AdminGateway.Reconcile 手动对账径——
// 对已立票直派 CTO-A（幂等调和谐唯一 spawn 点）；二次调用幂等：活席票被
// 收养不双派不回队（相位不动、run 不死）。
func TestReconcileRPC(t *testing.T) {
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "worker")
	t.Setenv("GO_HELPER_SEAT", "1")
	t.Setenv("SEAT_MODE", "hold")
	g, dir := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.InjectRoot = injectFixture(t)
	})
	ctx := context.Background()
	if err := g.DB().CreateTicket(ctx, "tr1", "已立票", nil); err != nil {
		t.Fatal(err)
	}
	if err := g.DB().UpdateSpec(ctx, "tr1", testSpec(t)); err != nil {
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
	// T35：已立票＝desired CTO-A 起席——run 账即调和产出（死活属壳环）。
	waitFor2(t, 15*time.Second, func() bool {
		rs, _ := g.DB().RunsForTicket(ctx, "tr1")
		for _, r := range rs {
			if r.Role == "CTO" {
				return true
			}
		}
		return false
	}, g)
	// T35：desired==actual 对齐——票相位不被调和器乱动。
	if p, _ := g.DB().PhaseOf(ctx, "tr2"); p != db.PhaseWorking {
		t.Fatalf("调和器动了活席票相位: %s", p)
	}
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
