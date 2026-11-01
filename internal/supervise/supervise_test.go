package supervise

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mkScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func cfgFor(t *testing.T, bin string) Config {
	return Config{
		Bin:     bin,
		Args:    []string{"serve"},
		Burst:   3,
		Stable:  time.Hour, // 演练窗内稳态复位不触发
		Backoff: 10 * time.Millisecond,
		Out:     os.Stderr,
	}
}

func waitForFile(t *testing.T, p string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(p); err == nil {
			return
		}
		<-time.After(10 * time.Millisecond)
	}
	t.Fatalf("等文件 %s 超时", p)
}

// TestRollback_熔断回退prev（验收锚+对抗锚：二进制选择行——删 os.Rename
// 或翻转 prev/bin，bin 不会变成 prev 内容、prev 标记永不出现，本表必红）。
func TestRollback_熔断回退prev(t *testing.T) {
	d := t.TempDir()
	bin, prev := filepath.Join(d, "meta"), filepath.Join(d, "meta.prev")
	marker := filepath.Join(d, "prev-ran")
	mkScript(t, bin, "exit 7")
	mkScript(t, prev, "echo ran >>\""+marker+"\"\nexec sleep 300")
	prevBytes, err := os.ReadFile(prev)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { done <- Run(ctx, cfgFor(t, bin)) }()

	waitForFile(t, marker) // prev 被晋升并跑起
	cancel()
	<-done

	got, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(prevBytes) {
		t.Fatalf("熔断后 bin 非 prev 内容:\n%s", got)
	}
	if _, err := os.Stat(prev); !os.IsNotExist(err) {
		t.Fatal("prev 晋升后应消失（rename）")
	}
}

// TestFuse_无prev停机 熔断无 prev 可退→停机等外层报警（不循环重启）。
func TestFuse_无prev停机(t *testing.T) {
	d := t.TempDir()
	bin := filepath.Join(d, "meta")
	count := filepath.Join(d, "count")
	mkScript(t, bin, "echo x >>\""+count+"\"\nexit 7")

	code := Run(context.Background(), cfgFor(t, bin))
	if code == 0 {
		t.Fatal("无 prev 熔断应非零退")
	}
	b, _ := os.ReadFile(count)
	if n := strings.Count(string(b), "\n"); n != 3 { // burst=3：恰三次尝试后停机
		t.Fatalf("熔断前尝试数应为 3: %d", n)
	}
}

// TestCleanExit 子进程正常退→守护跟随退 0（systemd 不误判 failure）。
func TestCleanExit(t *testing.T) {
	d := t.TempDir()
	bin := filepath.Join(d, "meta")
	mkScript(t, bin, "exit 0")
	if code := Run(context.Background(), cfgFor(t, bin)); code != 0 {
		t.Fatalf("子进程净退守护应退 0: %d", code)
	}
}
