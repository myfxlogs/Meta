package seat

import (
	"path/filepath"
	"testing"

	"meta/internal/adapter"
)

// TestClassOfExit_席位级（票0009 锚 4）：席位级死因（2/3/4/5）→ExitWrapper；
// ①/②a/②b/0 与未识码照旧。互逆侧 ExitCodeOf(ExitWrapper)=ExitInputErr。
func TestClassOfExit_席位级(t *testing.T) {
	for _, c := range []int{ExitSeatErr, ExitOrphan, ExitAbort, ExitInputErr} {
		if got := ClassOfExit(c); got != adapter.ExitWrapper {
			t.Fatalf("ClassOfExit(%d)=%v 预期 ExitWrapper", c, got)
		}
	}
	for code, want := range map[int]adapter.ExitClass{
		ExitClean:     adapter.ExitOK,
		ExitSpawnFail: adapter.ExitSpawn,
		ExitTransient: adapter.ExitTransient,
		ExitFatal:     adapter.ExitFatal,
		1:             adapter.ExitSpawn,
		-1:            adapter.ExitSpawn,
	} {
		if got := ClassOfExit(code); got != want {
			t.Fatalf("ClassOfExit(%d)=%v 预期 %v", code, got, want)
		}
	}
	if got := ExitCodeOf(adapter.ExitWrapper); got != ExitInputErr {
		t.Fatalf("ExitCodeOf(ExitWrapper)=%d 预期 %d", got, ExitInputErr)
	}
}

// TestRun_缺env（锚 4 钦定径）：META_* 不全→ExitSeatErr（席位级——
// 网关侧归 ExitWrapper，只落 seat_err 不标壳）。
func TestRun_缺env(t *testing.T) {
	for _, k := range []string{"META_SHELL", "META_GATEWAY_SOCK", "META_SEAT_SOCK",
		"META_RUN_TOKEN", "META_TICKET_ID"} {
		t.Setenv(k, "")
	}
	if got := Run(); got != ExitSeatErr {
		t.Fatalf("Run()=%d 预期 ExitSeatErr(%d)", got, ExitSeatErr)
	}
}

// TestRunChild_输入层（锚 4）：prompt 文件不可读与适配器 Build 失败→
// codeInputErr 哨兵（run() 落成 ExitInputErr→网关 ExitWrapper）。
func TestRunChild_输入层(t *testing.T) {
	ad, err := adapter.New("claude") // claude Build 恒成，stdin 文件径必失败
	if err != nil {
		t.Fatal(err)
	}
	s := &session{spec: spec{Spec: adapter.Spec{
		Shell: "claude", PromptFile: filepath.Join(t.TempDir(), "no-such.md")}}}
	code, tail := s.runChild(ad)
	if code != codeInputErr || tail == "" {
		t.Fatalf("prompt 不可读 runChild=(%d,%q) 预期 codeInputErr 哨兵", code, tail)
	}
	// Build 失败同码径（devin 缺 PromptFile/ArtifactDir 即拒）。
	ad2, err := adapter.New("devin")
	if err != nil {
		t.Fatal(err)
	}
	s2 := &session{spec: spec{Spec: adapter.Spec{Shell: "devin"}}}
	if code2, _ := s2.runChild(ad2); code2 != codeInputErr {
		t.Fatalf("Build 失败 runChild=%d 预期 codeInputErr", code2)
	}
}
