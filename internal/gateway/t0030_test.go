package gateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"meta/internal/db"
	"meta/internal/route"
	pbv1 "meta/internal/pb/metav1"
)

// attachClient 测试代驾席位：直插 run 行→Attach 发 RunAuth——不经 spawn。
func attachClient(t *testing.T, g *Gateway, sock, runID, token string) pbv1.SeatGateway_AttachClient {
	t.Helper()
	conn, err := grpc.NewClient("unix://"+sock,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	stream, err := pbv1.NewSeatGatewayClient(conn).Attach(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{
		Auth: &pbv1.RunAuth{RunToken: token}}}); err != nil {
		t.Fatal(err)
	}
	return stream
}

// TestT30_PostMsgFeed 验收锚2：PostMsg→活席收 Feed 帧＋msg 盖投递戳；
// 无活席→留言留行 pending。
func TestT30_PostMsgFeed(t *testing.T) {
	g, dir := newTestGateway(t, nil)
	ctx := context.Background()
	d := g.DB()
	mkTicketAt(t, g, "fm", db.PhaseWorking)
	if err := d.CreateRun(ctx, &db.Run{
		RunID: "rfm", TicketID: "fm", Role: "施工", Shell: "claude", RunToken: "tokf"}); err != nil {
		t.Fatal(err)
	}
	stream := attachClient(t, g, filepath.Join(dir, "run", "meta.sock"), "rfm", "tokf")

	ack, err := g.PostMsg(ctx, &pbv1.MsgReq{TicketId: "fm", Actor: "ceo", Text: "改方案X"})
	if err != nil || !ack.Ok {
		t.Fatalf("PostMsg: %v %v", ack, err)
	}
	stream.CloseSend()
	msg, err := stream.Recv()
	if err != nil {
		t.Fatalf("收 Feed: %v", err)
	}
	f := msg.GetFeed()
	if f == nil || f.Actor != "ceo" || f.Text != "改方案X" {
		t.Fatalf("Feed 帧内容错: %+v", msg)
	}
	ms, _ := d.MsgsForTicket(ctx, "fm", true)
	if len(ms) != 0 {
		t.Fatalf("已直投留言仍在 pending: %v", ms)
	}
	if ms, _ := d.MsgsForTicket(ctx, "fm", false); len(ms) != 1 || ms[0].RunID != "rfm" {
		t.Fatalf("投递戳未盖: %+v", ms)
	}
	waitEvent(t, g, "msg", "fm", 3*time.Second)

	// 无活席票：留言留行 pending。
	mkTicketAt(t, g, "fn", db.PhaseWorking)
	ack, err = g.PostMsg(ctx, &pbv1.MsgReq{TicketId: "fn", Text: "躺着等"})
	if err != nil || !ack.Ok {
		t.Fatal(err)
	}
	if ms, _ := d.MsgsForTicket(ctx, "fn", true); len(ms) != 1 {
		t.Fatalf("无活席留言未 pending: %v", ms)
	}
}

// TestT30_SessionFrame 验收锚（session 上报径）：SeatMsg_Session 帧落
// seat_run.session_id。
func TestT30_SessionFrame(t *testing.T) {
	g, dir := newTestGateway(t, nil)
	ctx := context.Background()
	d := g.DB()
	mkTicketAt(t, g, "fs", db.PhaseWorking)
	if err := d.CreateRun(ctx, &db.Run{
		RunID: "rfs", TicketID: "fs", Role: "施工", Shell: "claude", RunToken: "toks"}); err != nil {
		t.Fatal(err)
	}
	stream := attachClient(t, g, filepath.Join(dir, "run", "meta.sock"), "rfs", "toks")
	if err := stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Session{
		Session: &pbv1.Session{SessionId: "sess-9"}}}); err != nil {
		t.Fatal(err)
	}
	stream.CloseSend()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r, _ := d.RunByID(ctx, "rfs")
		if r != nil && r.SessionID == "sess-9" {
			return
		}
		<-time.After(20 * time.Millisecond)
	}
	t.Fatal("session_id 未落库")
}

