package gateway

import (
	"context"
	"testing"

	"meta/internal/db"
)

// TestP3_DeployStateOK 态判锚：deploy 步 exit=0＋socket 活→true；
// deploy 步红/无行→false（态判不证席死活——restart 杀席是必然）。
func TestP3_DeployStateOK(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "tp3", db.PhaseDeploying)
	if g.deployStateOK(ctx, "tp3") {
		t.Fatal("无 deploy 行竟判成")
	}
	_ = g.DB().InsertCmdRun(ctx, &db.CmdRun{
		TicketID: "tp3", Step: "deploy", ExitCode: 1})
	if g.deployStateOK(ctx, "tp3") {
		t.Fatal("deploy 红步竟判成")
	}
	_ = g.DB().InsertCmdRun(ctx, &db.CmdRun{
		TicketID: "tp3", Step: "deploy", ExitCode: 0})
	if !g.deployStateOK(ctx, "tp3") {
		t.Fatal("已装+socket 活未判成")
	}
}
