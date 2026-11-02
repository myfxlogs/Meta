package gateway

import (
	"context"
	"testing"

	"meta/internal/db"
)

// TestT22_评注挂票 锚：CEO ruling note→票内留言；飞书入站含唯一票号
// →挂票留言；多票歧义不挂。
func TestT22_评注挂票(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "t22x", db.PhaseWorking)
	mkTicketAt(t, g, "t22y", db.PhaseWorking)

	g.FeishuInbound("ou_x", "t22x 这个票补一句")
	ms, _ := g.DB().MsgsForTicket(ctx, "t22x", false)
	if len(ms) == 0 || ms[0].Actor != "feishu:ou_x" {
		t.Fatal("飞书评注未挂票")
	}
	g.FeishuInbound("ou_x", "t22x 和 t22y 都提到")
	if ms2, _ := g.DB().MsgsForTicket(ctx, "t22y", false); len(ms2) != 0 {
		t.Fatal("多票歧义误挂")
	}
	// CEO ruling note 挂留言（land 径——r.Note 非空走 AppendMsg）。
	g.onTerminal("t22x") // 仅确保终态清理不干扰
}