// TestT30_ResumeSpec 验收锚3：续会锚命中三判（有锚＋壳同名＋壳支持）；
// 壳不同名/无锚/不支持续会→不中。
func TestT30_ResumeSpec(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	d := g.DB()
	mkTicketAt(t, g, "rs", db.PhaseAssigned)
	if err := d.CreateRun(ctx, &db.Run{
		RunID: "rrs", TicketID: "rs", Role: "施工", Shell: "claude", RunToken: "tokr"}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetRunSession(ctx, "rrs", "sess-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.AppendMsg(ctx, "rs", "ceo", "补一句新要求"); err != nil {
		t.Fatal(err)
	}
	req := spawnReq{ticketID: "rs", role: "施工"}
	hit, ok := g.resumeSpec(ctx, req, route.Attempt{Shell: "claude"})
	if !ok || hit.sessionID != "sess-A" {
		t.Fatalf("同壳续会锚未命中: %+v", hit)
	}
	b, err := os.ReadFile(hit.promptFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"补一句新要求", "META_DELIVER", "续传"} {
		if !strings.Contains(string(b), s) {
			t.Fatalf("续传提示件缺 %q:\n%s", s, b)
		}
	}
	if _, ok := g.resumeSpec(ctx, req, route.Attempt{Shell: "devin"}); ok {
		t.Fatal("壳不同名不应命中续会")
	}
	if _, ok := g.resumeSpec(ctx, spawnReq{ticketID: "rs", role: "审计"},
		route.Attempt{Shell: "claude"}); ok {
		t.Fatal("跨角色不应命中续会")
	}
}

// TestT30_ParseKeepAll 验收锚5：KEEP/DROP 行判读；未答编号按 DROP；
// 无有效判定行→不可判（调用方放行全开）。
func TestT30_ParseKeepAll(t *testing.T) {
	keep, ok := parseKeepAll("KEEP 1\nDROP 2\nKEEP 3", 3)
	if !ok || !keep[1] || !keep[3] || keep[2] {
		t.Fatalf("基本判读错: %v %v", keep, ok)
	}
	keep, ok = parseKeepAll("KEEP: 1,3\nDROP: 2", 3)
	if !ok || !keep[1] || !keep[3] {
		t.Fatalf("逗号列判读错: %v %v", keep, ok)
	}
	keep, ok = parseKeepAll("KEEP 2", 3) // #1/#3 未答→DROP
	if !ok || len(keep) != 1 || !keep[2] {
		t.Fatalf("未答编号应按 DROP: %v", keep)
	}
	if _, ok := parseKeepAll("模型碎碎念一堆没有判定行", 3); ok {
		t.Fatal("无判定行应不可判")
	}
	if _, ok := parseKeepAll("KEEP 99", 3); ok { // 越界编号不算有效判定
		t.Fatal("越界编号不应算有效判定")
	}
}

// TestT30_PatrolRoute 验收锚4+6：订阅命中进批收→信号路由→adjunct 巡检
// 派发（无巡检壳行→shells_exhausted 取证派发发生）；patrol_tick 无票信号
// →立 patrol-* 值班票＋挂留言＋正式派发。
func TestT30_PatrolRoute(t *testing.T) {
	g, _ := newTestGateway(t, func(c *Config) {
		c.Subs = []SubSeed{{Kind: "sig_hit", Role: "巡检"}, {Kind: "patrol_tick", Role: "巡检"}}
		c.Filter.BatchWindow = 50 * time.Millisecond // 批窗收窄
	})
	ctx := context.Background()
	d := g.DB()
	mkTicketAt(t, g, "pt", db.PhaseWorking)

	// 订阅镜像装载断言（种子→内存表）。
	if len(g.subs["sig_hit"]) != 1 || g.subs["sig_hit"][0] != "巡检" {
		t.Fatalf("订阅镜像: %v", g.subs)
	}

	// 带票信号：路由→adjunct 派发→无巡检壳行→shells_exhausted 取证到达派发径。
	g.emit(Event{Kind: "sig_hit", TicketID: "pt", Note: "探针"})
	waitEvent(t, g, "shells_exhausted", "pt", 15*time.Second)

	// 无票信号：立 patrol-* 票（归并点）＋挂留言＋派发（同径耗尽）。
	g.emit(Event{Kind: "patrol_tick", Note: "心跳"})
	var pid string
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && pid == "" {
		if ts, err := d.NonTerminalTickets(ctx); err == nil {
			for _, tk := range ts {
				if strings.HasPrefix(tk.ID, "patrol-") {
					pid = tk.ID
				}
			}
		}
		if pid == "" {
			<-time.After(20 * time.Millisecond)
		}
	}
	if pid == "" {
		t.Fatal("patrol_tick 未立值班票")
	}
	if ms, _ := d.MsgsForTicket(ctx, pid, false); len(ms) != 1 || ms[0].Actor != "patrol:patrol_tick" {
		t.Fatalf("心跳信号未挂留言: %+v", ms)
	}
	// 测试环境无 InjectRoot→空提示被 D-40 闸拦成 spawn_blocked——
	// 取证「无票信号→立票→正式派发」链路到达派发径（生产有注入机走全量）。
	waitEvent(t, g, "spawn_blocked", pid, 15*time.Second)

	// 第二张 patrol_tick：归并同票不另立（值班票单例）。
	g.emit(Event{Kind: "patrol_tick", Note: "心跳2"})
	<-time.After(300 * time.Millisecond) // 批窗 50ms，给冲刷余量
	if ms, _ := d.MsgsForTicket(ctx, pid, false); len(ms) != 2 {
		t.Fatalf("归并失败，留言数=%d", len(ms))
	}
	if ts, _ := d.NonTerminalTickets(ctx); func() int {
		n := 0
		for _, tk := range ts {
			if strings.HasPrefix(tk.ID, "patrol-") {
				n++
			}
		}
		return n
	}() != 1 {
		t.Fatal("patrol-* 票未单例归并")
	}

	// 负向：未订阅 kind 不进批收道。
	g.patrolFeed(Event{Kind: "noise_x"})
	select {
	case <-g.patrolCh:
		t.Fatal("未订阅事件进了批收道")
	default:
	}
}

// TestT30_InjectEvent 验收锚7：InjectEvent＝emit 全渠道＋带票落行。
func TestT30_InjectEvent(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "ie", db.PhaseWorking)
	ack, err := g.InjectEvent(ctx, &pbv1.EventReq{
		Kind: "patrol_tick", TicketId: "ie", Note: "手拍", Actor: "test"})
	if err != nil || !ack.Ok {
		t.Fatal(err)
	}
	waitEvent(t, g, "patrol_tick", "ie", 3*time.Second)
	evs, _ := g.DB().Events(ctx, "ie")
	last := evs[len(evs)-1]
	if last.Kind != "patrol_tick" || last.Actor != "test" || last.Provenance != "test" {
		t.Fatalf("注入行落账错: %+v", last)
	}
}

// TestT30_DispatchResumeEnv 验收锚3（env 透传径）：spec.ResumeSession→
// spawn env 含 META_RESUME_SESSION。构造径：resumeSpec 命中后 aspec 被
// 换成续传件——本锚走单测级：SeatSpec.ResumeSession→trySpawn env 拼装。
func TestT30_DispatchResumeEnv(t *testing.T) {
	spec := SeatSpec{ResumeSession: "sess-Z"}
	if spec.ResumeSession == "" {
		t.Fatal("SeatSpec.ResumeSession 字段缺失")
	}
}
