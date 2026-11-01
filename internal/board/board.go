// Package board 是前端席位板（总设计 §9/§3.1 前端面，票0012）：
// 接入即一帧全态（DB 组装：活 run＋非终态票＋预算＋最近 stage），
// 此后事件→变化帧（seat upsert/seat_gone/ticket upsert 三型——内容恒为
// DB 现态重读投影，幂等重放安全）。不存历史不查过去；token 闸 fail closed
//（secret.board.token 行缺→一律 Unauthenticated）。
//
// 同一实现两出：board_listen TCP 挂 Connect 二进制（浏览器径），
// unix socket gRPC 注册（meta board --socket 验收径）。
package board

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	connect "connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"meta/internal/db"
	"meta/internal/gateway"
	pbv1 "meta/internal/pb/metav1"
	"meta/internal/pb/metav1/metav1connect"
)

// errUnauthenticated token 闸唯一拒因（两侧适配器各自翻成 Connect/gRPC 码）。
var errUnauthenticated = errors.New("board token 无效或 secret.board.token 行缺")

// Deps 服务依赖——订户口/库/凭证闸全由网关侧接线（board 不自带写径）。
type Deps struct {
	DB          *db.DB
	Subscribe   func() chan gateway.Event
	Unsubscribe func(chan gateway.Event)
	CheckToken  func(ctx context.Context, token string) bool
}

// Service 板服务（同时是 Connect handler 与 gRPC 实现的核心）。
type Service struct{ deps Deps }

// New 建板。
func New(d Deps) *Service { return &Service{deps: d} }

// GRPC 返回 gRPC 适配器——注册进 unix socket 主 server（CLI --socket 径）。
func (s *Service) GRPC() pbv1.BoardServer { return grpcBoard{svc: s} }

// SeatBoard Connect 面（metav1connect.BoardHandler）。
func (s *Service) SeatBoard(ctx context.Context, req *pbv1.BoardReq, stream metav1connect.BoardSeatBoardServerStream) error {
	if err := s.serve(ctx, req.Token, stream.Send); err != nil {
		if errors.Is(err, errUnauthenticated) {
			return connect.NewError(connect.CodeUnauthenticated, err.Error())
		}
		return err
	}
	return nil
}

type grpcBoard struct {
	pbv1.UnimplementedBoardServer
	svc *Service
}

func (b grpcBoard) SeatBoard(req *pbv1.BoardReq, stream grpc.ServerStreamingServer[pbv1.BoardFrame]) error {
	if err := b.svc.serve(stream.Context(), req.Token, stream.Send); err != nil {
		if errors.Is(err, errUnauthenticated) {
			return status.Error(codes.Unauthenticated, err.Error())
		}
		return err
	}
	return nil
}

