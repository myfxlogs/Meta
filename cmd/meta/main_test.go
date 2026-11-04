package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"meta/internal/config"
	"meta/internal/db"
	"meta/internal/gateway"
	pbv1 "meta/internal/pb/metav1"
)

// TestServe_无配置拒起（票0004 锚 7 首句）：无 --config 且缺省路径
// （<可执行>/../config/gateway.textproto）无文件→serve 拒起且报路径指引
// （配置必载 D-9——fail loud，登记表在码里就不叫配置化）。
func TestServe_无配置拒起(t *testing.T) {
	// 前提断言：测试二进制不在生产布局，缺省路径必不存在——保测试确定性。
	if _, err := os.Stat(config.DefaultPath()); err == nil {
		t.Skipf("缺省路径 %s 存在（生产布局跑测试）——不适用", config.DefaultPath())
	}
	err := serve([]string{
		"--db", filepath.Join(t.TempDir(), "m.db"),
		"--socket", filepath.Join(t.TempDir(), "m.sock"),
	})
	if err == nil || !strings.Contains(err.Error(), "--config") {
		t.Fatalf("无配置应拒起且带指引: %v", err)
	}
}

// TestBoardCLI_端到端（票0012 锚 5）：meta board 打印 ≥full＋1 变帧——
// --socket（unix gRPC）与 --connect（TCP Connect 二进制）两径各验。
func TestBoardCLI_端到端(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "run")
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
	defer g.Close()
	ct, err := db.EncryptSecret(key, []byte("tok-cli"))
	if err != nil {
		t.Fatal(err)
	}
	if err := g.DB().PutSecret(context.Background(), gateway.BoardTokenName, ct, nil); err != nil {
		t.Fatal(err)
	}
	bl, err := wireBoard(g, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer bl.Close()
	go g.Serve()
	// 非终态票入全态帧。
	if err := g.DB().CreateTicket(context.Background(), "TC", "CLI票", nil); err != nil {
		t.Fatal(err)
	}
	// 造一条活 run——full 里至少一席。
	if err := g.DB().CreateRun(context.Background(), &db.Run{
		RunID: "run-cli", TicketID: "TC", Role: "施工", Shell: "testshell", PID: os.Getpid(), // T35：调和器收尸假 pid——测试进程自任活席
		RunToken: "tk-cli", SocketPath: filepath.Join(runDir, "seats", "run-cli.sock"),
		LogPath: filepath.Join(runDir, "logs", "run-cli.log"),
	}); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"--socket", sock, "--token", "tok-cli", "--n", "2"},
		{"--connect", "http://" + bl.Addr, "--token", "tok-cli", "--n", "2"},
	} {
		pr, pw := io.Pipe()
		done := make(chan error, 1)
		go func() { done <- boardCmd(args, pw) }()
		// 读到 FULL 行后触发一帧变化（attach→seat upsert）：经真 Attach 径。
		go func() {
			sc := bufio.NewScanner(pr)
			for sc.Scan() {
				if strings.HasPrefix(sc.Text(), "FULL") {
					conn, err := grpc.NewClient("unix://"+sock,
						grpc.WithTransportCredentials(insecure.NewCredentials()))
					if err != nil {
						return
					}
					defer conn.Close()
					st, err := pbv1.NewSeatGatewayClient(conn).Attach(context.Background())
					if err != nil {
						return
					}
					_ = st.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{
						Auth: &pbv1.RunAuth{RunToken: "tk-cli"}}})
					_ = st.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Progress{
						Progress: &pbv1.Progress{Stage: "写码", Note: "cli"}}})
				}
			}
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("boardCmd %v: %v", args, err)
			}
		case <-time.After(15 * time.Second):
			t.Fatalf("boardCmd %v 超时未收满 2 帧", args)
		}
		_ = pw.Close()
	}
}

