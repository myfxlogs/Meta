package gateway

// T28 飞书面三件——一件一锚。

import (
	"context"
	"testing"

	"meta/internal/db"
)

// 件① 回复线程挂票：parent_id→feishu_card 反查父票→挂留言。
func TestT28_ReplyParentAttach(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "pa1", db.PhaseWorking)
	if err := g.DB().SaveFeishuCard(ctx, "pa1", "om_msg_parent", "oc_x"); err != nil {
		t.Fatal(err)
	}
	g.FeishuInbound("ou_x", "回复里补一句（无票号）", "om_msg_parent")
	ms, _ := g.DB().MsgsForTicket(ctx, "pa1", false)
	if len(ms) != 1 {
		t.Fatalf("回复未挂父票留言: %v", ms)
	}
	// parent 无卡行→走票号匹配径（不挂）。
	g.FeishuInbound("ou_x", "回复但父消息无卡", "om_none")
	if ms, _ := g.DB().MsgsForTicket(ctx, "pa1", false); len(ms) != 1 {
		t.Fatal("无卡 parent 误挂")
	}
}

// 件② 最长匹配：0034-p1 提及不被 0034 歧义弃挂。
// 件③ 边界匹配：长数字内嵌票号不挂（1230034 / 00345）。
func TestT28_TicketHitMatch(t *testing.T) {
	ids := []string{"0034", "0034-p1", "0077"}
	if got := ticketHit("看下 0034-p1 这票", ids); got != "0034-p1" {
		t.Fatalf("最长匹配应挂 0034-p1: %q", got)
	}
	if got := ticketHit("看下 0034 这票", ids); got != "0034" {
		t.Fatalf("根票应挂: %q", got)
	}
	if got := ticketHit("编号 1230034 的串", ids); got != "" {
		t.Fatalf("内嵌长数字应拒: %q", got)
	}
	if got := ticketHit("票 00345 不存在号", ids); got != "" {
		t.Fatalf("票号长串内嵌应拒: %q", got)
	}
	ids2 := []string{"ab12", "cd34"}
	if got := ticketHit("ab12 和 cd34 都提", ids2); got != "" {
		t.Fatalf("等长异号歧义应弃挂: %q", got)
	}
}

// 件②③ 入站全径：回复无 parent 时票号匹配照旧工作（回归）。
func TestT28_InboundMention(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "m1", db.PhaseWorking)
	mkTicketAt(t, g, "m1-p1", db.PhaseWorking)
	g.FeishuInbound("ou_x", "m1-p1 这票补一句", "")
	ms, _ := g.DB().MsgsForTicket(ctx, "m1-p1", false)
	if len(ms) != 1 {
		t.Fatal("最长匹配提及未挂")
	}
}
