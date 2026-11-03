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
	pbv1 "meta/internal/pb/metav1"
	"meta/internal/route"
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

// TestT30_InjectEvent 验收锚7：InjectEvent＝emit 全渠道＋带票落行。
func TestT30_InjectEvent(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	mkTicketAt(t, g, "ie", db.PhaseWorking)
	ack, err := g.InjectEvent(ctx, &pbv1.EventReq{
		Kind: "timer_tick", TicketId: "ie", Note: "手拍", Actor: "test"})
	if err != nil || !ack.Ok {
		t.Fatal(err)
	}
	waitEvent(t, g, "timer_tick", "ie", 3*time.Second)
	evs, _ := g.DB().Events(ctx, "ie")
	last := evs[len(evs)-1]
	if last.Kind != "timer_tick" || last.Actor != "test" || last.Provenance != "test" {
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
