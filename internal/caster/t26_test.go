package caster

// T26 病6 接线·路由表面：CTO-B 补发消费行＋audit_pass 部署行钦定 cli_drive；
// DispatchAction.cli_drive 透传 DispatchReq（对抗锚提名行：透传行——删则
// 表上 cli_drive 落空）。

import (
	"context"
	"testing"

	pbv1 "meta/internal/pb/metav1"
)

func TestT26_RouteTableCliDrive(t *testing.T) {
	tab, err := LoadTable("../../config/caster.textproto")
	if err != nil {
		t.Fatal(err)
	}
	var toDeploy, auditPass bool
	for _, rt := range tab.Routes {
		for _, a := range rt.Actions {
			d := a.GetDispatch()
			if d == nil || !d.CliDrive {
				continue
			}
			if rt.Kind == "spawn_needed" && rt.NoteContains == "CTO-B" {
				toDeploy = true
			}
			if rt.Kind == "audit_pass" {
				auditPass = true
			}
		}
	}
	if !toDeploy {
		t.Fatal("spawn_needed CTO-B 消费行未钦定 cli_drive")
	}
	if !auditPass {
		t.Fatal("audit_pass 部署派发行未钦定 cli_drive")
	}
}

// TestT26_DispatchReqCliDrive 透传：DispatchAction.cli_drive → DispatchReq。
func TestT26_DispatchReqCliDrive(t *testing.T) {
	f := &fakeAdmin{}
	e := &Engine{cfg: Config{Logf: func(string, ...any) {}}}
	e.execDispatch(context.Background(), f,
		&pbv1.EventItem{TicketId: "t9", Provenance: "p"},
		&pbv1.DispatchAction{Role: "CTO", CliDrive: true})
	if len(f.dispatches) == 0 || !f.dispatches[0].CliDrive {
		t.Fatal("cli_drive 未透传进 DispatchReq")
	}
}
