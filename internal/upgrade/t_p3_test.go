package upgrade

import (
	"fmt"
	"os"
	"io"
	"testing"
	"time"
)

// TestP3_CheckCmd等死再探 反向锚①：桩「旧口先活后灭→新口起」——checker
// 判绿走的是新口（等死段不过早判绿）。
func TestP3_CheckCmd等死再探(t *testing.T) {
	old := probeFn
	defer func() { probeFn = old }()
	calls := 0
	probeFn = func(sock string, timeout time.Duration) error {
		calls++
		if calls <= 2 {
			return nil // 旧口还活（前两探）
		}
		if calls == 3 {
			return fmt.Errorf("dead") // 旧口灭（restart 实发）
		}
		return nil // 新口起
	}
	if err := CheckCmd(io.Discard, "s", "p", "i", "", false); err != nil {
		t.Fatalf("新口起应判绿: %v", err)
	}
	if calls < 4 {
		t.Fatalf("未等死直接探绿（旧口误判）: calls=%d", calls)
	}
}

// TestP3_CheckCmd旧灭新不起 反向锚②：桩「旧灭新不起」→回退 prev 分支
// （copyAtomic 实证：install 位被 prev 覆写）。
func TestP3_CheckCmd旧灭新不起(t *testing.T) {
	old := probeFn
	defer func() { probeFn = old }()
	probeFn = func(string, time.Duration) error { return fmt.Errorf("dead") }
	dir := t.TempDir()
	prev, install := dir+"/p", dir+"/i"
	if err := os.WriteFile(prev, []byte("old-bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CheckCmd(io.Discard, "s", prev, install, "true", true); err == nil {
		t.Fatal("自检红应退错")
	}
}
