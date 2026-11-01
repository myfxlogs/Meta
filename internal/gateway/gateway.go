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
	"meta/internal/route"
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
	SelfPath         string        // `meta seat` 外壳二进制；缺省 os.Executable
	SeatArgv         []string      // 席位外壳 argv 全替（缺省 [SelfPath,"seat"]；测试注入 helper）
	Stagger          time.Duration // ②a 限流错峰时延，缺省 10s
	QuickExit        time.Duration // ①秒退判定窗口（attach 或死于此窗口内），缺省 20s
	MaxPerShell      int           // provider 并发闸：同名壳并行席位上限，0=不限

	// 注入机（票0003，§4）：Root 空＝注入机关，dispatch 无 --prompt-file 即缺径。
	InjectRoot    string   // 注入源根：roles/ 与 docs/经验库|技能库 所在
	InjectRecipes []string // 项目配方/纪律文档（相对 InjectRoot，序即拼）
}

func (c *Config) withDefaults() {
	// D-23 类坑反向：路径全量绝对化——席位 env（socket/注入文件）跨 cwd 引用。
	for _, p := range []*string{&c.DBPath, &c.RunDir, &c.SocketPath} {
		if abs, err := filepath.Abs(*p); err == nil {
			*p = abs
		}
	}
	if c.ProgressTimeout <= 0 {
		c.ProgressTimeout = 30 * time.Minute
	}
	if c.AuthTimeout <= 0 {
		c.AuthTimeout = 5 * time.Second
	}
	if c.AdoptDialTimeout <= 0 {
		c.AdoptDialTimeout = 2 * time.Second
	}
	if c.Stagger <= 0 {
		c.Stagger = 10 * time.Second
	}
	if c.QuickExit <= 0 {
		c.QuickExit = 20 * time.Second
	}
	if c.SelfPath == "" {
		if p, err := os.Executable(); err == nil {
			c.SelfPath = p
		}
	}
}

// runState 一张 seat_run 的运行时镜像＋双源判死所需的两条事件通道。
type runState struct {
	run *db.Run
	cmd *os.Process // spawn 径才有；收养径无柄

	attempt   route.Attempt // 壳路由径：本次尝试的登记行（helper 径为零值）
	killByGW  bool          // 网关令杀（预算硬停/冻结）——非壳之错，不罚健康
	noRespawn bool          // 判死后不自动重派（预算硬停摆位等 CTO 裁）

	attachedCh chan struct{} // attach 成功即闭（①秒退窗口的等待源）
	exitCh     chan struct{} // Wait 返回即闭

	mu           sync.Mutex
	attached     bool
	streamClosed bool
	exited       bool
	exitCode     int
	dead         bool
	adopted      bool
}

func newRunState(run *db.Run) *runState {
	return &runState{
		run:        run,
		attachedCh: make(chan struct{}),
		exitCh:     make(chan struct{}),
	}
}

// Gateway 网关。
type Gateway struct {
	pbv1.UnimplementedSeatGatewayServer
	pbv1.UnimplementedAdminGatewayServer

	cfg      Config
	database *db.DB
	srv      *grpc.Server
	ln       net.Listener
	vaultKey []byte // secret 表主密钥；缺载时 Secret RPC 全拒

	mu        sync.Mutex
	runs      map[string]*runState
	alarms    map[string]*clock.Alarm
	events    chan Event
	observers map[chan Event]struct{} // Observe 面订户（管理面导出）
	specs     map[string]SeatSpec     // tid → 派发参数（内存；重启缺→spawn_blocked）
	failSet   map[string]map[string]bool // tid → 本票已失败壳行键（换壳＝排除）
	warned    map[string]bool         // tid → 本尝试 80% 警已发（边沿触发）
	frozen    map[string]bool         // budget_frozen 票集（冻票无活性）
	active    map[string]int          // 壳名 → 活跃 run 数（provider 并发闸）
	spawnQ    chan spawnReq
	done      chan struct{}
	closed    bool
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
		cfg:       cfg,
		database:  d,
		runs:      map[string]*runState{},
		alarms:    map[string]*clock.Alarm{},
		events:    make(chan Event, 256),
		observers: map[chan Event]struct{}{},
		specs:     map[string]SeatSpec{},
		failSet:   map[string]map[string]bool{},
		warned:    map[string]bool{},
		frozen:    map[string]bool{},
		active:    map[string]int{},
		spawnQ:    make(chan spawnReq, 64),
		done:      make(chan struct{}),
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
	pbv1.RegisterAdminGatewayServer(g.srv, g)

	g.Reconcile(context.Background()) // §5.3 全相位对账，启动一次非周期
	go g.dispatcher()                  // spawn 调度器：事件驱动，非周期
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
	close(g.done)
	for _, a := range alarms {
		a.Stop()
	}
	_ = g.database.Close()
}

// DB 暴露库句柄（CLI/测试用；写径仍经网关调用）。
func (g *Gateway) DB() *db.DB { return g.database }

// EventCh 进程内事件观察口（前端面/CEO 面在后续阶段消费同一事件源）。
func (g *Gateway) EventCh() <-chan Event { return g.events }

func (g *Gateway) emit(e Event) {
	g.mu.Lock()
	frozen := g.frozen[e.TicketID]
	obs := make([]chan Event, 0, len(g.observers))
	for ch := range g.observers {
		obs = append(obs, ch)
	}
	g.mu.Unlock()
	// 冻票无活性：冻结即取消排队 spawn 事件（§5.5 D8），其余事件照发。
	if frozen && e.Kind == "spawn_needed" {
		return
	}
	select {
	case g.events <- e:
	default: // 观察口满不阻塞写径；持久日志在 ticket_event
	}
	for _, ch := range obs {
		select {
		case ch <- e:
		default:
		}
	}
}

// subscribe/unsubscribe Observe 面订户口。
func (g *Gateway) subscribe() chan Event {
	ch := make(chan Event, 256)
	g.mu.Lock()
	g.observers[ch] = struct{}{}
	g.mu.Unlock()
	return ch
}

func (g *Gateway) unsubscribe(ch chan Event) {
	g.mu.Lock()
	delete(g.observers, ch)
	g.mu.Unlock()
}

// armAlarm 给票挂/重挂进度闹钟；deadline 已过立即响（clock.Alarm 语义）。
// 查表与建表在同一临界区——修决策10遗留③两锁间窄竞（并发双建会漏一只仍响的闹钟）。
func (g *Gateway) armAlarm(ticketID string, deadline time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if old := g.alarms[ticketID]; old != nil {
		old.ResetAt(deadline)
		return
	}
	g.alarms[ticketID] = clock.NewAlarmAt(deadline, func() { g.onStall(ticketID) })
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
