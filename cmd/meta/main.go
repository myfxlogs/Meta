// meta 是网关单二进制（§3）：serve 起进程；secret put 是最小写径；
// seat 是真壳席位外壳（网关 spawn 的内部入口，非人手件）；
// ticket/dispatch/phase/events/runs/budget/observe 是管理面 CLI（经 unix socket
// 走 AdminGateway——不直连 DB，禁区#6）。
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"meta/internal/caster"
	"meta/internal/config"
	"meta/internal/db"
	"meta/internal/gateway"
	"meta/internal/inject"
	pbv1 "meta/internal/pb/metav1"
	"meta/internal/seat"
	"meta/internal/supervise"
	"meta/internal/upgrade"
)

// buildStamp 构建戳（-ldflags -X main.buildStamp=...）；version/升级预检用。
var buildStamp = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	if os.Args[1] == "seat" {
		os.Exit(seat.Run()) // 席位外壳：全部参数经 META_* env
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(os.Args[2:])
	case "secret":
		err = secret(os.Args[2:])
	case "ticket":
		err = ticketCmd(os.Args[2:])
	case "dispatch":
		err = dispatchCmd(os.Args[2:])
	case "phase":
		err = phaseCmd(os.Args[2:])
	case "events":
		err = eventsCmd(os.Args[2:])
	case "runs":
		err = runsCmd(os.Args[2:])
	case "budget":
		err = budgetCmd(os.Args[2:])
	case "observe":
		err = observeCmd(os.Args[2:])
	case "caster":
		err = casterCmd(os.Args[2:])
	case "retro":
		err = retroCmd(os.Args[2:])
	case "supervise":
		err = superviseCmd(os.Args[2:])
	case "upgrade":
		err = upgradeCmd(os.Args[2:])
	case "version":
		fmt.Println("meta", buildStamp)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "meta:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `用法:
  meta serve  --db run/meta.db --socket run/meta.sock --rundir run
              [--inject-root <repo> --inject-recipe <f[,f2]>]   (注入机：无 --prompt-file 时机械组装席位前缀)
  meta secret put --db run/meta.db --key run/vault.key [--acl 角色1,角色2] <name>   (值读 stdin)
  meta ticket new --socket <sock> --id <id> --title <t> [--spec-file <票面.md>] [--attempt-cap N] [--hard-cap N]
  meta dispatch --socket <sock> --ticket <id> --role <角色> [--prompt-file <f> 覆盖径] --workdir <d> [--prefer <壳>]
  meta phase --socket <sock> --ticket <id>
  meta events --socket <sock> --ticket <id>
  meta runs --socket <sock> --ticket <id>
  meta budget --socket <sock> --ticket <id> [--attempt-cap N] [--hard-cap N]
  meta observe --socket <sock>
  meta caster --socket <sock> --routes <f> --state-dir <d> --run-dir <r> --notify-cmd <sh> [--once] [--replay-history]
  meta retro --socket <sock> --ticket <id> --verdict pass|reject --ref <判决档>
  meta supervise --bin <bin> [--prev <p>] [--burst N] [--stable 60s] [--backoff 3s] <serve 旗同 meta serve>
  meta upgrade --new <bin> --install <path> [--prev <p>] [--restart-cmd "<cmd>"] [--check-socket <sock>]
  meta version`)
}

// serveFlags serve/supervise 共用的参数面（supervise 原样透传给 serve 子进程）。
type serveFlags struct {
	db, sock, runDir, injectRoot, injectRecipe, configP *string
	progressTimeout, stagger                           *time.Duration
}

func bindServeFlags(fs *flag.FlagSet) serveFlags {
	return serveFlags{
		db:              fs.String("db", "run/meta.db", "SQLite 库路径"),
		sock:            fs.String("socket", "run/meta.sock", "席位面 unix socket"),
		runDir:          fs.String("rundir", "run", "run 目录（seats/logs/vault.key）"),
		progressTimeout: fs.Duration("progress-timeout", 30*time.Minute, "进度帧闹钟"),
		stagger:         fs.Duration("stagger", 10*time.Second, "②a 限流错峰时延"),
		injectRoot:      fs.String("inject-root", "", "注入机源根（roles/+docs/两库所在；空＝注入机关）"),
		injectRecipe:    fs.String("inject-recipe", "", "项目配方/纪律文档（相对 inject-root，逗号分隔多件）"),
		configP:         fs.String("config", "", "网关配置 textproto（缺省 <可执行>/../config/gateway.textproto；必载拒起）"),
	}
}

// serveArgs 参数面→serve argv（supervise 透传用）。路径绝对化（D-23）：
// 相对 socket 会被 gRPC 误析为 URL scheme，席位 env 也要求绝对路径。
func (f serveFlags) serveArgs() []string {
	a := []string{"serve",
		"--db", absOr(f.db, "run/meta.db"),
		"--socket", absOr(f.sock, "run/meta.sock"),
		"--rundir", absOr(f.runDir, "run"),
		"--progress-timeout", f.progressTimeout.String(),
		"--stagger", f.stagger.String()}
	if *f.injectRoot != "" {
		a = append(a, "--inject-root", absOr(f.injectRoot, ""))
	}
	if *f.injectRecipe != "" {
		a = append(a, "--inject-recipe", *f.injectRecipe)
	}
	if *f.configP != "" {
		a = append(a, "--config", absOr(f.configP, ""))
	}
	return a
}

func absOr(p *string, def string) string {
	v := def
	if p != nil && *p != "" {
		v = *p
	}
	if abs, err := filepath.Abs(v); err == nil {
		return abs
	}
	return v
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	f := bindServeFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	// 路径归一（D-23）：相对 socket 会被 gRPC 误析为 URL scheme；
	// 席位 env（META_*）与注入文件路径跨 cwd 引用，必须绝对。
	*f.db, *f.sock, *f.runDir = absOr(f.db, ""), absOr(f.sock, ""), absOr(f.runDir, "")
	if *f.injectRoot != "" {
		*f.injectRoot = absOr(f.injectRoot, "")
	}

	// 配置必载（D-9）：缺省 <可执行>/../config/gateway.textproto；缺/坏拒起。
	cfgPath := *f.configP
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}
	gc, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	// 显式旗标 override 配置行（现行旗标全保留；配置是底不是顶）。
	set := map[string]bool{}
	fs.Visit(func(fl *flag.Flag) { set[fl.Name] = true })
	gcDur := func(name string, d *durationpb.Duration, flagV time.Duration) time.Duration {
		if !set[name] && d != nil && d.AsDuration() > 0 {
			return d.AsDuration()
		}
		return flagV
	}
	gcfg := gateway.Config{
		DBPath:            *f.db,
		RunDir:            *f.runDir,
		SocketPath:        *f.sock,
		ProgressTimeout:   gcDur("progress-timeout", gc.ProgressTimeout, *f.progressTimeout),
		Stagger:           gcDur("stagger", gc.Stagger, *f.stagger),
		AuthTimeout:       gcDur("", gc.AuthTimeout, 0),
		AdoptDialTimeout:  gcDur("", gc.AdoptDialTimeout, 0),
		QuickExit:         gcDur("", gc.QuickExit, 0),
		MaxPerShell:       int(gc.MaxPerShell),
		DefaultAttemptCap: gc.DefaultAttemptCap,
		TicketHardCap:     gc.TicketHardCap,
		RoleCaps:          map[string]int64{},
		InjectRoot:        *f.injectRoot,
		InjectRecipes:     splitCSV(*f.injectRecipe),
	}
	for _, rb := range gc.RoleBudget {
		gcfg.RoleCaps[rb.Role] = rb.AttemptCap
	}
	for _, s := range gc.Shell {
		gcfg.ShellRows = append(gcfg.ShellRows, gateway.ShellSeed{
			Name: s.Name, Model: s.Model, Role: s.Role,
			Priority: int(s.Priority), AttemptCap: s.AttemptCap,
		})
	}

	g, err := gateway.New(gcfg)
	if err != nil {
		return err
	}
	defer g.Close()

	errCh := make(chan error, 1)
	go func() { errCh <- g.Serve() }()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case s := <-sig:
		fmt.Fprintln(os.Stderr, "meta: 收信号", s)
		return nil
	case err := <-errCh:
		return err
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// superviseCmd 守护径（B4）：前台拉起 <bin> serve，连崩熔断自动回退 prev。
func superviseCmd(args []string) error {
	fs := flag.NewFlagSet("supervise", flag.ContinueOnError)
	f := bindServeFlags(fs)
	bin := fs.String("bin", "", "被守护二进制路径（必；D-24 可配缝）")
	prev := fs.String("prev", "", "回滚源（缺省 <bin>.prev）")
	burst := fs.Int("burst", 5, "连崩熔断阈值")
	stable := fs.Duration("stable", 60*time.Second, "子进程稳态时长（归零连崩计数）")
	backoff := fs.Duration("backoff", 3*time.Second, "重起间隔")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *bin == "" {
		return fmt.Errorf("--bin 必填")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	code := supervise.Run(ctx, supervise.Config{
		Bin:     *bin,
		Prev:    *prev,
		Args:    f.serveArgs(),
		Burst:   *burst,
		Stable:  *stable,
		Backoff: *backoff,
		Out:     os.Stderr,
	})
	if code != 0 {
		return fmt.Errorf("supervise 熔断停机 exit=%d", code)
	}
	return nil
}

// upgradeCmd 自升级径：换二进制＋restart＋健康自检，自检红回退 prev。
func upgradeCmd(args []string) error {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	newBin := fs.String("new", "", "新二进制路径（必）")
	install := fs.String("install", "", "安装目标路径（必，如 /opt/meta/bin/meta）")
	prev := fs.String("prev", "", "回滚快照落点（缺省 <install>.prev）")
	restartCmd := fs.String("restart-cmd", "", "重启命令（如 systemctl --user restart meta-gateway）")
	checkSock := fs.String("check-socket", "", "健康自检 socket（空＝不自检）")
	checkTimeout := fs.Duration("check-timeout", 15*time.Second, "自检上限")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *newBin == "" || *install == "" {
		return fmt.Errorf("--new 与 --install 必填")
	}
	return upgrade.Do(upgrade.Config{
		NewBin:       *newBin,
		Install:      *install,
		Prev:         *prev,
		RestartCmd:   *restartCmd,
		CheckSock:    *checkSock,
		CheckTimeout: *checkTimeout,
		Out:          os.Stderr,
	})
}

func secret(args []string) error {
	if len(args) < 1 || args[0] != "put" {
		usage()
		return fmt.Errorf("secret 子命令只实现 put")
	}
	fs := flag.NewFlagSet("secret put", flag.ContinueOnError)
	dbPath := fs.String("db", "run/meta.db", "SQLite 库路径")
	keyPath := fs.String("key", "run/vault.key", "主密钥路径")
	acl := fs.String("acl", "", "放行角色，逗号分隔（CEO,CTO,审计,施工）")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("secret put <name>")
	}
	name := fs.Arg(0)

	key, err := db.LoadOrCreateVaultKey(*keyPath)
	if err != nil {
		return err
	}
	if err := os.Chmod(*keyPath, 0o600); err != nil {
		return err
	}

	plain, err := io.ReadAll(os.Stdin) // 值走 stdin，不落 argv/日志
	if err != nil {
		return err
	}
	ct, err := db.EncryptSecret(key, plain)
	if err != nil {
		return err
	}

	d, err := db.Open(*dbPath)
	if err != nil {
		return err
	}
	defer d.Close()
	var roles []string
	if *acl != "" {
		roles = strings.Split(*acl, ",")
	}
	if err := d.PutSecret(context.Background(), name, ct, roles); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "meta: secret %s 已写入（%dB 密文，acl=%q）\n", name, len(ct), *acl)
	return nil
}

// ---------- 管理面 CLI（AdminGateway over unix socket） ----------

func admin(sock string) (pbv1.AdminGatewayClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient("unix://"+absOr(&sock, ""),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return pbv1.NewAdminGatewayClient(conn), conn, nil
}

func ticketCmd(args []string) error {
	if len(args) < 1 || args[0] != "new" {
		return fmt.Errorf("ticket 子命令只实现 new")
	}
	fs := flag.NewFlagSet("ticket new", flag.ContinueOnError)
	sock := fs.String("socket", "run/meta.sock", "")
	id := fs.String("id", "", "")
	title := fs.String("title", "", "")
	specFile := fs.String("spec-file", "", "票面 markdown（八栏全量入库；缺一不立）")
	acap := fs.Int64("attempt-cap", 0, "每次尝试内 token 上限")
	hcap := fs.Int64("hard-cap", 0, "票级累计硬顶")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("--id 必填")
	}
	var spec []byte
	if *specFile != "" {
		raw, err := os.ReadFile(*specFile)
		if err != nil {
			return err
		}
		ts, err := inject.ParseTicketMD(raw)
		if err != nil {
			return err
		}
		spec, err = proto.Marshal(ts)
		if err != nil {
			return err
		}
	}
	c, conn, err := admin(*sock)
	if err != nil {
		return err
	}
	defer conn.Close()
	ack, err := c.CreateTicket(context.Background(), &pbv1.NewTicket{
		Id: *id, Title: *title, Spec: spec, AttemptCap: *acap, HardCap: *hcap})
	if err != nil {
		return err
	}
	fmt.Println("ticket", *id, "ok=", ack.Ok, ack.Note)
	return nil
}

func dispatchCmd(args []string) error {
	fs := flag.NewFlagSet("dispatch", flag.ContinueOnError)
	sock := fs.String("socket", "run/meta.sock", "")
	tid := fs.String("ticket", "", "")
	role := fs.String("role", "", "")
	pf := fs.String("prompt-file", "", "")
	wd := fs.String("workdir", "", "")
	prefer := fs.String("prefer", "", "该壳尝试提首")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, conn, err := admin(*sock)
	if err != nil {
		return err
	}
	defer conn.Close()
	ack, err := c.Dispatch(context.Background(), &pbv1.DispatchReq{
		TicketId: *tid, Role: *role, PromptFile: *pf, WorkDir: *wd, PreferShell: *prefer})
	if err != nil {
		return err
	}
	if !ack.Ok {
		return fmt.Errorf("dispatch: %s", ack.Note)
	}
	fmt.Println("dispatched", *tid, *role)
	return nil
}

func phaseCmd(args []string) error {
	fs := flag.NewFlagSet("phase", flag.ContinueOnError)
	sock := fs.String("socket", "run/meta.sock", "")
	tid := fs.String("ticket", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, conn, err := admin(*sock)
	if err != nil {
		return err
	}
	defer conn.Close()
	p, err := c.Phase(context.Background(), &pbv1.TicketRef{Id: *tid})
	if err != nil {
		return err
	}
	fmt.Printf("%s\t%s\tfrozen=%v\n", *tid, p.Phase, p.BudgetFrozen)
	return nil
}

func eventsCmd(args []string) error {
	fs := flag.NewFlagSet("events", flag.ContinueOnError)
	sock := fs.String("socket", "run/meta.sock", "")
	tid := fs.String("ticket", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, conn, err := admin(*sock)
	if err != nil {
		return err
	}
	defer conn.Close()
	l, err := c.Events(context.Background(), &pbv1.TicketRef{Id: *tid})
	if err != nil {
		return err
	}
	for _, e := range l.Items {
		fmt.Printf("%d\t%s\t%s→%s\t%s\t%s\n", e.Id, e.Kind, e.FromPhase, e.ToPhase, e.Actor, e.Note)
	}
	return nil
}

func runsCmd(args []string) error {
	fs := flag.NewFlagSet("runs", flag.ContinueOnError)
	sock := fs.String("socket", "run/meta.sock", "")
	tid := fs.String("ticket", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, conn, err := admin(*sock)
	if err != nil {
		return err
	}
	defer conn.Close()
	l, err := c.Runs(context.Background(), &pbv1.TicketRef{Id: *tid})
	if err != nil {
		return err
	}
	for _, r := range l.Items {
		fmt.Printf("%s\tshell=%s\tpid=%d\texit=%d\tdead=%v\tattached=%v\tadopted=%v\n",
			r.RunId, r.Shell, r.Pid, r.ExitCode, r.Dead, r.Attached, r.Adopted)
	}
	return nil
}

func budgetCmd(args []string) error {
	fs := flag.NewFlagSet("budget", flag.ContinueOnError)
	sock := fs.String("socket", "run/meta.sock", "")
	tid := fs.String("ticket", "", "")
	acap := fs.Int64("attempt-cap", -1, "")
	hcap := fs.Int64("hard-cap", -1, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, conn, err := admin(*sock)
	if err != nil {
		return err
	}
	defer conn.Close()
	cur, err := c.Budget(context.Background(), &pbv1.TicketRef{Id: *tid})
	if err != nil {
		return err
	}
	if *acap >= 0 || *hcap >= 0 {
		if *acap < 0 {
			*acap = cur.Cap
		}
		if *hcap < 0 {
			*hcap = cur.HardCap
		}
		if _, err := c.SetBudget(context.Background(), &pbv1.BudgetReq{
			TicketId: *tid, AttemptCap: *acap, HardCap: *hcap}); err != nil {
			return err
		}
		cur, err = c.Budget(context.Background(), &pbv1.TicketRef{Id: *tid})
		if err != nil {
			return err
		}
	}
	fmt.Printf("%s\tcap=%d used=%d\thard=%d total=%d\n", *tid, cur.Cap, cur.Used, cur.HardCap, cur.UsedTotal)
	return nil
}

func observeCmd(args []string) error {
	fs := flag.NewFlagSet("observe", flag.ContinueOnError)
	sock := fs.String("socket", "run/meta.sock", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, conn, err := admin(*sock)
	if err != nil {
		return err
	}
	defer conn.Close()
	stream, err := c.Observe(context.Background(), &pbv1.Empty{})
	if err != nil {
		return err
	}
	for {
		e, err := stream.Recv()
		if err != nil {
			return err
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", e.Kind, e.TicketId, e.RunId, e.Role, e.Note)
	}
}

// casterCmd 播报员引擎（票0004 §3）：AdminGateway 客户端常驻——
// 水位游标回读＋路由表机械映射（notify/dispatch）。
func casterCmd(args []string) error {
	fs := flag.NewFlagSet("caster", flag.ContinueOnError)
	sock := fs.String("socket", "run/meta.sock", "")
	routes := fs.String("routes", "", "路由表 textproto（必）")
	stateDir := fs.String("state-dir", "", "水位/渲染产物目录（必）")
	runDir := fs.String("run-dir", "run", "run 目录（{{run_dir}} 占位符）")
	notifyCmd := fs.String("notify-cmd", "", "通知脚本（必）")
	once := fs.Bool("once", false, "只 connect+resync 一次即退")
	replay := fs.Bool("replay-history", false, "水位缺失时追补历史（缺省快进只推位）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *routes == "" || *stateDir == "" || *notifyCmd == "" {
		return fmt.Errorf("--routes/--state-dir/--notify-cmd 必填")
	}
	sockAbs := absOr(sock, "")
	eng, err := caster.New(caster.Config{
		RoutesPath:    absOr(routes, ""),
		StateDir:      absOr(stateDir, ""),
		RunDir:        absOr(runDir, ""),
		SocketPath:    sockAbs,
		NotifyCmd:     *notifyCmd,
		ReplayHistory: *replay,
		Dial: func(ctx context.Context) (caster.AdminClient, error) {
			c, conn, err := admin(sockAbs)
			if err != nil {
				return nil, err
			}
			return &adminCaster{c: c, conn: conn}, nil
		},
	})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if *once {
		return eng.Once(ctx)
	}
	return eng.Run(ctx)
}

// adminCaster caster.AdminClient 的真实现（AdminGateway over unix socket）。
type adminCaster struct {
	c    pbv1.AdminGatewayClient
	conn *grpc.ClientConn
}

func (a *adminCaster) EventsAfter(ctx context.Context, sinceID int64, limit int32) (*pbv1.EventList, error) {
	return a.c.EventsAfter(ctx, &pbv1.EventCursor{SinceId: sinceID, Limit: limit})
}

func (a *adminCaster) Dispatch(ctx context.Context, req *pbv1.DispatchReq) (*pbv1.Ack, error) {
	return a.c.Dispatch(ctx, req)
}

func (a *adminCaster) Observe(ctx context.Context) (pbv1.AdminGateway_ObserveClient, error) {
	return a.c.Observe(ctx, &pbv1.Empty{})
}

// retroCmd 回翻件入口（§15.4）：滞留 待审 票一次性机械补录 audit_start→
// audit_pass|reject 两跳。
func retroCmd(args []string) error {
	fs := flag.NewFlagSet("retro", flag.ContinueOnError)
	sock := fs.String("socket", "run/meta.sock", "")
	tid := fs.String("ticket", "", "")
	verdict := fs.String("verdict", "", "pass|reject")
	ref := fs.String("ref", "", "判决档/决策号指证（必）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *tid == "" || *verdict == "" || *ref == "" {
		return fmt.Errorf("--ticket/--verdict/--ref 必填")
	}
	c, conn, err := admin(*sock)
	if err != nil {
		return err
	}
	defer conn.Close()
	ack, err := c.RetroVerdict(context.Background(), &pbv1.RetroReq{
		TicketId: *tid, Verdict: *verdict, Ref: *ref})
	if err != nil {
		return err
	}
	if !ack.Ok {
		return fmt.Errorf("retro: %s", ack.Note)
	}
	fmt.Println("retro", *tid, *verdict, "ok:", ack.Note)
	return nil
}
