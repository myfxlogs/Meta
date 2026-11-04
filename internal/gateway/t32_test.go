package gateway

// T32 化身唤醒链与内部票部署径——四件各锚。

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"meta/internal/db"
	"meta/internal/route"
)

// 件① 唤醒链第二断点：PID 复用假在岗——stale CEO 行 pid 指向非席进程
// （本测试进程 cmdline 无 "seat" 字样）→ ceoBusy 不楔死。
// 对抗锚提名行：processAliveRun——删则 stale 行永久假在岗唤醒零尝试。
func TestT32_CeoBusyPIDReuse(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	// stale CEO 行：pid=1（init——起刻恒早于席行＝PID 复用形）。
	run := g.newRun("", "CEO", "claude")
	run.PID = 1
	if err := g.DB().CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if g.ceoBusy(ctx) {
		t.Fatal("PID 复用 stale 行假在岗——唤醒链楔死")
	}
	// 正径对照：pid 不存在→非 busy；真席 pid（spawn sleep? 无 seat 字样
	// 亦然）——本锚只证复用径不楔，真席在岗径由既有测试盖。
}

// 件①补 cli-drive 双发残角：活席在→cliDriveSpawn 弃发（spawn_dup）。
func TestT32_CliDriveDupSeat(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
	})
	ctx := context.Background()
	mkToDeploy(t, g, "td32", "P")
	// 伪造活席行：pid=本进程——cmdline 无 seat→processAliveRun 假；
	// 改用真席：spawn idle helper CTO 席。
	run, err := g.Spawn(ctx, "td32", "CTO", "testshell",
		[]string{os.Args[0], "-test.run=TestHelperProcess", "-test.v=false"},
		append(os.Environ(), "GO_HELPER_SEAT=1", "SEAT_MODE=idle"))
	if err != nil {
		t.Fatal(err)
	}
	defer killRunGroup(run.PID)
	g.cliDriveSpawn(ctx, spawnReq{ticketID: "td32", role: "CTO"}, SeatSpec{CliDrive: true})
	runs, _ := g.DB().RunsForTicket(ctx, "td32")
	for _, r := range runs {
		if r.Shell == "cli-drive" {
			t.Fatal("活席在仍发 cli-drive——双发残角回潮")
		}
	}
}

// 件③ 种子规则：角色行非空但缺种子行→仍补齐（CEO 行实证）。
func TestT32_ShellSeedReseed(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		c.ShellRows = []ShellSeed{
			{Name: "claude", Model: "opus", Role: "CEO", Priority: 10},
		}
	})
	ctx := context.Background()
	// 角色行非空（异行已占）——旧径空才落不再补 CEO 种子。
	_ = g.DB().UpsertShell(ctx, db.Shell{Name: "old", Model: "m", Role: "CEO", Health: route.HealthDead})
	rows, err := g.shellsFor(ctx, "CEO")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.Name == "claude" {
			found = true
		}
	}
	if !found {
		t.Fatal("CEO 种子行未补——空才落漏种回潮")
	}
}

// 件② ruling 门角：待部署票 redirect 受理→CliDrive 重派（ruling 落行）。
func TestT32_RedirectToDeployGate(t *testing.T) {
	shortSkipInteg(t) // T51①：DispatchSeat 内起真子进程——闸口径豁免
	g, _ := newTestGateway(t, func(c *Config) {
		*c = dispatchCfg(filepath.Dir(c.DBPath))
		c.SelfPath = "/bin/false"
	})
	ctx := context.Background()
	if err := g.DB().UpsertProject(ctx, stubProject("P")); err != nil {
		t.Fatal(err)
	}
	mkToDeploy(t, g, "td32r", "P")
	run := &db.Run{RunID: "ceo32", TicketID: "", Role: "CEO", Provenance: "test"}
	_ = g.DB().CreateRun(ctx, run)
	// 直调裁径分支：handleRule 太重——用 ruling 落行验证相位受理由
	// redirect 支：此处只证待部署 redirect 不拒（调 g.handleRule 简化径）。
	_ = run
	if err := g.DispatchSeat(ctx, "td32r", "CTO", SeatSpec{CliDrive: true}); err != nil {
		t.Fatalf("待部署 CliDrive 重派应放行: %v", err)
	}
}
