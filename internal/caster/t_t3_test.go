package caster

import "testing"

// TestT3_中文映射覆盖 D-65 锚：票面点名的漏网动作全有中文定性；
// 映射回原文即漏（违决策23/48——业主不看英文术语）。
func TestT3_中文映射覆盖(t *testing.T) {
	for _, k := range []string{
		"adjunct_dead", "adjunct_yield", "yield", "seat_err", "seat_dead",
		"seat_up", "void", "requeue", "exec_denied", "deploy_failed",
		"merge_conflict", "spawn_blocked", "inject_failed", "dispatch_failed",
		"filter_degraded", "audit_dead", "design_dead", "msg", "patrol_tick",
		"shell_limited", "shell_dead", "shell_recovered", "shells_degraded",
	} {
		if got := actionZh(k); got == k {
			t.Fatalf("kind=%s 无中文定性（映射漏网）", k)
		}
	}
}