// serve 核心流：token 闸 → 先订后装（组装间隙事件以变化帧补发，幂等）→
// 全态帧 → 事件订阅循环推变化帧（禁定时——全靠事件源）。
func (s *Service) serve(ctx context.Context, token string, send func(*pbv1.BoardFrame) error) error {
	if !s.deps.CheckToken(ctx, token) {
		return errUnauthenticated
	}
	ch := s.deps.Subscribe()
	defer s.deps.Unsubscribe(ch)
	full, err := s.full(ctx)
	if err != nil {
		return fmt.Errorf("全态组装: %w", err)
	}
	if err := send(&pbv1.BoardFrame{Body: &pbv1.BoardFrame_Full{Full: full}}); err != nil {
		return err
	}
	for {
		select {
		case e := <-ch:
			for _, f := range s.frames(ctx, e) {
				if err := send(f); err != nil {
					return err
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// full 全态组装（对抗性锚 a 提名行：三件套——活 run＋非终态票＋预算/stage——
// 删则接入空板锚红）。
func (s *Service) full(ctx context.Context) (*pbv1.BoardFull, error) {
	runs, err := s.deps.DB.ActiveRuns(ctx)
	if err != nil {
		return nil, err
	}
	stages, err := s.deps.DB.LatestStages(ctx)
	if err != nil {
		return nil, err
	}
	tickets, err := s.deps.DB.NonTerminalTickets(ctx)
	if err != nil {
		return nil, err
	}
	out := &pbv1.BoardFull{}
	for _, r := range runs {
		out.Seats = append(out.Seats, s.seatRow(ctx, r, stages[r.TicketID]))
	}
	for _, t := range tickets {
		out.Tickets = append(out.Tickets, s.ticketRow(ctx, t))
	}
	return out, nil
}

// frames 事件→变化帧映射（三型）：RunID 非空→重读 run（dead/缺→seat_gone，
// 活→seat upsert）；TicketID 非空→重读票→ticket upsert（终态相位亦上行，
// 板只 upsert 不删行）。
func (s *Service) frames(ctx context.Context, e gateway.Event) []*pbv1.BoardFrame {
	var out []*pbv1.BoardFrame
	if e.RunID != "" {
		run, err := s.deps.DB.RunByID(ctx, e.RunID)
		if err == nil {
			if run == nil || run.Dead {
				out = append(out, &pbv1.BoardFrame{Body: &pbv1.BoardFrame_SeatGone{
					SeatGone: &pbv1.SeatGone{RunId: e.RunID}}})
			} else {
				out = append(out, &pbv1.BoardFrame{Body: &pbv1.BoardFrame_SeatUpsert{
					SeatUpsert: s.seatRow(ctx, run, s.stageOf(ctx, run.TicketID))}})
			}
		}
	}
	if e.TicketID != "" {
		if t, err := s.deps.DB.GetTicket(ctx, e.TicketID); err == nil && t != nil {
			out = append(out, &pbv1.BoardFrame{Body: &pbv1.BoardFrame_TicketUpsert{
				TicketUpsert: s.ticketRow(ctx, t)}})
		}
	}
	return out
}

// seatRow run 行→席位行投影（§9 列：角色/壳/模型/所领票/当前 stage/预算用量）。
func (s *Service) seatRow(ctx context.Context, r *db.Run, stageNote string) *pbv1.SeatRow {
	row := &pbv1.SeatRow{
		RunId:    r.RunID,
		Role:     r.Role,
		TicketId: r.TicketID,
		Attached: !r.AttachedAt.IsZero() || !r.AdoptedAt.IsZero(),
		Adjunct:  r.Adjunct,
	}
	row.Shell, row.Model = splitShell(r.Shell)
	if !r.StartedAt.IsZero() {
		row.StartedAtUnixNano = r.StartedAt.UnixNano()
	}
	stage, note, _ := strings.Cut(stageNote, " ")
	row.Stage, row.Note = stage, note
	if b, err := s.deps.DB.GetBudget(ctx, r.TicketID); err == nil {
		row.Used, row.Cap = b.Used, b.Cap
		row.UsedTotal, row.HardCap = b.UsedTotal, b.HardCap
	}
	return row
}

func (s *Service) ticketRow(ctx context.Context, t *db.Ticket) *pbv1.TicketRow {
	row := &pbv1.TicketRow{
		Id: t.ID, Title: t.Title, Phase: string(t.Phase), BudgetFrozen: t.BudgetFrozen,
	}
	if b, err := s.deps.DB.GetBudget(ctx, t.ID); err == nil {
		row.UsedTotal, row.HardCap = b.UsedTotal, b.HardCap
	}
	return row
}

// stageOf 单票最近 stage（变化帧径小查询——全态帧走 LatestStages 一次取全）。
func (s *Service) stageOf(ctx context.Context, ticketID string) string {
	m, err := s.deps.DB.LatestStages(ctx)
	if err != nil {
		return ""
	}
	return m[ticketID]
}

// splitShell 壳 label（"name/model" 或裸名）拆壳名/模型列。
func splitShell(label string) (name, model string) {
	if i := strings.IndexByte(label, '/'); i >= 0 {
		return label[:i], label[i+1:]
	}
	return label, ""
}

// Listener board_listen TCP 监听件（Connect 二进制面）。
type Listener struct {
	Addr string
	srv  *http.Server
	ln   net.Listener
}

// ListenAndServe 挂板：loopback 强闸——非 loopback 地址拒起（loopback+token
// 是钦定最小暴露面，§3.4）；CORS 全放行（静态页 file:// 零源预检要过，
// 凭证不在 header 在 token 字段）。
func ListenAndServe(svc *Service, addr string) (*Listener, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("board_listen 解析: %w", err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		if host != "localhost" {
			return nil, fmt.Errorf("board_listen 仅 loopback（钦定最小暴露面）: %s", host)
		}
	}
	server := connect.NewServer()
	metav1connect.RegisterBoardHandler(server, svc)
	mux := http.NewServeMux()
	connecthttp.Mount(mux, server)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("board_listen %s: %w", addr, err)
	}
	l := &Listener{Addr: ln.Addr().String(), ln: ln,
		srv: &http.Server{Handler: cors(mux)}}
	go func() { _ = l.srv.Serve(ln) }()
	return l, nil
}

// Close 停监听（不断流内连接——http.Server.Close 语义；进程退出即全断）。
func (l *Listener) Close() { _ = l.srv.Close() }

// cors 极简跨域放行（静态页从 file:// 或别端口接入都要过预检；
// 关防本体是 token 闸不在 origin）。
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		h.Set("Access-Control-Allow-Headers",
			"Content-Type, Connect-Protocol-Version, Connect-Timeout-Ms, "+
				"Connect-Accept-Encoding, Connect-Content-Encoding, Authorization, "+
				"X-Grpc-Web, X-User-Agent, Grpc-Timeout")
		h.Set("Access-Control-Expose-Headers",
			"Content-Type, Connect-Protocol-Version, Connect-Accept-Encoding, Connect-Content-Encoding")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
