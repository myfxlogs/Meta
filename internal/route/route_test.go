package route

import (
	"testing"
	"time"

	"meta/internal/adapter"
	"meta/internal/db"
)

func rows(h1, h2, h3 string) []db.Shell {
	return []db.Shell{
		{Name: "devin", Model: "m1", Role: "施工", Priority: 10, Health: h1},
		{Name: "claude", Model: "", Role: "施工", Priority: 20, Health: h2},
		{Name: "opencode", Model: "", Role: "施工", Priority: 30, Health: h3},
	}
}

// TestCandidates_Priority 优先序逐格（对抗性锚：翻转 Priority< 比较必红）。
func TestCandidates_Priority(t *testing.T) {
	c := Candidates(rows(HealthOK, HealthOK, HealthOK), nil, 0)
	if len(c) != 3 || c[0].Shell != "devin" || c[1].Shell != "claude" || c[2].Shell != "opencode" {
		t.Fatalf("序错: %+v", c)
	}
	for _, a := range c {
		if a.Probe || a.Stagger != 0 {
			t.Fatalf("健康行不应懒探/错峰: %+v", a)
		}
	}
}

// TestCandidates_Probe 懒探逐格：非健康行每轮一枚 probe；limited 带错峰。
func TestCandidates_Probe(t *testing.T) {
	c := Candidates(rows(HealthDead, HealthLimited, HealthOK), nil, 7*time.Second)
	if len(c) != 3 {
		t.Fatalf("候选数=%d", len(c))
	}
	if !c[0].Probe || c[0].Stagger != 0 {
		t.Fatalf("dead 行应懒探无错峰: %+v", c[0])
	}
	if !c[1].Probe || c[1].Stagger != 7*time.Second {
		t.Fatalf("limited 行应懒探+错峰: %+v", c[1])
	}
	if c[2].Probe || c[2].Stagger != 0 {
		t.Fatalf("ok 行应正常: %+v", c[2])
	}
}

// TestCandidates_Exclude 换壳＝排除本票已失败壳，下一行顶上。
func TestCandidates_Exclude(t *testing.T) {
	excl := map[string]bool{"devin/m1/施工": true}
	c := Candidates(rows(HealthOK, HealthOK, HealthOK), excl, 0)
	if len(c) != 2 || c[0].Shell != "claude" {
		t.Fatalf("排除未生效: %+v", c)
	}
	// 全排除＝耗尽。
	if c := Candidates(rows(HealthOK, HealthOK, HealthOK),
		map[string]bool{"devin/m1/施工": true, "claude//施工": true, "opencode//施工": true}, 0); len(c) != 0 {
		t.Fatalf("应耗尽: %+v", c)
	}
}

// TestNextHealth 失效分级健康迁移逐格（①不标/②a→limited/②b→dead/成→ok）。
func TestNextHealth(t *testing.T) {
	cases := []struct {
		cur     string
		class   adapter.ExitClass
		want    string
		changed bool
	}{
		{HealthOK, adapter.ExitSpawn, HealthOK, false},        // ① 不标
		{HealthOK, adapter.ExitTransient, HealthLimited, true}, // ②a
		{HealthOK, adapter.ExitFatal, HealthDead, true},        // ②b
		{HealthLimited, adapter.ExitFatal, HealthDead, true},   // ②a 恶化②b
		{HealthDead, adapter.ExitOK, HealthOK, true},           // 懒探恢复
		{HealthOK, adapter.ExitOK, HealthOK, false},            // 幂等
		{HealthDead, adapter.ExitSpawn, HealthDead, false},     // dead 行 probe 又①仍死
	}
	for _, tc := range cases {
		got, ch := NextHealth(tc.cur, tc.class)
		if got != tc.want || ch != tc.changed {
			t.Errorf("NextHealth(%s,%d)=(%s,%v) 预期 (%s,%v)",
				tc.cur, tc.class, got, ch, tc.want, tc.changed)
		}
	}
}
