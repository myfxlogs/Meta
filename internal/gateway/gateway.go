// Package gateway 是 meta 网关进程本体：席位面 Attach（RunAuth 首帧）、
// spawn＋per-seat UDS、SIGCHLD+EOF 双源判死、全相位启动对账、一次性闹钟。
// 网关无智能：只出事件、执机械闸；语义裁决归智能体（§3）。
package gateway

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"google.golang.org/grpc"

	"meta/internal/clock"
	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
)

// Config 网关运行配置。
type Config struct {
	DBPath           string        // SQLite 库路径
	RunDir           string        // run/ 目录：seats/ logs/ vault.key 在其下
	SocketPath       string        // 席位面主 socket（unix）
	ProgressTimeout  time.Duration // 进度帧闹钟，缺省 30min（§5.2）
	AuthTimeout      time.Duration // Attach 首帧等待上限，缺省 5s
	AdoptDialTimeout time.Duration // 收养拨号上限，缺省 2s
}

func (c *Config) withDefaults() {
	if c.ProgressTimeout <= 0 {
		c.ProgressTimeout = 30 * time.Minute
	}
	if c.AuthTimeout <= 0 {
		c.AuthTimeout = 5 * time.Second
	}
	if c.AdoptDialTimeout <= 0 {
		c.AdoptDialTimeout = 2 * time.Second
	}
}

// runState 一张 seat_run 的运行时镜像＋双源判死所需的两条事件通道。
type runState struct {
	run *db.Run
	cmd *os.Process // spawn 径才有；收养径无柄

	mu           sync.Mutex
	attached     bool
	streamClosed bool
	exited       bool
	exitCode     int
	dead         bool
	adopted      bool
}

// Gateway 网关。
type Gateway struct {
	pbv1.UnimplementedSeatGatewayServer

	cfg      Config
	database *db.DB
	srv      *grpc.Server
	ln       net.Listener
	vaultKey []byte // secret 表主密钥；缺载时 Secret RPC 全拒

	mu     sync.Mutex
	runs   map[string]*runState
	alarms map[string]*clock.Alarm
	events chan Event
	closed bool
}

// New 建仓：开库 → 建 run 目录 → 主 socket → 全相位启动对账。
// 对账在监听前完成——席位连上时世界已一致。
func New(cfg Config) (*Gateway, error) {
	cfg.withDefaults()
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	g := &Gateway{
		cfg:      cfg,
		database: d,
		runs:     map[string]*runState{},
		alarms:   map[string]*clock.Alarm{},
		events:   make(chan Event, 256),
	}
	for _, dir := range []string{
		filepath.Join(cfg.RunDir, "seats"),
		filepath.Join(cfg.RunDir, "logs"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			d.Close()
			return nil, err
		}
	}
	// vault.key 存在才载（secret put 之前可能还没有）。
	if k, err := os.ReadFile(filepath.Join(cfg.RunDir, "vault.key")); err == nil {
		g.vaultKey = k
	}
	if err := os.MkdirAll(filepath.Dir(cfg.SocketPath), 0o700); err != nil {
		d.Close()
		return nil, err
	}
	_ = os.Remove(cfg.SocketPath) // 重启残留
	ln, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		d.Close()
		return nil, fmt.Errorf("listen %s: %w", cfg.SocketPath, err)
	}
	_ = os.Chmod(cfg.SocketPath, 0o600)
	g.ln = ln
	g.srv = grpc.NewServer()
	pbv1.RegisterSeatGatewayServer(g.srv, g)

	g.Reconcile(context.Background()) // §5.3 全相位对账，启动一次非周期
	return g, nil
}

// Serve 开始接受连接（调用方 go 起或阻塞用）。
func (g *Gateway) Serve() error { return g.srv.Serve(g.ln) }

// Close 停服：断流（席位侧收到 EOF 进入收养等待）、关库。
func (g *Gateway) Close() {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return
	}
	g.closed = true
	alarms := make([]*clock.Alarm, 0, len(g.alarms))
	for _, a := range g.alarms {
		alarms = append(alarms, a)
	}
	g.mu.Unlock()
	g.srv.Stop()
	_ = g.ln.Close()
	_ = os.Remove(g.cfg.SocketPath)
	for _, a := range alarms {
		a.Stop()
	}
	_ = g.database.Close()
}

// DB 暴露库句柄（CLI/测试用；写径仍经网关调用）。
func (g *Gateway) DB() *db.DB { return g.database }

// Events 进程内事件观察口（前端面/CEO 面在后续阶段消费同一事件源）。
func (g *Gateway) Events() <-chan Event { return g.events }

func (g *Gateway) emit(e Event) {
	select {
	case g.events <- e:
	default: // 观察口满不阻塞写径；持久日志在 ticket_event
	}
}

// armAlarm 给票挂/重挂进度闹钟；deadline 已过立即响（clock.Alarm 语义）。
func (g *Gateway) armAlarm(ticketID string, deadline time.Time) {
	g.mu.Lock()
	old := g.alarms[ticketID]
	g.mu.Unlock()
	if old != nil {
		old.ResetAt(deadline)
		return
	}
	a := clock.NewAlarmAt(deadline, func() { g.onStall(ticketID) })
	g.mu.Lock()
	g.alarms[ticketID] = a
	g.mu.Unlock()
}

// AlarmDeadline 取票的当前闹钟 deadline（审计面/测试）。
func (g *Gateway) AlarmDeadline(ticketID string) (time.Time, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	a, ok := g.alarms[ticketID]
	if !ok {
		return time.Time{}, false
	}
	return a.Deadline(), true
}

func (g *Gateway) cancelAlarm(ticketID string) {
	g.mu.Lock()
	a := g.alarms[ticketID]
	delete(g.alarms, ticketID)
	g.mu.Unlock()
	if a != nil {
		a.Stop()
	}
}

// onStall 进度闹钟响：不杀——发「疑似卡死」事件归 CTO 裁（§5.2）。
func (g *Gateway) onStall(ticketID string) {
	ctx := context.Background()
	_ = g.database.AppendEvent(ctx, ticketID, "stall_suspect", db.Event{
		Actor: "gateway", Note: "进度帧超时未至",
	})
	g.emit(Event{Kind: "stall_suspect", TicketID: ticketID})
}
