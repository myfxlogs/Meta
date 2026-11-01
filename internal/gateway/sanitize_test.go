package gateway

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pbv1 "meta/internal/pb/metav1"
)

// TestLogTail_Clean 毒源点出口净化（票0019 锚2）：席位日志尾带毒字节→
// logTail 出合法串（决策 67 实证毒源 dispatch.go logTail→seat_err note）。
func TestLogTail_Clean(t *testing.T) {
	f := filepath.Join(t.TempDir(), "seat.log")
	content := append([]byte("ok line\n"), 0xff, 0xfe, 'a', 0x80)
	if err := os.WriteFile(f, content, 0o600); err != nil {
		t.Fatal(err)
	}
	got := logTail(f, 300)
	if got == "" {
		t.Fatal("logTail 空")
	}
	if !utf8.ValidString(got) {
		t.Fatalf("logTail 出毒: %q", got)
	}
}

// TestEventsAfter_BadRowMarked 喂毒端到端锚（票0019 锚2/锚5＋对抗锚，决策 68
// 钦定）：第二 sql 连接绕过净化直插毒 note（模拟决策 67 存量毒形态）→
// 真 socket AdminGateway.EventsAfter 无 Internal 错＋回读 item 全字段合法。
// 毒行 note 必含 U+FFFD（净化痕迹）——坏行走 bad_row 顶替亦红，锚的是
// 「净化径」不是「顶替径」（删 eventItem 净化行→bad_row→缺 �→红）。
func TestEventsAfter_BadRowMarked(t *testing.T) {
	g, dir := newTestGateway(t, nil)
	_ = g
	ctx := context.Background()
	// 存量毒注入：直插 DB——写面净化挡不住的历史遗留形态。
	sq, err := sql.Open("sqlite",
		filepath.Join(dir, "meta.db")+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	defer sq.Close()
	if _, err := sq.ExecContext(ctx,
		`INSERT INTO ticket_event (ticket_id, kind, from_phase, to_phase, actor, note, provenance, created_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		"t-poison", "progress", "", "", "legacy",
		string([]byte{0xff, 0xfe, 'a', 0x80}), "", time.Now().UnixNano()); err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.NewClient("unix://"+filepath.Join(dir, "run", "meta.sock"),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cli := pbv1.NewAdminGatewayClient(conn)
	el, err := cli.EventsAfter(ctx, &pbv1.EventCursor{SinceId: 0, Limit: 50})
	if err != nil {
		t.Fatalf("EventsAfter 崩（决策 67 复发）: %v", err)
	}
	var found bool
	for _, it := range el.Items {
		for name, v := range map[string]string{
			"Kind": it.Kind, "Note": it.Note, "Actor": it.Actor,
			"TicketId": it.TicketId, "Role": it.Role, "Provenance": it.Provenance,
		} {
			if !utf8.ValidString(v) {
				t.Fatalf("item.%s 含毒: %q", name, v)
			}
		}
		if it.TicketId == "t-poison" {
			found = true
			if it.Kind != "progress" {
				t.Fatalf("毒行被顶替（应净化非 bad_row）: %+v", it)
			}
			if !strings.Contains(it.Note, "�") {
				t.Fatalf("毒行 note 缺 U+FFFD——净化径未走: %q", it.Note)
			}
		}
	}
	if !found {
		t.Fatal("毒行未回读——事件页缺行")
	}
}

// TestObserve_BadEventMarked 流防崩锚（票0019 锚2＋对抗锚）：两段——
// ①EventCh 进程内道：emit 毒 note→订户收到净串（锚 emit 净化行——
// 删之→本段红）；②真 socket Observe：订户 Recv 得帧不返错且字段合法
// （锚流发送净化/预检——删之→毒帧 marshal 崩流→本段红）。
func TestObserve_BadEventMarked(t *testing.T) {
	g, dir := newTestGateway(t, nil)
	// ①进程内道：emit 净化直证（对抗锚提名行）。
	g.emit(Event{Kind: "poison", TicketID: "t1",
		Note: string([]byte{0xff, 0xfe, 'a'})})
	select {
	case e := <-g.EventCh():
		if !utf8.ValidString(e.Note) || !utf8.ValidString(e.Kind) {
			t.Fatalf("emit 出毒: %+v", e)
		}
		if e.Kind != "poison" {
			t.Fatalf("事件被顶替——bad_row 应只在流出面: %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("EventCh 无帧")
	}
	// ②真 socket Observe：订户登记有窗——有界重发至首帧落地。
	conn, err := grpc.NewClient("unix://"+filepath.Join(dir, "run", "meta.sock"),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cli := pbv1.NewAdminGatewayClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	st, err := cli.Observe(ctx, &pbv1.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for i := 0; i < 100; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			g.emit(Event{Kind: "poison", TicketID: "t1",
				Note: string([]byte{0xff, 0xfe, 'a'})})
			<-time.After(10 * time.Millisecond)
		}
	}()
	ev, err := st.Recv()
	if err != nil {
		t.Fatalf("Observe 断流（决策 67 复发）: %v", err)
	}
	if !utf8.ValidString(ev.Note) || !utf8.ValidString(ev.Kind) {
		t.Fatalf("ObsEvent 含毒: %+v", ev)
	}
	if ev.Kind != "poison" {
		t.Fatalf("帧被顶替（应净化非 bad_row）: %+v", ev)
	}
}
