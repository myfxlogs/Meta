package board_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	connect "connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"meta/internal/board"
	"meta/internal/db"
	"meta/internal/gateway"
	pbv1 "meta/internal/pb/metav1"
	"meta/internal/pb/metav1/metav1connect"
)

// ---------- 测试用席位（helper process） ----------

// TestHelperProcess idle 模式：spawn 后永不 attach（seat_gone 径的亡席）。
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_HELPER_SEAT") != "1" {
		return
	}
	select {}
}

const boardToken = "boardtok-0012"

// rig 真网关＋板服务全接线（Connect TCP＋unix socket gRPC 双面）。
type rig struct {
	g       *gateway.Gateway
	runDir  string
	sock    string
	svc     *board.Service
	bl      *board.Listener
	cli     metav1connect.BoardClient
	gconn   *grpc.ClientConn
	gcli    pbv1.BoardClient
	seatCli pbv1.SeatGatewayClient
}

func newRig(t *testing.T, seedToken bool) *rig {
	t.Helper()
	dir := t.TempDir()
	runDir := filepath.Join(dir, "run")
	// vault.key 先于 New 落位——网关注入时载入。
	key, err := db.LoadOrCreateVaultKey(filepath.Join(runDir, "vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(runDir, "meta.sock")
	g, err := gateway.New(gateway.Config{
		DBPath:     filepath.Join(dir, "meta.db"),
		RunDir:     runDir,
		SocketPath: sock,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.Close)
	if seedToken {
		ct, err := db.EncryptSecret(key, []byte(boardToken+"\n")) // 种子带尾换行也要过
		if err != nil {
			t.Fatal(err)
		}
		if err := g.DB().PutSecret(context.Background(), gateway.BoardTokenName, ct, nil); err != nil {
			t.Fatal(err)
		}
	}
	svc := board.New(board.Deps{
		DB:          g.DB(),
		Subscribe:   g.Subscribe,
		Unsubscribe: g.Unsubscribe,
		CheckToken:  g.CheckBoardToken,
	})
	pbv1.RegisterBoardServer(g.GRPCServer(), svc.GRPC())
	go g.Serve()

	bl, err := board.ListenAndServe(svc, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(bl.Close)

	conn, err := grpc.NewClient("unix://"+sock,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	tr := connecthttp.NewTransport(http.DefaultClient, "http://"+bl.Addr)
	return &rig{
		g: g, runDir: runDir, sock: sock, svc: svc, bl: bl,
		cli:     metav1connect.NewBoardClient(connect.NewClient(tr)),
		gconn:   conn,
		gcli:    pbv1.NewBoardClient(conn),
		seatCli: pbv1.NewSeatGatewayClient(conn),
	}
}

// mkRun 造 run 行（fake 进程径——Attach 验 token 用）。
func mkRun(t *testing.T, r *rig, runID, ticketID, role, token string) {
	t.Helper()
	if err := r.g.DB().CreateRun(context.Background(), &db.Run{
		RunID:      runID,
		TicketID:   ticketID,
		Role:       role,
		Shell:      "devin/swe-2-high",
		RunToken:   token,
		SocketPath: filepath.Join(r.runDir, "seats", runID+".sock"),
		LogPath:    filepath.Join(r.runDir, "logs", runID+".log"),
	}); err != nil {
		t.Fatal(err)
	}
}

// mkAssigned 建票并推到 已派。
func mkAssigned(t *testing.T, r *rig, id string) {
	t.Helper()
	d := r.g.DB()
	ctx := context.Background()
	if err := d.CreateTicket(ctx, id, id+" 票", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Transition(ctx, id, db.PhaseDesigning, db.EdgeDesignStart, "test", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Transition(ctx, id, db.PhaseAssigned, db.EdgeDispatch, "test", ""); err != nil {
		t.Fatal(err)
	}
}

// attach 真 gRPC 流 Attach+Claim（Claim 可选）——真实事件径。
func attach(t *testing.T, r *rig, token, ticketID string, claim bool) pbv1.SeatGateway_AttachClient {
	t.Helper()
	st, err := r.seatCli.Attach(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{
		Auth: &pbv1.RunAuth{RunToken: token}}}); err != nil {
		t.Fatal(err)
	}
	if claim {
		if err := st.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{
			Claim: &pbv1.Claim{TicketId: ticketID}}}); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

// recvUntil 有界等帧（谓词命中即返；10s 判死——非轮询，ctx 超时兜底）。
func recvUntil(t *testing.T, stream metav1connect.BoardSeatBoardClientStream, what string, pred func(*pbv1.BoardFrame) bool) *pbv1.BoardFrame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type res struct {
		f *pbv1.BoardFrame
		e error
	}
	for {
		ch := make(chan res, 1)
		go func() {
			f, e := stream.Receive()
			ch <- res{f, e}
		}()
		select {
		case r := <-ch:
			if r.e != nil {
				t.Fatalf("收帧失败等 %s: %v", what, r.e)
			}
			if pred(r.f) {
				return r.f
			}
		case <-ctx.Done():
			t.Fatalf("等帧超时: %s", what)
		}
	}
}

// ---------- token 闸（锚 4＋对抗锚 b） ----------

func TestBoard_token闸_failclosed(t *testing.T) {
	r := newRig(t, false) // secret.board.token 行缺——任何 token 全拒
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := r.cli.SeatBoard(ctx, &pbv1.BoardReq{Token: boardToken})
	if err == nil {
		_, err = stream.Receive()
	}
	assertUnauth(t, err)

	// unix socket gRPC 面同闸。
	gs, err := r.gcli.SeatBoard(ctx, &pbv1.BoardReq{Token: boardToken})
	if err == nil {
		_, err = gs.Recv()
	}
	assertUnauth(t, err)
}

// 对抗锚 b：等值比较行翻（!=/恒真）→ 错 token 也放行——本锚即红。
func TestBoard_token闸_错token(t *testing.T) {
	r := newRig(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, bad := range []string{"wrong-tok", "", boardToken + "x"} {
		stream, err := r.cli.SeatBoard(ctx, &pbv1.BoardReq{Token: bad})
		if err == nil {
			_, err = stream.Receive()
		}
		assertUnauth(t, err)
	}
	// 对 token 放行（种子尾换行不入比较）。
	stream, err := r.cli.SeatBoard(ctx, &pbv1.BoardReq{Token: boardToken})
	if err != nil {
		t.Fatal(err)
	}
	f, err := stream.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if f.GetFull() == nil {
		t.Fatalf("首帧非 full: %v", f)
	}
	_ = stream.Close()
}

func assertUnauth(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("应拒却放行")
	}
	var ce *connect.Error
	if errors.As(err, &ce) && ce.Code() == connect.CodeUnauthenticated {
		return
	}
	if st, ok := status.FromError(err); ok && st.Code() == codes.Unauthenticated {
		return
	}
	t.Fatalf("拒因非 Unauthenticated: %v", err)
}

// ---------- 全态帧（锚 2＋对抗锚 a） ----------

// 对抗锚 a：full 组装行删→接入空板——本锚断言座位/票/预算/stage 字段全在。
func TestBoard_full帧_组装全(t *testing.T) {
	r := newRig(t, true)
	ctx := context.Background()
	mkAssigned(t, r, "T1")
	mkRun(t, r, "run-a", "T1", "施工", "tok-a")
	if err := r.g.DB().MarkAttached(ctx, "run-a"); err != nil {
		t.Fatal(err)
	}
	if err := r.g.DB().AppendEvent(ctx, "T1", "progress", db.Event{
		Actor: "run-a", Note: "写码 联调中"}); err != nil {
		t.Fatal(err)
	}
	if err := r.g.DB().SetBudgetCaps(ctx, "T1", 400, 4000); err != nil {
		t.Fatal(err)
	}
	if err := r.g.DB().AddUsage(ctx, "T1", 123, true); err != nil {
		t.Fatal(err)
	}
	// 死席与终态票不入板。
	mkRun(t, r, "run-dead", "T1", "施工", "tok-d")
	if err := r.g.DB().MarkDead(ctx, "run-dead"); err != nil {
		t.Fatal(err)
	}
	if err := r.g.DB().CreateTicket(ctx, "T9", "完成票", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := r.g.DB().Transition(ctx, "T9", db.PhaseVoid, db.EdgeVoid, "test", ""); err != nil {
		t.Fatal(err)
	}

	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	stream, err := r.cli.SeatBoard(cctx, &pbv1.BoardReq{Token: boardToken})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	f, err := stream.Receive()
	if err != nil {
		t.Fatal(err)
	}
	full := f.GetFull()
	if full == nil {
		t.Fatalf("首帧非 full: %v", f)
	}
	if len(full.Seats) != 1 {
		t.Fatalf("seats=%d 预期 1（死席须滤）: %+v", len(full.Seats), full.Seats)
	}
	seat := full.Seats[0]
	if seat.RunId != "run-a" || seat.Role != "施工" || seat.TicketId != "T1" {
		t.Fatalf("席位字段错: %+v", seat)
	}
	if seat.Shell != "devin" || seat.Model != "swe-2-high" {
		t.Fatalf("壳/模型拆解错: %+v", seat)
	}
	if seat.Stage != "写码" || seat.Note != "联调中" {
		t.Fatalf("stage 列错: %+v", seat)
	}
	if seat.Used != 123 || seat.Cap != 400 || seat.UsedTotal != 123 || seat.HardCap != 4000 {
		t.Fatalf("预算列错: %+v", seat)
	}
	if !seat.Attached {
		t.Fatalf("attached 应为 true: %+v", seat)
	}
	if len(full.Tickets) != 1 {
		t.Fatalf("tickets=%d 预期 1（终态票须滤）: %+v", len(full.Tickets), full.Tickets)
	}
	tk := full.Tickets[0]
	if tk.Id != "T1" || tk.Phase != "已派" || tk.UsedTotal != 123 || tk.HardCap != 4000 {
		t.Fatalf("票字段错: %+v", tk)
	}
}

// ---------- 变化帧（锚 3） ----------

func TestBoard_变化帧三型(t *testing.T) {
	r := newRig(t, true)
	ctx := context.Background()
	mkAssigned(t, r, "T1")

	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	stream, err := r.cli.SeatBoard(cctx, &pbv1.BoardReq{Token: boardToken})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if f, err := stream.Receive(); err != nil || f.GetFull() == nil {
		t.Fatalf("首帧非 full: %v %v", f, err)
	}

	// attach+claim→施工中：SEAT_UPSERT + TICKET_UPSERT（phase:claim）
	mkRun(t, r, "run-a", "T1", "施工", "tok-a")
	st := attach(t, r, "tok-a", "T1", true)
	defer st.CloseSend()
	recvUntil(t, stream, "seat upsert(run-a)", func(f *pbv1.BoardFrame) bool {
		u := f.GetSeatUpsert()
		return u != nil && u.RunId == "run-a" && u.Attached
	})
	recvUntil(t, stream, "ticket upsert(施工中)", func(f *pbv1.BoardFrame) bool {
		u := f.GetTicketUpsert()
		return u != nil && u.Id == "T1" && u.Phase == "施工中"
	})

	// progress→seat upsert（stage 变）
	if err := st.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Progress{
		Progress: &pbv1.Progress{Stage: "自测", Note: "锚3"}}}); err != nil {
		t.Fatal(err)
	}
	recvUntil(t, stream, "seat upsert(stage=自测)", func(f *pbv1.BoardFrame) bool {
		u := f.GetSeatUpsert()
		return u != nil && u.RunId == "run-a" && u.Stage == "自测" && u.Note == "锚3"
	})

	// kill 席→seat_gone（真进程 SIGKILL→双源判死径）
	run, err := r.g.Spawn(ctx, "T1", "施工", "testshell",
		[]string{os.Args[0], "-test.run=TestHelperProcess", "-test.v=false"},
		append(os.Environ(), "GO_HELPER_SEAT=1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(run.PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	recvUntil(t, stream, "seat_gone("+run.RunID+")", func(f *pbv1.BoardFrame) bool {
		g := f.GetSeatGone()
		return g != nil && g.RunId == run.RunID
	})

	// 票相位迁移→ticket upsert（DispatchSeat 走边 已立→已派）
	if err := r.g.DB().CreateTicket(ctx, "T2", "新票", nil); err != nil {
		t.Fatal(err)
	}
	if err := r.g.DispatchSeat(ctx, "T2", "施工", gateway.SeatSpec{}); err != nil {
		t.Fatal(err)
	}
	recvUntil(t, stream, "ticket upsert(T2 已派)", func(f *pbv1.BoardFrame) bool {
		u := f.GetTicketUpsert()
		return u != nil && u.Id == "T2" && u.Phase == "已派"
	})
}

// ---------- 幂等重放（重复事件不脏板） ----------

func TestBoard_变化帧幂等(t *testing.T) {
	r := newRig(t, true)
	ctx := context.Background()
	mkAssigned(t, r, "T1")
	mkRun(t, r, "run-a", "T1", "施工", "tok-a")

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	stream, err := r.cli.SeatBoard(cctx, &pbv1.BoardReq{Token: boardToken})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.Receive(); err != nil {
		t.Fatal(err)
	}
	// 同一事件源触发两帧 seat upsert——内容恒为 DB 现态重读（相同 stage），
	// 客户端重放第二帧得到同一态：幂等。
	st := attach(t, r, "tok-a", "T1", true)
	defer st.CloseSend()
	prog := &pbv1.SeatMsg{Body: &pbv1.SeatMsg_Progress{
		Progress: &pbv1.Progress{Stage: "写码", Note: "幂等"}}}
	if err := st.Send(prog); err != nil {
		t.Fatal(err)
	}
	if err := st.Send(prog); err != nil {
		t.Fatal(err)
	}
	n := 0
	recvUntil(t, stream, "幂等第二帧", func(f *pbv1.BoardFrame) bool {
		u := f.GetSeatUpsert()
		if u != nil && u.RunId == "run-a" && u.Stage == "写码" && u.Note == "幂等" {
			n++
		}
		return n >= 2
	})
}

// ---------- loopback 闸 ----------

func TestBoard_非loopback拒起(t *testing.T) {
	r := newRig(t, true)
	if _, err := board.ListenAndServe(r.svc, "0.0.0.0:0"); err == nil {
		t.Fatal("非 loopback 地址须拒")
	}
}
