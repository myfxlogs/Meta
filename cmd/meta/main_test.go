package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
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
