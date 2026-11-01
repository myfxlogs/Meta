package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		RunID: "run-cli", TicketID: "TC", Role: "施工", Shell: "testshell",
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
	if err := g.DB().UpsertProject(ctx, db.Project{ID: "meta", DefaultBranch: "master"}); err != nil {
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
