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
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"meta/internal/db"
	"meta/internal/gateway"
	pbv1 "meta/internal/pb/metav1"
	"meta/internal/seat"
)

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
  meta secret put --db run/meta.db --key run/vault.key [--acl 角色1,角色2] <name>   (值读 stdin)
  meta ticket new --socket <sock> --id <id> --title <t> [--attempt-cap N] [--hard-cap N]
  meta dispatch --socket <sock> --ticket <id> --role <角色> --prompt-file <f> --workdir <d> [--prefer <壳>]
  meta phase --socket <sock> --ticket <id>
  meta events --socket <sock> --ticket <id>
  meta runs --socket <sock> --ticket <id>
  meta budget --socket <sock> --ticket <id> [--attempt-cap N] [--hard-cap N]
  meta observe --socket <sock>`)
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	dbPath := fs.String("db", "run/meta.db", "SQLite 库路径")
	sock := fs.String("socket", "run/meta.sock", "席位面 unix socket")
	runDir := fs.String("rundir", "run", "run 目录（seats/logs/vault.key）")
	progressTimeout := fs.Duration("progress-timeout", 30*time.Minute, "进度帧闹钟")
	stagger := fs.Duration("stagger", 10*time.Second, "②a 限流错峰时延")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := gateway.New(gateway.Config{
		DBPath:          *dbPath,
		RunDir:          *runDir,
		SocketPath:      *sock,
		ProgressTimeout: *progressTimeout,
		Stagger:         *stagger,
	})
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
	conn, err := grpc.NewClient("unix://"+sock,
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
	acap := fs.Int64("attempt-cap", 0, "每次尝试内 token 上限")
	hcap := fs.Int64("hard-cap", 0, "票级累计硬顶")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("--id 必填")
	}
	c, conn, err := admin(*sock)
	if err != nil {
		return err
	}
	defer conn.Close()
	ack, err := c.CreateTicket(context.Background(), &pbv1.NewTicket{
		Id: *id, Title: *title, AttemptCap: *acap, HardCap: *hcap})
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
