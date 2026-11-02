package gateway

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

func sendDeliverT5(t *testing.T, stream pbv1.SeatGateway_AttachClient, ref string) {
	t.Helper()
	if err := stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Deliver{
		Deliver: &pbv1.Deliver{Ref: ref}}}); err != nil {
		t.Fatal(err)
	}
}

// TestT5_巡检免审径 票种闸锚：patrol-* 票 deliver→patrol_done→完成
// （不进待审/待部署）；普通票不误伤仍走 待审。
func TestT5_巡检免审径(t *testing.T) {
	g, dir := newTestGateway(t, nil)
	ctx := context.Background()

	// patrol 票：施工中→deliver→完成。
	mkTicketAt(t, g, "patrol-t5", db.PhaseWorking)
	if err := g.DB().CreateRun(ctx, &db.Run{
		RunID: "rt5", TicketID: "patrol-t5", Role: "巡检", Shell: "devin",
		RunToken: "tok5"}); err != nil {
		t.Fatal(err)
	}
	stream := attachClient(t, g, filepath.Join(dir, "run", "meta.sock"), "rt5", "tok5")
	sendDeliverT5(t, stream, "r5")
	waitPhase(t, g, "patrol-t5", db.PhaseDone, 10*time.Second)
	evs, _ := g.DB().Events(ctx, "patrol-t5")
	var saw bool
	for _, e := range evs {
		saw = saw || e.Kind == "patrol_done"
	}
	if !saw {
		t.Fatal("缺 patrol_done 边留痕")
	}
	for _, e := range evs {
		if e.Kind == "deliver" || e.Kind == "audit_start" {
			t.Fatalf("patrol 票误入审计径: %+v", e)
		}
	}
	stream.CloseSend()

	// 普通票对照：deliver→待审（不误伤）。
	mkTicketAt(t, g, "normal-t5", db.PhaseWorking)
	if err := g.DB().CreateRun(ctx, &db.Run{
		RunID: "rn5", TicketID: "normal-t5", Role: "施工", Shell: "devin",
		RunToken: "tokn"}); err != nil {
		t.Fatal(err)
	}
	stream2 := attachClient(t, g, filepath.Join(dir, "run", "meta.sock"), "rn5", "tokn")
	sendDeliverT5(t, stream2, "n5")
	waitPhase(t, g, "normal-t5", db.PhaseReview, 10*time.Second)
	stream2.CloseSend()
}