// TestAdmin_相对socket（D-23 锚）：`meta observe --socket run/meta.sock`
// 相对路径必须绝对化后连通——修复前 gRPC 把 "run/meta.sock" 误析为
// URL scheme 永远拨不通。
func TestAdmin_相对socket(t *testing.T) {
	d := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	g, err := gateway.New(gateway.Config{
		DBPath:     "run/meta.db",
		RunDir:     "run",
		SocketPath: "run/meta.sock",
	})
	if err != nil {
		t.Fatal(err)
	}
	go g.Serve()
	defer g.Close()

	// 等 socket（有界）
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat("run/meta.sock"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("socket 未起")
		}
		<-time.After(20 * time.Millisecond)
	}

	c, conn, err := admin("run/meta.sock") // 相对路径——D-23 病灶点
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = c.Phase(ctx, &pbv1.TicketRef{Id: "nope"})
	// 连通＝拿到服务端应答（NotFound＝票不在，服务活着）；拨不通才是红。
	if st, ok := status.FromError(err); !ok || st.Code() != codes.NotFound {
		t.Fatalf("相对 socket 未连通: %v", err)
	}
	// serve 侧路径也已绝对化：库文件在 cwd/run 下。
	if _, err := os.Stat(filepath.Join(d, "run", "meta.db")); err != nil {
		t.Fatal("serve 相对路径未落 cwd")
	}
}

