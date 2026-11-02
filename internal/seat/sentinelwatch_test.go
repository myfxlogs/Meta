package seat

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pbv1 "meta/internal/pb/metav1"
)

// ---------- 票0045：inotify watcher 第三泵源（静默壳文件写径） ----------

// TestRun_SilentShellExecWatch（主锚·对抗提名位——删 watcher 泵调用行或
// 启动行本用例必红）：壳 stdout 全程零行，sentinel.log 写 META_EXEC:<step>
// →子进程存活期内桩网关收 ExecReq{step}（窗 ≤10s）——watcher 泵实证，
// 非 :320 终泵兜底产物（sleep 内探得即证泵在运行中）。
func TestRun_SilentShellExecWatch(t *testing.T) {
	art := t.TempDir()
	sf := filepath.Join(art, "sentinel.log")
	// 写 EXEC 哨兵→stdout 静默 sleep 6s→写 DELIVER 退场。
	stub, done := runSeatSessionAsync(t, "施工", art,
		&fakeAdapter{argv: []string{"/bin/sh", "-c",
			"printf 'META_EXEC:silentprobe\\n' >> '" + sf + "'; sleep 6; " +
				"printf 'META_DELIVER:w1\\n' >> '" + sf + "'"}})
	m := waitFrame(stub, 4*time.Second, func(m *pbv1.SeatMsg) bool {
		return m.GetExecReq() != nil
	})
	if m == nil {
		t.Fatal("静默期 ExecReq 未入桩——watcher 泵失效（泵仍挂 stdout 活性）")
	}
	if m.GetExecReq().Step != "silentprobe" {
		t.Fatalf("ExecReq.step 不符: %q", m.GetExecReq().Step)
	}
	select { // 探得时 run 未返＝子进程存活期内泵出（sleep 6 远大于 4s 窗）
	case c := <-done:
		t.Fatalf("run 已退（code=%d）——存活期实证窗不成立", c)
	default:
	}
	if code := <-done; code != ExitClean {
		t.Fatalf("文件径 DELIVER 收场 run=%d 预期 ExitClean", code)
	}
}

// TestRun_SilentShellDeliverWatch（姊妹锚）：stdout 零行＋sentinel.log 写
// META_DELIVER:ref→ExitClean＋Deliver.ref==ref——文件信道独立 stdout 活性。
func TestRun_SilentShellDeliverWatch(t *testing.T) {
	art := t.TempDir()
	sf := filepath.Join(art, "sentinel.log")
	code, msgs := runSeatSession(t, "施工", art,
		&fakeAdapter{argv: []string{"/bin/sh", "-c",
			"sleep 1; printf 'META_DELIVER:silentref\\n' >> '" + sf + "'; sleep 1"}})
	if code != ExitClean {
		t.Fatalf("静默壳文件径哨兵应 ExitClean: %d", code)
	}
	d := hasDeliver(msgs)
	if d == nil || d.Ref != "silentref" {
		t.Fatalf("Deliver.ref 不符: %+v", d)
	}
}

// TestRun_SilentShellHalfLine（半行锚）：sentinel.log 残段（无 \n）静默
// 写下不触发 ExecReq；补 \n 后恰发一次——泵 \n 收口语义零变更。
func TestRun_SilentShellHalfLine(t *testing.T) {
	art := t.TempDir()
	sf := filepath.Join(art, "sentinel.log")
	// t0 写残段→sleep 2（残段期探桩须无 ExecReq）→t2 补 \n→恰发一次→
	// t4 写 DELIVER 退场。
	stub, done := runSeatSessionAsync(t, "施工", art,
		&fakeAdapter{argv: []string{"/bin/sh", "-c",
			"printf 'META_EXEC:halfx' >> '" + sf + "'; sleep 2; " +
				"printf '\\n' >> '" + sf + "'; sleep 2; " +
				"printf 'META_DELIVER:hd\\n' >> '" + sf + "'"}})
	isExec := func(m *pbv1.SeatMsg) bool { return m.GetExecReq() != nil }
	if m := waitFrame(stub, 1500*time.Millisecond, isExec); m != nil {
		t.Fatalf("残段期不应有 ExecReq（无 \\n 不消费）: %+v", m.GetExecReq())
	}
	m := waitFrame(stub, 3*time.Second, isExec)
	if m == nil || m.GetExecReq().Step != "halfx" {
		t.Fatalf("补 \\n 后 ExecReq 未达或不符: %+v", m)
	}
	if code := <-done; code != ExitClean {
		t.Fatalf("run=%d 预期 ExitClean", code)
	}
	n := 1 // 已探得一条
	grace := time.After(300 * time.Millisecond)
count:
	for {
		select {
		case m := <-stub.got:
			if m.GetExecReq() != nil {
				n++
			}
		case <-grace:
			break count
		}
	}
	if n != 1 {
		t.Fatalf("同一哨兵行应恰发一次 ExecReq: %d", n)
	}
}

// TestRun_ConcurrentPumpRace（-race 强制面·对抗提名位）：stdout 活跃行
// （流源哨兵→pairDedup）与 sentinel.log 文件写（watcher 泵→pairDedup）
// 并发——pumpMu/pairMu 任一叶锁摘除即 -race 红。
func TestRun_ConcurrentPumpRace(t *testing.T) {
	art := t.TempDir()
	sf := filepath.Join(art, "sentinel.log")
	code, _ := runSeatSession(t, "施工", art,
		&fakeAdapter{argv: []string{"/bin/sh", "-c",
			"for i in $(seq 1 40); do echo META_EXEC:race$i; " +
				"printf 'META_ADMIN:set_project|meta\\n' >> '" + sf + "'; done; " +
				"echo META_DELIVER:rr"}})
	if code != ExitClean {
		t.Fatalf("并发泵径 run=%d 预期 ExitClean", code)
	}
}

// TestRun_WatchDegrade（降级锚）：ArtifactDir 指向不存在目录→AddWatch 败
// →watcher 不起＋stderr 留痕；stdout 哨兵照常即泵——既有两泵语义不变
// （ArtifactDir=="" 面由既有零 artDir 用例群覆盖）。
func TestRun_WatchDegrade(t *testing.T) {
	art := filepath.Join(t.TempDir(), "nonexist")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	code, msgs := runSeatSession(t, "施工", art,
		&fakeAdapter{argv: []string{"/bin/sh", "-c",
			"echo META_DELIVER:dref; exit 0"}})
	_ = w.Close()
	os.Stderr = old
	out, _ := io.ReadAll(r)
	if code != ExitClean {
		t.Fatalf("watcher 降级席应照常 ExitClean: %d", code)
	}
	if d := hasDeliver(msgs); d == nil || d.Ref != "dref" {
		t.Fatalf("stdout 哨兵径不受降级影响: %+v", d)
	}
	if !strings.Contains(string(out), "sentinel watch") {
		t.Fatalf("watch init 败 stderr 无痕: %q", out)
	}
}
