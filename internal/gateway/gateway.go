// Package gateway 是 meta 网关进程本体：席位面 Attach（RunAuth 首帧）、
// spawn＋per-seat UDS、SIGCHLD+EOF 双源判死、全相位启动对账、一次性闹钟。
// 网关无智能：只出事件、执机械闸；语义裁决归智能体（§3）。
package gateway

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"

	"meta/internal/clock"
	"meta/internal/db"
	pbv1 "meta/internal/pb/metav1"
	"meta/internal/route"
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

	// 预算/壳登记配置化（D-9/D-27，票0004）：出 config/gateway.textproto。
	DefaultAttemptCap int64            // 角色行未配时的尝试内帽（毛值 token）
	TicketHardCap     int64            // 票级累计硬顶缺省（毛值）
	RoleCaps          map[string]int64 // 角色 → 尝试内帽缺省
	ShellRows         []ShellSeed      // 壳登记表种子（角色行空才落，DB 行是活态）

	// AuditShellSkipStreak 审计重派壳偏置连败阈值（D-46/票0022）：同壳尾部
	// 连败 ≥N 从本轮 attempts 剔除；≤0=关。只偏置审计重派，不动健康语义。
	AuditShellSkipStreak int

	// 注入机（票0003，§4）：Root 空＝注入机关，dispatch 无 --prompt-file 即缺径。
	InjectRoot    string   // 注入源根：roles/ 与 docs/经验库|技能库 所在
	InjectRecipes []string // 项目配方/纪律文档（相对 InjectRoot，序即拼）

	// 立票点火去抖窗（票0039/T11）：票事件落账→重置对账闹钟，缺省 30s，
	// ≤0＝关。去抖非周期——静默期闹钟自灭，不违 R2。
	ReconcileDebounce time.Duration

	// 部署闸（票0010，§10/B1/B2/D1）。
	Projects       []*pbv1.ProjectSeed // 项目配方种子（票0046：钦定源——启动回种全列覆写，DB 行是文面投影）
	SudoBin        string              // 代跑 sudo 路径；缺省 "sudo"
	DockerBin      string              // 容器笼 docker 路径（票0029）；缺省 "docker"
	SeatImage      string              // T10 席位笼镜像（空＝旧径活）
	SeatNet        string              // T10 笼网
	SeatMountMask  []string            // T10 tmpfs 遮罩
	SeatExtraMount []string            // T10 额外挂载
	ExecTimeout    time.Duration       // 单步超时缺省（配方沙箱未声明时），缺省 15m

	// CEO 面（票0011）：事件泵放行/无头唤起 kind 列与化身提示模板（决策54/E16）。
	CeoEvents     map[string]bool // Events 流放行 kind 集
	CeoSpawnKinds map[string]bool // 需 CEO 事件 kind 集（无订阅→pending+ensureCEO）
	CeoPrompt     string          // 无头席提示模板（{{kind}}/{{ticket_id}}/{{note}}/{{run_dir}}/{{gateway_sock}}）
}

// ShellSeed 壳登记表种子一行（配置行→DB 活态行；attempt_cap 驻内存不持久化——
// 它是「该行尝试帽」的派发生命期参数，不是登记表状态）。
type ShellSeed struct {
	Name, Model, Role string
	Priority          int
	AttemptCap        int64
}

