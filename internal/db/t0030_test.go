package db

import (
	"context"
	"testing"
)

// TestT30_MsgLifecycle ticket_msg 锚：落行→pending 查询→盖投递戳→
// pending 出列全量在列。
func TestT30_MsgLifecycle(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	mkTicket(t, d, "tm")
	id, err := d.AppendMsg(ctx, "tm", "ceo", "改方案X")
	if err != nil {
		t.Fatal(err)
	}
	ms, err := d.MsgsForTicket(ctx, "tm", true)
	if err != nil || len(ms) != 1 || ms[0].ID != id {
		t.Fatalf("pending 查询: %v %v", ms, err)
	}
	if err := d.MarkMsgDelivered(ctx, id, "runA"); err != nil {
		t.Fatal(err)
	}
	if ms, _ := d.MsgsForTicket(ctx, "tm", true); len(ms) != 0 {
		t.Fatalf("投递后仍在 pending: %v", ms)
	}
	if ms, _ := d.MsgsForTicket(ctx, "tm", false); len(ms) != 1 || ms[0].RunID != "runA" {
		t.Fatalf("全量查询丢行/戳: %v", ms)
	}
}

// TestT30_SessionAnchor session_id 列锚：SetRunSession→
// LatestSessionForTicketRole 取回最近锚行；无锚票回 nil。
func TestT30_SessionAnchor(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	mkTicket(t, d, "ts")
	mkTicket(t, d, "tnone")
	for i, sid := range []string{"s1", "s2"} {
		r := &Run{RunID: "r" + sid, TicketID: "ts", Role: "施工", Shell: "claude",
			RunToken: "tok" + sid}
		if err := d.CreateRun(ctx, r); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if err := d.SetRunSession(ctx, r.RunID, sid); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := d.LatestSessionForTicketRole(ctx, "ts", "施工")
	if err != nil || got == nil || got.RunID != "rs1" || got.SessionID != "s1" {
		t.Fatalf("续会锚取错: %+v %v", got, err)
	}
	if got, _ := d.LatestSessionForTicketRole(ctx, "tnone", "施工"); got != nil {
		t.Fatalf("无锚票应 nil: %+v", got)
	}
	if got, _ := d.LatestSessionForTicketRole(ctx, "ts", "审计"); got != nil {
		t.Fatalf("跨角色不应见锚: %+v", got)
	}
}