// TestCLI_GlueE2E 债 D-38 缺口二（CLI 胶水端到端）：真网关 socket 上跑
// ticket new → dispatch → phase → events 全链——胶水函数非单测即集成。
func TestCLI_GlueE2E(t *testing.T) {
	d := t.TempDir()
	sock := filepath.Join(d, "run", "meta.sock")
	g, err := gateway.New(gateway.Config{
		DBPath: filepath.Join(d, "meta.db"), RunDir: filepath.Join(d, "run"),
		SocketPath: sock, SeatArgv: []string{"/bin/true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	go g.Serve()
	defer g.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("socket 未起")
		}
		<-time.After(20 * time.Millisecond)
	}
	if err := ticketCmd([]string{"new", "--socket", sock, "--id", "tcli", "--title", "cli 票"}); err != nil {
		t.Fatal("ticket new:", err)
	}
	if err := dispatchCmd([]string{"--socket", sock, "--ticket", "tcli", "--role", "施工", "--workdir", "/tmp"}); err != nil {
		t.Fatal("dispatch:", err)
	}
	if err := phaseCmd([]string{"--socket", sock, "--ticket", "tcli"}); err != nil {
		t.Fatal("phase:", err)
	}
	c, conn, err := admin(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	p, err := c.Phase(context.Background(), &pbv1.TicketRef{Id: "tcli"})
	if err != nil || p.Phase != "已派" {
		t.Fatalf("相位应 已派: %v %v", p, err)
	}
	if err := eventsCmd([]string{"--socket", sock, "--ticket", "tcli"}); err != nil {
		t.Fatal("events:", err)
	}
}

// TestCLI_ProjectLink 票0015 锚9：真网关 socket 上——
// `meta ticket new --project`（已登记→落／未登记→拒票不立）、
// `meta ticket set-project`（存量回填径→RPC→闸）、
// `meta dispatch --project`（派发径挂接）、`meta require --project`（需求票挂接）。
func TestCLI_ProjectLink(t *testing.T) {
	d := t.TempDir()
	sock := filepath.Join(d, "run", "meta.sock")
	g, err := gateway.New(gateway.Config{
		DBPath: filepath.Join(d, "meta.db"), RunDir: filepath.Join(d, "run"),
		SocketPath: sock, SeatArgv: []string{"/bin/true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	go g.Serve()
	defer g.Close()
	ctx := context.Background()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("socket 未起")
		}
		<-time.After(20 * time.Millisecond)
	}
	// T36：项目配方缺省 record_only——require 径需 live 才真立票。
	if err := g.DB().UpsertProject(ctx, db.Project{ID: "meta", DefaultBranch: "master", AutoTicketMode: "live"}); err != nil {
		t.Fatal(err)
	}
	// ticket new --project：未登记→拒且票不立。
	if err := ticketCmd([]string{"new", "--socket", sock, "--id", "bad", "--title", "x",
		"--project", "nope"}); err == nil {
		t.Fatal("未登记项目立票应拒")
	}
	if tk, _ := g.DB().GetTicket(ctx, "bad"); tk != nil {
		t.Fatal("未登记拒后票不应立")
	}
	if err := ticketCmd([]string{"new", "--socket", sock, "--id", "tnew", "--title", "x",
		"--project", "meta"}); err != nil {
		t.Fatal("ticket new --project:", err)
	}
	if tk, _ := g.DB().GetTicket(ctx, "tnew"); tk == nil || tk.Project != "meta" {
		t.Fatalf("ticket new --project 未落: %+v", tk)
	}
	// ticket set-project：存量回填径。
	if err := g.DB().CreateTicket(ctx, "told", "存量票", nil); err != nil {
		t.Fatal(err)
	}
	if err := ticketCmd([]string{"set-project", "--socket", sock,
		"--ticket", "told", "--project", "meta"}); err != nil {
		t.Fatal("ticket set-project:", err)
	}
	if tk, _ := g.DB().GetTicket(ctx, "told"); tk.Project != "meta" {
		t.Fatalf("set-project 未落: %+v", tk)
	}
	// dispatch --project：派发径挂接（新票派施工，相位走边到已派）。
	if err := g.DB().CreateTicket(ctx, "tdsp", "派发票", nil); err != nil {
		t.Fatal(err)
	}
	if err := dispatchCmd([]string{"--socket", sock, "--ticket", "tdsp", "--role", "施工",
		"--workdir", "/tmp", "--project", "meta"}); err != nil {
		t.Fatal("dispatch --project:", err)
	}
	if tk, _ := g.DB().GetTicket(ctx, "tdsp"); tk.Project != "meta" {
		t.Fatalf("dispatch --project 未落: %+v", tk)
	}
	// require --project：CEO 面（META_RUN_TOKEN＝真 CEO run）。
	if err := g.DB().CreateRun(ctx, &db.Run{
		RunID: "ceo-cli", Role: "CEO", RunToken: "tok-ceo", PID: 1,
		SocketPath: filepath.Join(d, "run", "seats", "ceo.sock"),
		Provenance: "terminal",
	}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("META_RUN_TOKEN", "tok-ceo")
	if err := requireCmd([]string{"--socket", sock, "--goal", "需求挂项目",
		"--project", "meta"}); err != nil {
		t.Fatal("require --project:", err)
	}
	// 需求票 id 不知——按 project 查行存在性。
	found := false
	for _, tk := range mustNonTerminal(t, g) {
		if tk.Project == "meta" && tk.ID != "tnew" && tk.ID != "told" && tk.ID != "tdsp" {
			found = true
		}
	}
	if !found {
		t.Fatal("require --project 需求票未挂 meta")
	}
}

// pushPhase 建票并沿主链推到目标相位（CreateTicket 自带 mandate v1，
// 施工中/部署中边核过）。target=已立 时建完即返。
func pushPhase(t *testing.T, h *db.DB, id, title string, target db.Phase) {
	t.Helper()
	ctx := context.Background()
	if err := h.CreateTicket(ctx, id, title, nil); err != nil {
		t.Fatal(err)
	}
	chain := []struct {
		to   db.Phase
		kind db.EdgeKind
	}{
		{db.PhaseDesigning, db.EdgeDesignStart},
		{db.PhaseAssigned, db.EdgeDispatch},
		{db.PhaseWorking, db.EdgeClaim},
		{db.PhaseReview, db.EdgeDeliver},
		{db.PhaseAuditing, db.EdgeAuditStart},
		{db.PhaseToDeploy, db.EdgeAuditPass},
		{db.PhaseDeploying, db.EdgeDeployStart},
		{db.PhaseDone, db.EdgeDeployDone},
	}
	for _, s := range chain {
		if p, _ := h.PhaseOf(ctx, id); p == target {
			return
		}
		if _, err := h.Transition(ctx, id, s.to, s.kind, "test", ""); err != nil {
			t.Fatalf("pushPhase %s→%s: %v", id, s.to, err)
		}
	}
}

// TestTicketsCLI 票0052 锚：非终态全列按票号序／终态零出现／空板
// 「无在途票」／标题截断——直读库四态（无网关依赖，纯读命令面无
// RPC 面）；另验 --socket 推导库径与库缺失报错不建空库。
func TestTicketsCLI(t *testing.T) {
	// T47：socket RPC 一致径——起真网关，经其 DB 柄造票。
	d := t.TempDir()
	sock := filepath.Join(d, "run", "meta.sock")
	g, err := gateway.New(gateway.Config{
		DBPath: filepath.Join(d, "meta.db"), RunDir: filepath.Join(d, "run"),
		SocketPath: sock, SeatArgv: []string{"/bin/true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	go g.Serve()
	defer g.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("socket 未起")
		}
		<-time.After(20 * time.Millisecond)
	}
	h := g.DB()
	ctx := context.Background()

	run := func(args ...string) (string, error) {
		var sb strings.Builder
		err := ticketsCmd(append(args, "--socket", sock), &sb)
		return sb.String(), err
	}

	// 空板显式「无在途票」（严格单行）。
	out, err := run()
	if err != nil || out != "无在途票\n" {
		t.Fatalf("空板应单行无在途票: %q err=%v", out, err)
	}

	// 非终态三相位＋blocked（非终态照列）＋两终态——按反序立票验票号序。
	pushPhase(t, h, "T30", "审票", db.PhaseAuditing)
	pushPhase(t, h, "T04", "已派票", db.PhaseAssigned)
	pushPhase(t, h, "T12", "完工票", db.PhaseDone)
	pushPhase(t, h, "T07", "长题票 "+strings.Repeat("标题很长", 12), db.PhaseCreated)
	if err := h.CreateTicket(ctx, "T25", "作废票", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Transition(ctx, "T25", db.PhaseVoid, db.EdgeVoid, "test", ""); err != nil {
		t.Fatal(err)
	}
	if err := h.CreateTicket(ctx, "T18", "卡票", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Transition(ctx, "T18", db.PhaseBlocked, db.EdgeBlocked, "gateway", "人工裁定"); err != nil {
		t.Fatal(err)
	}

	out, err = run()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("非终态应 4 行（终态零出现）: %q", out)
	}
	for i, want := range []struct{ id, phase string }{
		{"T04", "已派"}, {"T07", "已立"}, {"T18", "blocked"}, {"T30", "审计中"},
	} {
		f := strings.Split(lines[i], "\t")
		if len(f) != 3 || f[0] != want.id || f[1] != want.phase {
			t.Fatalf("行 %d 应 %s %s: %q", i, want.id, want.phase, lines[i])
		}
	}
	// 标题截断：>40 rune 收「…」且恰 41 rune；短题原样。
	if got := strings.Split(lines[1], "\t")[2]; !strings.HasSuffix(got, "…") ||
		len([]rune(got)) != 41 {
		t.Fatalf("长题应截 40+…: %q", got)
	}
	if got := strings.Split(lines[0], "\t")[2]; got != "已派票" {
		t.Fatalf("短题应原样: %q", got)
	}
	for _, gone := range []string{"T12", "T25"} {
		if strings.Contains(out, gone) {
			t.Fatalf("终态票 %s 不应出现: %q", gone, out)
		}
	}
}

// TestStatusCLI 票0051 锚：版本戳/在途计数/blocked 计数三要素＋空板显式零
// ——真网关 socket 上 svc=up＋推导库径同读／满板相位逐项计数与 blocked
// 单列／终态不计在途／活席数／死 socket svc=down 降级仍出计数／恒单行。
// 满板断言在无网关库上做——活网关调和器会迁边，计数面非确定。
func TestStatusCLI(t *testing.T) {
	// 面一：活网关（sock/db 同位＝生产布局）——svc=up＋--socket 推导库径。
	dir := t.TempDir()
	runDir := filepath.Join(dir, "run")
	sock := filepath.Join(runDir, "meta.sock")
	dbp := filepath.Join(runDir, "meta.db")
	g, err := gateway.New(gateway.Config{
		DBPath: dbp, RunDir: runDir, SocketPath: sock,
		SeatArgv: []string{"/bin/true"},
		// T52：调和窗口拉大——造票迁边不在测窗内发生（计数断言确定性）。
		ReconcileDebounce: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	go g.Serve()
	defer g.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("socket 未起")
		}
		<-time.After(20 * time.Millisecond)
	}
	run := func(args ...string) string {
		var sb strings.Builder
		if err := statusCmd(args, &sb); err != nil {
			t.Fatalf("statusCmd %v: %v", args, err)
		}
		return sb.String()
	}
	oneLine := func(out string) {
		if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
			t.Fatalf("应严格单行: %q", out)
		}
	}

	// 空板（网关库零票即断言，先于调和器迁边窗口）：三要素显式零＋
	// svc=up＋全相位字段在＋库径推导命中（否则库缺失已报错）。
	out := run("--socket", sock)
	oneLine(out)
	for _, want := range []string{"meta ", "svc=up", "在途=0", "blocked=0", "活席=0",
		"已立=0", "设计中=0", "已派=0", "施工中=0", "待审=0", "审计中=0", "待部署=0", "部署中=0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("空板缺 %q: %q", want, out)
		}
	}

	// 面二：活网关库柄造满板（直写不发事件不调迁——调和器零 kick）。
	h := g.DB()
	ctx := context.Background()
	for i, ph := range db.NonTerminal {
		pushPhase(t, h, "S"+string(rune('1'+i)), "S票", ph)
	}
	if err := h.CreateTicket(ctx, "SB", "卡票", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Transition(ctx, "SB", db.PhaseBlocked, db.EdgeBlocked,
		"gateway", "未知/禁动失败 kind=seat_dead——人工裁定"); err != nil {
		t.Fatal(err)
	}
	pushPhase(t, h, "SD", "完工票", db.PhaseDone)
	for _, r := range []*db.Run{
		{RunID: "r-live", TicketID: "S4", Role: "施工", Shell: "testshell", PID: os.Getpid(), RunToken: "tk-live"},
		{RunID: "r-dead", TicketID: "S4", Role: "施工", Shell: "testshell", PID: os.Getpid(), RunToken: "tk-dead"},
	} {
		if err := h.CreateRun(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.MarkDead(ctx, "r-dead"); err != nil {
		t.Fatal(err)
	}
	out = run("--socket", sock)
	oneLine(out)
	// T52 钦定锚＝与库内实况一致（活网关迁边随时序——对 DB 真理面
	// 实时核对相位计数/blocked/活席，不押固定数）。
	ts, _ := h.NonTerminalTickets(ctx)
	wantPh := map[db.Phase]int{}
	bl := 0
	for _, tt := range ts {
		if tt.Phase == db.PhaseBlocked {
			bl++
			continue
		}
		wantPh[tt.Phase]++
	}
	ar, _ := h.ActiveRuns(ctx)
	must := []string{fmt.Sprintf("在途=%d", len(ts)-bl), fmt.Sprintf("blocked=%d", bl),
		fmt.Sprintf("活席=%d", len(ar))}
	for _, ph := range db.NonTerminal {
		must = append(must, fmt.Sprintf("%s=%d", ph, wantPh[ph]))
	}
	for _, want := range must {
		if !strings.Contains(out, want) {
			t.Fatalf("满板缺 %q: %q", want, out)
		}
	}

	// T52：socket 不可达＝「需 socket」人话错（直库径已废）。
	var sb strings.Builder
	if err := statusCmd([]string{"--socket", filepath.Join(dir, "none", "meta.sock")}, &sb); err == nil ||
		!strings.Contains(err.Error(), "需 socket") {
		t.Fatalf("socket 不可达应报需 socket: %v", err)
	}
}

func mustNonTerminal(t *testing.T, g *gateway.Gateway) []*db.Ticket {
	t.Helper()
	ts, err := g.DB().NonTerminalTickets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// TestImportLegacy_Parser 锚8 补强：importLegacy 解析器本体直测——fixture md
// 两个决策块→两行（provenance 抽取）；二跑零增（kind+title 幂等）。
func TestImportLegacy_Parser(t *testing.T) {
	d := t.TempDir()
	dbp := filepath.Join(d, "m.db")
	h, err := db.Open(dbp)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ctx := context.Background()
	md := []byte("# ceo_memory\n\n" +
		"### 决策 1：上线路径\n- 触发：业主 terminal\n- provenance：`terminal`\n- 裁：走 A\n\n" +
		"### 决策 2：预算追加\n- provenance：`feishu:o1`\n- 裁：加 10 万\n")
	if n, err := importLegacy(ctx, h, md); err != nil || n != 2 {
		t.Fatalf("首跑应入 2 行: n=%d err=%v", n, err)
	}
	if n, err := importLegacy(ctx, h, md); err != nil || n != 0 {
		t.Fatalf("二跑应零增: n=%d err=%v", n, err)
	}
	rows, err := h.ListMemory(ctx, "decision", 0)
	if err != nil || len(rows) != 2 {
		t.Fatalf("行数不符: %v %d", err, len(rows))
	}
	byTitle := map[string]*db.Memory{}
	for _, m := range rows {
		byTitle[m.Title] = m
	}
	if m := byTitle["上线路径"]; m == nil || m.Provenance != "terminal" || m.EventRef != "legacy-import" {
		t.Fatalf("决策 1 行不符: %+v", m)
	}
	if m := byTitle["预算追加"]; m == nil || m.Provenance != "feishu:o1" {
		t.Fatalf("决策 2 行不符: %+v", m)
	}
	if m := byTitle["上线路径"]; m != nil && !strings.Contains(string(m.Content), "走 A") {
		t.Fatalf("正文未入: %q", m.Content)
	}
}

// TestCLI_TicketResume T39 锚：blocked 票经 `meta ticket resume` 回原相位。
func TestCLI_TicketResume(t *testing.T) {
	d := t.TempDir()
	sock := filepath.Join(d, "run", "meta.sock")
	g, err := gateway.New(gateway.Config{
		DBPath: filepath.Join(d, "meta.db"), RunDir: filepath.Join(d, "run"),
		SocketPath: sock, SeatArgv: []string{"/bin/true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	go g.Serve()
	defer g.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("socket 未起")
		}
		<-time.After(20 * time.Millisecond)
	}
	ctx := context.Background()
	if err := g.DB().CreateTicketProv(ctx, "tr39", "blocked 票", nil, "terminal", ""); err != nil {
		t.Fatal(err)
	}
	// 手工推到 blocked（已派→blocked 边存在径：dispatch→blocked 链可直跳——
	// 用 blocked 边钦定径：Transition 任何相位皆可→blocked）。
	if _, err := g.DB().Transition(ctx, "tr39", db.PhaseBlocked, db.EdgeBlocked, "gateway", "失败表到限"); err != nil {
		t.Fatal(err)
	}
	if err := ticketCmd([]string{"resume", "--socket", sock, "--ticket", "tr39"}); err != nil {
		t.Fatal("ticket resume:", err)
	}
	if p, _ := g.DB().PhaseOf(ctx, "tr39"); p == db.PhaseBlocked {
		t.Fatal("resume 后仍 blocked")
	}
}

// TestBlockedCLI blocked 运维窗（T52：socket RPC 一致径）——空表显式
// 「无」／blocked 票必现带末因原文／非 blocked 零出现／多票按票号序。
func TestBlockedCLI(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "run", "meta.sock")
	g, err := gateway.New(gateway.Config{
		DBPath: filepath.Join(dir, "meta.db"), RunDir: filepath.Join(dir, "run"),
		SocketPath: sock, SeatArgv: []string{"/bin/true"}})
	if err != nil {
		t.Fatal(err)
	}
	go g.Serve()
	defer g.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("socket 未起")
		}
		<-time.After(20 * time.Millisecond)
	}
	h := g.DB()
	ctx := context.Background()
	run := func(args ...string) string {
		var sb strings.Builder
		if err := blockedCmd(append(args, "--socket", sock), &sb); err != nil {
			t.Fatalf("blockedCmd %v: %v", args, err)
		}
		return sb.String()
	}

	// 空表。
	if out := run(); !strings.Contains(out, "无 blocked 票") {
		t.Fatalf("空表应显式「无」: %q", out)
	}

	// 造票：先 B2 后 B1——输出须按票号序（B1 在 B2 前）；NC 非 blocked 零出现。
	if err := h.CreateTicket(ctx, "B2", "乙票", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Transition(ctx, "B2", db.PhaseBlocked, db.EdgeBlocked,
		"gateway", "失败表到限 kind=spawn_failed 计数=4>cap=3：next_shell"); err != nil {
		t.Fatal(err)
	}
	if err := h.CreateTicket(ctx, "B1", "甲票", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Transition(ctx, "B1", db.PhaseBlocked, db.EdgeBlocked,
		"gateway", "未知/禁动失败 kind=seat_dead——人工裁定"); err != nil {
		t.Fatal(err)
	}
	if err := h.CreateTicket(ctx, "NC", "正常票", nil); err != nil {
		t.Fatal(err)
	}

	out := run()
	i1, i2 := strings.Index(out, "B1"), strings.Index(out, "B2")
	if i1 < 0 || i2 < 0 || i1 > i2 {
		t.Fatalf("多票序应 B1 前 B2 后: %q", out)
	}
	for _, want := range []string{"甲票", "乙票",
		"未知/禁动失败 kind=seat_dead——人工裁定",
		"失败表到限 kind=spawn_failed 计数=4>cap=3：next_shell",
		"meta rule --action redirect --ticket B1",
		"meta rule --action void --ticket B2", "已立→blocked"} {
		if !strings.Contains(out, want) {
			t.Fatalf("输出缺 %q: %q", want, out)
		}
	}
	if strings.Contains(out, "NC") || strings.Contains(out, "正常票") {
		t.Fatalf("非 blocked 票应零出现: %q", out)
	}
}
