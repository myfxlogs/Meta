package gateway

import (
	"context"
	"path/filepath"
	"testing"

	"meta/internal/db"
)

// TestP3_DeployStateOK 态判锚（0044 换源后口径）：cmd_run 旁证通道——
// deploy 步 exit=0＋装着戳==期戳＋socket 活→true；deploy 步红/无行→false
// （态判不证席死活——restart 杀席是必然）。
func TestP3_DeployStateOK(t *testing.T) {
	g, dir := newTestGateway(t, nil)
	ctx := context.Background()
	inst := filepath.Join(dir, "bin", "meta")
	t.Setenv("META_INSTALL", inst)
	stubDeploySeams(t, map[string]string{inst: "abc-deploy"}, "abc", nil)
	if err := g.DB().UpsertProject(ctx, db.Project{
		ID: "P", RepoURL: dir, DefaultBranch: "master"}); err != nil {
		t.Fatal(err)
	}
	mkTicketAt(t, g, "tp3", db.PhaseDeploying)
	if err := g.DB().SetTicketProject(ctx, "tp3", "P"); err != nil {
		t.Fatal(err)
	}
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
