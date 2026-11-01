package upgrade

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"

	pbv1 "meta/internal/pb/metav1"
)

func mkBin(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestDo_换装与prev快照：install→prev 拷贝、新二进制原子落位、restart 令执行。
func TestDo_换装与prev快照(t *testing.T) {
	d := t.TempDir()
	install := filepath.Join(d, "meta")
	marker := filepath.Join(d, "restarted")
	mkBin(t, install, "echo old")
	mkBin(t, filepath.Join(d, "new"), "echo meta-test-v2")

	err := Do(Config{
		NewBin:     filepath.Join(d, "new"),
		Install:    install,
		RestartCmd: "echo x >>" + marker,
		Out:        os.Stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(install)
	if string(got) != "#!/bin/sh\necho meta-test-v2" {
		t.Fatalf("新二进制未落位: %s", got)
	}
	pv, err := os.ReadFile(install + ".prev")
	if err != nil || string(pv) != "#!/bin/sh\necho old" {
		t.Fatalf("prev 快照错: %v %s", err, pv)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("restart 令未执行")
	}
}

// TestDo_预检拦坏版：version 跑不动的新二进制不进 install。
func TestDo_预检拦坏版(t *testing.T) {
	d := t.TempDir()
	install := filepath.Join(d, "meta")
	mkBin(t, install, "echo old")
	mkBin(t, filepath.Join(d, "bad"), "exit 9") // version 跑不出

	if err := Do(Config{NewBin: filepath.Join(d, "bad"), Install: install}); err == nil {
		t.Fatal("坏新版应被预检拦下")
	}
	got, _ := os.ReadFile(install)
	if string(got) != "#!/bin/sh\necho old" {
		t.Fatal("坏版污染了 install")
	}
	if _, err := os.Stat(install + ".prev"); !os.IsNotExist(err) {
		t.Fatal("预检失败不该写 prev")
	}
}

// TestProbe_死活 Probe 对真 gRPC server 与死 socket 两态。
func TestProbe_死活(t *testing.T) {
	d := t.TempDir()
	sock := filepath.Join(d, "meta.sock")
	if err := Probe(sock, 300*time.Millisecond); err == nil {
		t.Fatal("无服务 socket 自检应红")
	}
	// 起真 gRPC server（未实现方法→Unimplemented 应答——服务活着即绿）。
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pbv1.RegisterAdminGatewayServer(srv, &pbv1.UnimplementedAdminGatewayServer{})
	go srv.Serve(ln)
	defer srv.Stop()
	if err := Probe(sock, 3*time.Second); err != nil {
		t.Fatalf("活服务自检应绿: %v", err)
	}
}
