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
