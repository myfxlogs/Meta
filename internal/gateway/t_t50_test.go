package gateway

// 票 T50 锚测：部署重试径三缺——CliDrive 部署支统一/429中文签名/bloc­ked 停滞闸。

import (
	"context"
	"testing"
	"time"

	"meta/internal/db"
)

// 锚①：部署中票 deriveSpec 出 CliDrive（blocked→resume 同径）。
func TestT50_DeployingUsesCliDrive(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "tt50a", db.PhaseDeploying)
	tk, _ := g.DB().GetTicket(ctx, "tt50a")
	spec := g.deriveSpec(ctx, tk, "CTO", FailureRule{Kind: "", Action: ""}, nil, time.Time{})
	if !spec.CliDrive {
		t.Fatal("部署中未置 CliDrive——blocked 部署票 resume 仍补 AI 席")
	}
	mkTicketAt(t, g, "tt50b", db.PhaseToDeploy)
	tk2, _ := g.DB().GetTicket(ctx, "tt50b")
	spec2 := g.deriveSpec(ctx, tk2, "CTO", FailureRule{}, nil, time.Time{})
	if !spec2.CliDrive {
		t.Fatal("待部署 CliDrive 回归")
	}
}

// 锚③：blocked 票 onStall 零 stall_suspect/零杀席。
func TestT50_BlockedNoStall(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "tt50c", db.PhaseWorking)
	_, _ = g.DB().Transition(ctx, "tt50c", db.PhaseBlocked, db.EdgeBlocked, "gateway", "x")
	g.onStall("tt50c")
	evs, _ := g.DB().Events(ctx, "tt50c")
	for _, e := range evs {
		if e.Kind == "stall_suspect" || e.Kind == "stall_kill" {
			t.Fatalf("blocked 票仍触停滞径: %s", e.Kind)
		}
	}
}