// deferCap 延期帽 K（§5.2 A5）：裁决 defer 可重挂闹钟的次数上限。
const deferCap = 2

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
	if c.SudoBin == "" {
		c.SudoBin = "sudo"
	}
	if c.DockerBin == "" {
		c.DockerBin = "docker"
	}
	if c.ExecTimeout <= 0 {
		c.ExecTimeout = 15 * time.Minute
	}
	if c.SelfPath == "" {
		if p, err := os.Executable(); err == nil {
			c.SelfPath = p
		}
	}
	if c.ReconcileDebounce <= 0 {
		c.ReconcileDebounce = 30 * time.Second
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
	failReported bool // 失效已报账（reportShell 原子置位——同一死双径只记一次）

	sendMu sync.Mutex                    // 座流发送互斥（ack/exec_resp/feed 并发安全）
	stream pbv1.SeatGateway_AttachServer // 票0030：存活 attach 流句柄（feed 直投径）
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

	mu           sync.Mutex
	runs         map[string]*runState
	alarms       map[string]*clock.Alarm
	events       chan Event
	observers    map[chan Event]struct{}    // Observe 面订户（管理面导出）
	specs        map[specKey]SeatSpec       // (tid,role) → 派发参数（内存；重启缺→spawn_blocked）
	failSet      map[string]map[string]bool // tid → 本票已失败壳行键（换壳＝排除）
	spreadCnt    map[string]int             // T20：角色→轮转计数（健康壳均摊位）
	warned       map[string]bool            // tid → 本尝试 80% 警已发（边沿触发）
	frozen       map[string]bool            // budget_frozen 票集（冻票无活性）
	active       map[string]int             // 壳名 → 活跃 run 数（provider 并发闸）
	attemptCaps  map[string]int64           // ShellKey → 该行尝试帽覆盖（§7）
	projectLocks map[string]map[string]bool // 项目部署锁：project → {integrate|deploy}（B1；设计锁＝designLocks）
	spawnQ       chan spawnReq
	done         chan struct{}
	closed       bool

	// T26 派发/部署径收口件：
	dispatchMu      sync.Mutex             // DispatchSeat 闸段互斥——去重判/相位读/锁取占同临界区（病1）
	degradedRetried map[string]bool        // tid → failSet 已回落重试过（病5 降级出路有界）
	stallFalls      map[string]int         // tid → 僵尸席处决计数（病7：≥3 止杀只报）
	cloneLocks      map[string]*sync.Mutex // tid → 部署克隆互斥（病3 同票级）

	// CEO 面（票0011）：锁＝连接登记非 DB 旗（禁区#10/B8）。
	ceoSub        chan Event                                                    // 活跃 Events 订阅的派发道；nil＝无订阅
	ceoSubRunID   string                                                        // 当前订阅属哪条 run（D-22 计数分账）
	ceoPending    []Event                                                       // 无订阅期排队的需 CEO 事件（切换不丢，§8）
	ceoFails      int                                                           // 无头唤起连败计数（3→ceo_last_resort）
	ceoDispatched map[string]int                                                // run_id → 已派发给该 run 的事件数（D-22）
	designLocks   map[string]string                                             // project → 持设计锁的票（B1 三锁之一，§1.8）
	budgetOf      func(ctx context.Context, ticketID string) (db.Budget, error) // D-36 变异注入缝

	// 票0039/T11：立票点火去抖闹钟——票事件落账（db eventHook）重置，
	// 醒→goroutine 跑既有 reconcile。reconBusy＝对账在飞期抑制自锁环
	// （对账自产事件不再续命闹钟——否则滞留票 spawn_needed 每窗重放成
	// 变相周期轮询，违 R2）；reconDirty＝busy 期非静默事件脏标（判决
	// 返修：不丢不续命——对账收尾查脏再醒一轮，竞态窗闭合）。
	reconTimer   *time.Timer
	reconBusy    bool
	reconDirty   bool
	reconcileNow func(ctx context.Context) // 测试注入缝；缺省 g.reconcile
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
		cfg:          cfg,
		database:     d,
		runs:         map[string]*runState{},
		alarms:       map[string]*clock.Alarm{},
		events:       make(chan Event, 256),
		observers:    map[chan Event]struct{}{},
		specs:        map[specKey]SeatSpec{},
		failSet:      map[string]map[string]bool{},
		spreadCnt:    map[string]int{},
		warned:       map[string]bool{},
		frozen:       map[string]bool{},
		active:       map[string]int{},
		projectLocks: map[string]map[string]bool{},
		spawnQ:       make(chan spawnReq, 64),
		done:         make(chan struct{}),

		degradedRetried: map[string]bool{},
		stallFalls:      map[string]int{},
		cloneLocks:      map[string]*sync.Mutex{},

		ceoDispatched: map[string]int{},
		designLocks:   map[string]string{},
	}
	for _, dir := range []string{
		filepath.Join(cfg.RunDir, "seats"),
		filepath.Join(cfg.RunDir, "logs"),
		filepath.Join(cfg.RunDir, "exec"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			d.Close()
			return nil, err
		}
	}
	// 代跑干净 clone 父目录：0755——run_as 降权用户要能遍历/建 clone 子目录（D1）。
	if err := os.MkdirAll(filepath.Join(cfg.RunDir, "deploy"), 0o755); err != nil {
		d.Close()
		return nil, err
	}
	// 项目配方回种（票0046 真源归一）：config project{} 是钦定源，DB project
	// 行是它的启动投影——每次 New() 以文面全列覆写，「行在即不动」语义废止。
	// 漂移字段落 project_recipe_sync 事件（ticket_id 空，同 ceo_say 空票径）
	// ＋stderr 留痕；零漂/初种不产事件（防每启噪音）。回种在 socket listen
	// 之前——exec 步永远只见回种后行；cfg.Projects 空＝不清既有行。
	for _, ps := range cfg.Projects {
		if ps == nil || ps.Id == "" {
			continue
		}
		sb := ""
		if ps.Sandbox != nil {
			if b, err := prototext.Marshal(ps.Sandbox); err == nil {
				sb = string(b)
			}
		}
		drift, err := d.ReseedProject(context.Background(), db.Project{
			ID: ps.Id, RepoURL: ps.RepoUrl, DefaultBranch: ps.DefaultBranch,
			IntegrateCmd: ps.IntegrateCmd, CheckCmd: ps.CheckCmd,
			DeployCmd: ps.DeployCmd, ProbeCmd: ps.ProbeCmd,
			RollbackCmd: ps.RollbackCmd, SandboxSpec: sb,
		})
		if err != nil {
			d.Close()
			return nil, fmt.Errorf("项目回种 %s: %w", ps.Id, err)
		}
		if len(drift) > 0 {
			note := fmt.Sprintf("项目 %s 配方漂移字段: %s", ps.Id, strings.Join(drift, ","))
			_ = d.AppendEvent(context.Background(), "", "project_recipe_sync", db.Event{
				Actor: "gateway", Note: note})
			g.emit(Event{Kind: "project_recipe_sync", Note: note})
			fmt.Fprintf(os.Stderr, "gateway: project_recipe_sync %s\n", note)
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
	// T34 §7.2：socket 0660＋属组 metaseat——交互 CEO（mluser 在组内）走正规
	// gRPC，废 sudo-sqlite 明文轮询径。组解析失败降级保 0660（测试/开发机
	// 无 metaseat 组不碍起）。
	_ = os.Chmod(cfg.SocketPath, 0o660)
	if grp, err := user.LookupGroup("metaseat"); err == nil {
		if gid, err := strconv.Atoi(grp.Gid); err == nil {
			_ = os.Chown(cfg.SocketPath, -1, gid)
		}
	}
	g.ln = ln
	g.srv = grpc.NewServer()
	pbv1.RegisterSeatGatewayServer(g.srv, g)
	pbv1.RegisterAdminGatewayServer(g.srv, g)
	pbv1.RegisterCeoGatewayServer(g.srv, &ceoAPI{g: g})
	g.budgetOf = d.GetBudget

	// 壳行尝试帽索引：ShellKey→cap（§7 attempt_cap>0 行覆盖角色缺省的落点）。
	g.attemptCaps = map[string]int64{}
	for _, s := range cfg.ShellRows {
		if s.AttemptCap > 0 {
			g.attemptCaps[db.Shell{Name: s.Name, Model: s.Model, Role: s.Role}.ShellKey()] = s.AttemptCap
		}
	}

	g.reconcile(context.Background())           // §5.3 全相位对账，启动一次非周期
	g.rebuildProjectLocks(context.Background()) // 项目锁按票相位重建（B1）
	go g.dispatcher()                           // spawn 调度器：事件驱动，非周期
	go g.ceoPump(g.subscribe())                 // CEO 事件泵：先订阅后起泵，不丢首事件
	// 票0039/T11：点火去抖——DB 事件钩接 armRecon；reconcileNow 注入缝。
	g.reconcileNow = g.reconcile
	d.SetEventHook(g.armRecon)
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
	g.mu.Lock()
	if g.reconTimer != nil {
		g.reconTimer.Stop()
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
	// 事件字段净化（决策 67 D-45③）：emit 是进程内事件全出口——壳原文
	// 尾段/毒 note 经此进 events 道与各订户流，带毒即 marshal 崩订户。
	e.Kind, e.TicketID, e.RunID, e.Role, e.Note, e.Provenance =
		clean(e.Kind), clean(e.TicketID), clean(e.RunID), clean(e.Role),
		clean(e.Note), clean(e.Provenance)
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

// reconSilentKinds 机器记账类事件（票0039）：对账/派发/巡检泵自身产出——
// 不武装闹钟。缺它们则滞留票走「醒→spawn_needed→dispatcher 异步落
// shells_exhausted→再武装→再醒」＝变相周期轮询，违 R2 零心跳。
// created/phase/msg/外部 inject 等真事件照常武装（点火本意）。
var reconSilentKinds = map[string]bool{
	"spawn_needed": true, "spawn_blocked": true, "shells_exhausted": true,
	"inject_failed": true, "dispatch_failed": true, "patrol_tick": true,
	"patrol_drop": true, "patrol_skip": true, "filter_degraded": true,
	"seat_err": true, "adjunct_dead": true, "progress": true,
	"adopted": true, // T17③：对账重收养不续命对账闹钟（D-82 自锁环）
	"usage":   true, "msg_delivered": true, "stall_suspect": true,
}

// armRecon 立票点火去抖闹钟（票0039/T11，R2「随事件重置的一次性闹钟」
// 钦定形态）：票事件落账（db eventHook，机器记账 kind 滤除）重置——
// 醒→goroutine 跑既有 reconcile（判别逻辑不动只加触发）。对账在飞期
// 到达的事件置 reconDirty（不续命＝不变相轮询，不丢＝竞态窗闭合）；
// 收尾查脏再醒一轮。窗＝cfg.ReconcileDebounce，≤0 关。
func (g *Gateway) armRecon(kind string) {
	if reconSilentKinds[kind] {
		return
	}
	span := g.cfg.ReconcileDebounce
	if span <= 0 {
		return
	}
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return
	}
	if g.reconBusy {
		// 判决返修：busy 期事件置脏标不丢不续命——对账收尾查脏再醒
		// 一轮（抗「先扫后落」竞态窗：重启风暴期立票掉缝=偶现点火洞）。
		g.reconDirty = true
		g.mu.Unlock()
		return
	}
	if g.reconTimer != nil {
		g.reconTimer.Reset(span)
		g.mu.Unlock()
		return
	}
	g.reconTimer = time.AfterFunc(span, func() {
		g.mu.Lock()
		g.reconBusy = true
		g.reconTimer = nil
		g.reconDirty = false
		g.mu.Unlock()
		g.reconcileNow(context.Background())
		g.mu.Lock()
		g.reconBusy = false
		dirty := g.reconDirty
		g.reconDirty = false
		g.mu.Unlock()
		if dirty {
			g.armRecon("created") // 收尾查脏：busy 期真事件→再醒一轮
		}
	})
	g.mu.Unlock()
}

// batchDeployCheck 批量部署计数闸（T4/E17）：待部署≥5 或流水线清空
// （非终态票中无 施工中/设计中）→ 逐张补 spawn_needed 到 CTO 部署支
// ——事件驱动一次性，非周期扫描。
func (g *Gateway) batchDeployCheck() {
	ctx := context.Background()
	ts, err := g.database.NonTerminalTickets(ctx)
	if err != nil {
		return
	}
	var pending []*db.Ticket
	pipelineBusy := false
	for _, t := range ts {
		switch t.Phase {
		case db.PhaseToDeploy:
			pending = append(pending, t)
		case db.PhaseWorking, db.PhaseDesigning:
			pipelineBusy = true
		}
	}
	if len(pending) < 5 && (pipelineBusy || len(pending) == 0) {
		return
	}
	_ = g.database.AppendEvent(ctx, "", "batch_deploy", db.Event{
		Actor: "gateway",
		Note:  fmt.Sprintf("批量部署触发：待部署=%d 流水线忙=%v", len(pending), pipelineBusy)})
	g.emit(Event{Kind: "batch_deploy", Note: fmt.Sprintf("待部署 %d 张批量触发", len(pending))})
	for _, t := range pending {
		_ = g.database.AppendEvent(ctx, t.ID, "spawn_needed", db.Event{
			Actor: "gateway", Note: "对账补发: CTO-B 部署支（批部署）"})
		g.emit(Event{Kind: "spawn_needed", TicketID: t.ID, Role: "CTO"})
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

// onStall 进度闹钟响：发「疑似卡死」事件归 CTO 裁（§5.2）。
// note 带 run=<最新活 non-adjunct runid>：路由表把行喂给裁决席提示模板（§5 时序1）。
// T26 病7+病6：部署票（CTO 部署支）僵尸席＝CliDrive 钦定兜底径——处决
// 挂席（四步已齐先落 deploy_done），死判径（onRunDead）放锁＋spawn_needed
// 自动转 CliDrive 重派；同票处决 2 次后止杀只报（防处决循环烧票）。
// 对抗锚提名行：CTO 部署票处决分支——删则僵尸席只报不死、部署停摆。
func (g *Gateway) onStall(ticketID string) {
	ctx := context.Background()
	note := "进度帧超时未至"
	var run *db.Run
	if r, _ := g.database.ActiveRunForTicket(ctx, ticketID); r != nil {
		run = r
		note += " 席=" + run.RunID // T3 去 run= 术语（裁决席同径读行）
	}
	_ = g.database.AppendEvent(ctx, ticketID, "stall_suspect", db.Event{
		Actor: "gateway", Note: note,
	})
	runID := ""
	if run != nil {
		runID = run.RunID
	}
	g.emit(Event{Kind: "stall_suspect", TicketID: ticketID, RunID: runID})

	if run == nil || run.Role != "CTO" || run.Adjunct {
		return // 非部署席照旧只报不杀，归 CTO 裁
	}
	phase, err := g.database.PhaseOf(ctx, ticketID)
	if err != nil || (phase != db.PhaseToDeploy && phase != db.PhaseDeploying) {
		return
	}
	g.mu.Lock()
	g.stallFalls[ticketID]++
	falls := g.stallFalls[ticketID]
	g.mu.Unlock()
	if falls > 2 {
		return // 处决 2 次仍僵＝系统性故障——只报归裁，不再循环杀
	}
	// 四步已齐＝部署实成在前、席僵在后——直接落 deploy_done（病4 事件驱动
	// 收口同径），不冤杀已成功部署。
	if phase == db.PhaseDeploying && g.deployStepsDone(ctx, ticketID) {
		if g.deployDone(ctx, ticketID, "停滞钟响：四步已齐——席僵不挡收口") {
			return
		}
	}
	if phase == db.PhaseDeploying {
		// 先落 deploy_failed 回 待部署（锁随 transition 释），再杀——
		// 死判径相位已回 待部署 即走「claim 前死」支补 spawn_needed。
		if err := g.transition(ctx, ticketID, db.PhaseToDeploy, db.EdgeDeployFailed,
			"gateway", "停滞处决：部署席挂起→CliDrive 钦定兜底（先核回滚态 run="+run.RunID+")"); err != nil {
			return
		}
	}
	note2 := "停滞处决：僵尸部署席→CliDrive 钦定兜底"
	_ = g.database.AppendEvent(ctx, ticketID, "stall_kill", db.Event{
		Actor: "gateway", Note: note2 + " run=" + run.RunID})
	g.emit(Event{Kind: "stall_kill", TicketID: ticketID, RunID: run.RunID, Note: note2})
	st := g.bindRun(run)
	st.noRespawn = false // 处决非禁重派——onRunDead 补发径负责拉起 CliDrive
	g.killRun(st)
}

// transition 全相位迁移收口（票0004 §6）：迁移成功后发 phase:<edge> 门铃
// （补 dispatch 边一直缺的 poke）；EdgeReject 达 §6.2 三驳→reject_limit
// 事件行＋emit。先落行后门铃——门铃先于落行的窗口会被下一 poke 补回，
// 但严格因果更省一次迟延。
func (g *Gateway) transition(ctx context.Context, ticketID string, to db.Phase, kind db.EdgeKind, actor, note string) error {
	var from db.Phase
	var project string
	found := false
	if t, err := g.database.GetTicket(ctx, ticketID); err == nil && t != nil {
		from, project, found = t.Phase, t.Project, true
	}
	if _, err := g.database.Transition(ctx, ticketID, to, kind, actor, note); err != nil {
		return err
	}
	g.emit(Event{Kind: "phase:" + string(kind), TicketID: ticketID})
	// T4 批量部署开关（E17）：入 待部署 后计数——≥5 张或流水线清空
	// （无 施工中/设计中）→ 逐张补发 spawn_needed 触发部署派发
	// （B1 串行锁照走闸——计数从 ticket 表读零轮询）。
	if to == db.PhaseToDeploy {
		go g.batchDeployCheck()
	}
	// 项目部署锁随相位占用释放（B1）：出离 部署中 释 integrate+deploy。
	// deploy_failed 的本票不补 spawn——回 待部署 归 CEO 裁。
	if from == db.PhaseDeploying {
		skip := ""
		if kind == db.EdgeDeployFailed {
			skip = ticketID
		}
		g.releaseProjectLocks(ctx, project, skip, true, lockIntegrate, lockDeploy)
	}
	// B1 设计锁（designLocks）释放点＝持锁票离开施工中（交付/泊车/回队）
	// 或入终态——releaseDesignLock 内 holder 核对使非持锁票过此为无操作。
	if found && (from == db.PhaseWorking || to == db.PhaseDone || to == db.PhaseVoid) {
		g.releaseDesignLock(ctx, ticketID)
	}
	if to == db.PhaseDone || to == db.PhaseVoid {
		g.onTerminal(ticketID) // D-15：终态即删进程级内存键
	}
	if kind == db.EdgeReject {
		if t, err := g.database.GetTicket(ctx, ticketID); err == nil && t.RejectCount >= 3 {
			_ = g.database.AppendEvent(ctx, ticketID, "reject_limit", db.Event{
				Actor: "gateway", Note: "驳回达 3，等你发落",
			})
			g.emit(Event{Kind: "reject_limit", TicketID: ticketID})
		}
	}
	return nil
}
