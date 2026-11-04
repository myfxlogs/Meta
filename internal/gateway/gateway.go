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

	// 幂等调和器周期（v2 §3.3-A，R2′ 钦定唯一周期例外）：每 60s 跑一轮
	// reconcile()；任意事件后另经 kick 道立即跑。≤0＝关周期只留事件驱动。
	// 复用 reconcile_debounce 配置位（字段名留，语义已改）。
	ReconcileDebounce time.Duration

	// 失败表（v2 §3.3-B）：kind→确定性处置＋期窗计数上限，超限→blocked。
	FailureRules []FailureRule

	// Mandate 策略（v2 §3.3-C）：现行版本＋已撤版本集——进 施工中/部署中
	// 边前核票行授权书仍有效。
	PolicyVersion   string
	RevokedVersions []string

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
		c.ReconcileDebounce = 60 * time.Second
	}
	if len(c.FailureRules) == 0 {
		c.FailureRules = defaultFailureRules()
	}
}

// runState 一张 seat_run 的运行时镜像＋双源判死所需的两条事件通道。
type runState struct {
	run *db.Run
	cmd *os.Process // spawn 径才有；收养径无柄

	attempt   route.Attempt // 壳路由径：本次尝试的登记行（helper 径为零值）
	killByGW  bool          // 网关令杀（预算硬停/冻结）——非壳之错，不罚健康
	killCause string        // T47 死因帧：谁令杀（stall_kill/budget_hard/freeze/ruling/reap）——猝死零输出之谜定位件
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
	spreadCnt    map[string]int             // T20：角色→轮转计数（健康壳均摊位）
	active       map[string]int             // 壳名 → 活跃 run 数（provider 并发闸）
	runWorkDir   map[string]string          // runid → 席位工作区（收尸清洁索引，重启缺＝跳过安全）
	attemptCaps  map[string]int64           // ShellKey → 该行尝试帽覆盖（§7）
	projectLocks map[string]map[string]bool // 项目部署锁：project → {integrate|deploy}（B1；设计锁＝designLocks）
	spawnQ       chan spawnReq
	done         chan struct{}
	closed       bool

	// T26 派发/部署径收口件：
	dispatchMu      sync.Mutex             // DispatchSeat 闸段互斥——去重判/相位读/锁取占同临界区（病1）
	cloneLocks      map[string]*sync.Mutex // tid → 部署克隆互斥（病3 同票级）

	// CEO 面（票0011）：锁＝连接登记非 DB 旗（禁区#10/B8）。
	ceoSub        chan Event                                                    // 活跃 Events 订阅的派发道；nil＝无订阅
	ceoSubRunID   string                                                        // 当前订阅属哪条 run（D-22 计数分账）
	ceoPending    []Event                                                       // 无订阅期排队的需 CEO 事件（切换不丢，§8）
	ceoFails      int                                                           // 无头唤起连败计数（3→ceo_last_resort）
	ceoDispatched map[string]int                                                // run_id → 已派发给该 run 的事件数（D-22）
	designLocks   map[string]string                                             // project → 持设计锁的票（B1 三锁之一，§1.8）
	budgetOf      func(ctx context.Context, ticketID string) (db.Budget, error) // D-36 变异注入缝

	// T35 幂等调和器驱动：事件钩→非阻塞 kick；60s ticker＋kick 双源
	// 串行跑 reconcile（同一 goroutine 天然去抖——缓冲 1 多踢自并）。
	reconKick    chan struct{}
	reconMu      sync.Mutex // T48 返修：直调径与 kick 环并发互斥——整轮串行
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
		spreadCnt:    map[string]int{},
		active:       map[string]int{},
		runWorkDir:   map[string]string{},
		projectLocks: map[string]map[string]bool{},
		spawnQ:       make(chan spawnReq, 64),
		done:         make(chan struct{}),

		cloneLocks:      map[string]*sync.Mutex{},
		reconKick:       make(chan struct{}, 1),

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
			AutoTicketMode: ps.AutoTicketMode,
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

	g.reconcile(context.Background())           // 启动一轮对齐（V4：kill -9 后 60s 内收敛）
	g.rebuildProjectLocks(context.Background()) // 项目锁按票相位重建（B1）
	go g.dispatcher()                           // spawn 调度器：事件驱动，非周期
	go g.ceoPump(g.subscribe())                 // CEO 事件泵：先订阅后起泵，不丢首事件
	// T35：幂等调和器——事件钩 kick＋周期 ticker 双源驱动。
	g.reconcileNow = g.reconcile
	d.SetEventHook(g.kickRecon)
	go g.reconcileLoop()
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
	// 事件字段净化（决策 67 D-45③）：emit 是进程内事件全出口——壳原文
	// 尾段/毒 note 经此进 events 道与各订户流，带毒即 marshal 崩订户。
	e.Kind, e.TicketID, e.RunID, e.Role, e.Note, e.Provenance =
		clean(e.Kind), clean(e.TicketID), clean(e.RunID), clean(e.Role),
		clean(e.Note), clean(e.Provenance)
	g.mu.Lock()
	obs := make([]chan Event, 0, len(g.observers))
	for ch := range g.observers {
		obs = append(obs, ch)
	}
	g.mu.Unlock()
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

// reconSilentKinds 机器记账类事件：调和器/派发自身产出——不踢调和
// （自踢自转＝变相周期轮询放大）。created/phase/msg/外部 inject 等真
// 事件照踢（「任意事件后立即跑一轮」钦定语义）。
var reconSilentKinds = map[string]bool{
	"spawn_blocked": true, "shells_exhausted": true,
	"inject_failed": true, "dispatch_failed": true, "patrol_tick": true,
	"patrol_drop": true, "patrol_skip": true, "filter_degraded": true,
	"seat_err": true, "adjunct_dead": true, "progress": true,
	"adopted": true, "adopt_failed": true,
	"usage": true, "msg_delivered": true, "stall_suspect": true,
	"blocked": true, "reconcile": true, "budget_frozen": true,
	"inject_truncated": true,
}

// kickRecon 事件驱动调和钩（db eventHook）：非静默 kind → 非阻塞 kick。
func (g *Gateway) kickRecon(kind string) {
	if reconSilentKinds[kind] {
		return
	}
	select {
	case g.reconKick <- struct{}{}:
	default: // 已有 kick 排队＝下一轮必跑，多踢自并
	}
}

// reconcileLoop 调和器驱动环（v2 §3.3-A，R2′ 钦定唯一周期例外）：
// 60s ticker＋事件 kick 双源，同一 goroutine 串行跑 reconcile——
// 幂等纯查表零外发，跑 N 次与 1 次结果相同。
func (g *Gateway) reconcileLoop() {
	t := time.NewTicker(g.cfg.ReconcileDebounce)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			g.reconcileNow(context.Background())
		case <-g.reconKick:
			g.reconcileNow(context.Background())
		case <-g.done:
			return
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

// onStall 进度闹钟响（失败表 kill_respawn 行）：发 stall_suspect 账＋
// 杀挂席——相位不动，调和器按期窗 stall 计数 ≤2 换壳重派、超限 blocked。
// 部署中先查收口（四步已齐落 deploy_done，不冤杀已成部署）。
func (g *Gateway) onStall(ticketID string) {
	ctx := context.Background()
	note := "进度帧超时未至"
	var run *db.Run
	if r, _ := g.database.ActiveRunForTicket(ctx, ticketID); r != nil {
		run = r
		note += " 席=" + run.RunID
	}
	_ = g.database.AppendEvent(ctx, ticketID, "stall_suspect", db.Event{
		Actor: "gateway", Note: note})
	runID := ""
	if run != nil {
		runID = run.RunID
	}
	g.emit(Event{Kind: "stall_suspect", TicketID: ticketID, RunID: runID})
	if run == nil || run.Adjunct {
		return // 无行动者/协从席只报——调和器补位
	}
	if phase, err := g.database.PhaseOf(ctx, ticketID); err == nil &&
		phase == db.PhaseDeploying && g.deployStepsDone(ctx, ticketID) {
		if g.deployDone(ctx, ticketID, "停滞钟响：四步已齐——席僵不挡收口") {
			return
		}
	}
	note2 := "停滞处决：进度帧超时杀席 run=" + run.RunID
	_ = g.database.AppendEvent(ctx, ticketID, "stall_kill", db.Event{
		Actor: "gateway", Note: note2})
	g.emit(Event{Kind: "stall_kill", TicketID: ticketID, RunID: run.RunID, Note: note2})
	st := g.bindRun(run)
	st.killByGW = true // 网关令杀＝非壳之错，不罚健康
	st.killCause = "stall_kill"
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
	// 项目部署锁随相位占用释放（B1）：出离 部署中 释 integrate+deploy。
	// T35：不再就地补发——锁释后滞留票由调和器拾。
	if from == db.PhaseDeploying ||
		(from == db.PhaseToDeploy && to == db.PhaseBlocked) {
		g.releaseProjectLocks(ctx, project, "", true, lockIntegrate, lockDeploy)
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

// defaultFailureRules 失败表缺省（v2 §3.3-B——config/gateway.textproto
// 的 failure_rule 段是全量覆盖径；缺省与钦定行同值，改表走票）。
func defaultFailureRules() []FailureRule {
	return []FailureRule{
		{Kind: "seat_err", Action: "next_shell", Cap: 0},
		{Kind: "seat_dead", Action: "next_shell", Cap: 0},
		{Kind: "spawn_failed", Action: "next_shell", Cap: 0},
		{Kind: "shell_limited", Action: "retry_same", Cap: 3},
		{Kind: "shell_dead", Action: "shell_dead_next", Cap: 0},
		{Kind: "stall_suspect", Action: "kill_respawn", Cap: 2},
		{Kind: "stall_kill", Action: "kill_respawn", Cap: 2},
		{Kind: "clidrive_dead", Action: "next_shell", Cap: 0},
		{Kind: "reject", Action: "same_shell", Cap: 3},
		{Kind: "deploy_failed", Action: "rollback", Cap: 1},
		{Kind: "budget_frozen", Action: "freeze", Cap: 0},
		{Kind: "*", Action: "blocked", Cap: 0},
	}
}
