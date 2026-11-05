package gateway

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// writePresence 造在岗锁件（自进程 pid——活凭据）。
func writePresence(t *testing.T, g *Gateway, pid int) {
	t.Helper()
	f := filepath.Join(g.cfg.RunDir, "ceo-present.json")
	body := fmt.Sprintf(`{"shell":"devin","pid":%d,"since":"x"}`, pid)
	if err := os.WriteFile(f, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// memoryPutDirect 直连流发一帧 put（绕 CLI）。
func memoryPutDirect(t *testing.T, g *Gateway, token string) (int64, error) {
	st := &fakeMemoryStream{ctx: context.Background()}
	err := g.CeoMemory(st.enqueue(&pbv1.MemoryOp{RunToken: token,
		Op: &pbv1.MemoryOp_Put{Put: &pbv1.MemoryPut{
			Kind: "decision", Title: "t", Content: []byte("c")}}}))
	if err == io.EOF {
		return 0, nil // 客户端收流 EOF＝正常收口（grpc 同径）
	}
	return 0, err
}

// fakeMemoryStream 最小 CeoGateway_MemoryServer：预载一帧。
type fakeMemoryStream struct {
	ctx     context.Context
	pending *pbv1.MemoryOp
	sent    []*pbv1.MemoryOp
}

func (f *fakeMemoryStream) enqueue(m *pbv1.MemoryOp) *fakeMemoryStream {
	f.pending = m
	return f
}
func (f *fakeMemoryStream) Recv() (*pbv1.MemoryOp, error) {
	if f.pending == nil {
		return nil, io.EOF
	}
	m := f.pending
	f.pending = nil
	return m, nil
}
func (f *fakeMemoryStream) Send(m *pbv1.MemoryOp) error {
	f.sent = append(f.sent, m)
	return nil
}
func (f *fakeMemoryStream) Context() context.Context     { return f.ctx }
func (f *fakeMemoryStream) SendMsg(any) error            { return nil }
func (f *fakeMemoryStream) RecvMsg(any) error            { return nil }
func (f *fakeMemoryStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeMemoryStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeMemoryStream) SetTrailer(metadata.MD)       {}
func (f *fakeMemoryStream) Header() (metadata.MD, error) { return nil, nil }
func (f *fakeMemoryStream) Trailer() metadata.MD         { return nil }

// TestT2_在岗锁鉴权 三锚：锁活 pid→无 token 放行写；锁缺/ pid 死→拒。
func TestT2_在岗锁鉴权(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()

	// 无锁→拒。
	if _, err := memoryPutDirect(t, g, ""); err == nil {
		t.Fatal("无锁竟放行——鉴权洞")
	}
	// 死 pid 锁→拒。
	writePresence(t, g, 999999)
	if _, err := memoryPutDirect(t, g, ""); err == nil {
		t.Fatal("死 pid 锁竟放行——凭据失效未检")
	}
	// 活 pid 锁→放行＋provenance=terminal。
	writePresence(t, g, syscall.Getpid())
	if _, err := memoryPutDirect(t, g, ""); err != nil {
		t.Fatalf("活锁应放行: %v", err)
	}
	rows, _ := g.DB().ListMemory(ctx, "decision", 10)
	if len(rows) == 0 || rows[0].Provenance != "terminal" {
		t.Fatalf("落账 provenance 错: %+v", rows)
	}
	if !strings.HasPrefix(rows[0].EventRef, "terminal:") {
		t.Fatalf("eventRef 缺 terminal 戳: %+v", rows[0])
	}
}

// TestT2_Rule落账 锚：rule 成功裁决→ceo_memory 自动写行（decision+
// 理由+provenance）；失败裁决不写。
func TestT2_Rule落账(t *testing.T) {
	g, _ := newTestGateway(t, nil)
	ctx := context.Background()
	writePresence(t, g, syscall.Getpid())
	mkTicketAt(t, g, "r1", db.PhaseWorking)

	// 败径（票不存在）不落账。
	if ack, err := g.CeoRule(ctx, &pbv1.Ruling{
		Action: "budget_raise", TicketId: "ghost", HardCap: 9}); err == nil && !ack.Ok {
		// ok 为 false 是预期拒
	} else if ack != nil && ack.Ok {
		t.Fatal("幽灵票裁决竟成功")
	}

	ack, err := g.CeoRule(ctx, &pbv1.Ruling{
		Action: "budget_raise", TicketId: "r1", HardCap: 999, Note: "扩容"})
	if err != nil || !ack.Ok {
		t.Fatalf("rule: %v %+v", err, ack)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		rows, _ := g.DB().ListMemory(ctx, "decision", 10)
		for _, m := range rows {
			if strings.Contains(m.Title, "rule budget_raise r1") &&
				strings.Contains(string(m.Content), "扩容") &&
				m.Provenance == "terminal" {
				return
			}
		}
		if time.Now().After(deadline) {
			rs, _ := g.DB().ListMemory(ctx, "decision", 50)
			t.Fatalf("裁决未落账: %+v", rs)
		}
		<-time.After(20 * time.Millisecond)
	}
}
