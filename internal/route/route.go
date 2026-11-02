// Package route 是壳路由的纯函数核（§7）：登记表按优先序展开 spawn
// 尝试序，健康态决定正常尝试还是懒探；失效分级①②a②b决定健康态迁移。
// 全部机械判——语义裁决（该不该换/何时恢复）不归本包。
package route

import (
	"sort"
	"time"

	"meta/internal/adapter"
	"meta/internal/db"
)

// 健康态值（shell.health 列）。
const (
	HealthOK      = "ok"
	HealthLimited = "limited" // ②a 限流/瞬态
	HealthDead    = "dead"    // ②b 配额尽/授权/持续错
	HealthUnknown = "unknown"
)

// Attempt 一次 spawn 轮里的一枚候选。
type Attempt struct {
	Shell   string
	Model   string
	Probe   bool          // 非健康行的懒探尝试（§7：下次需要时试一次）
	Stagger time.Duration // ②a 错峰：limited 行须错峰 spawn
}

// Candidates 把角色登记行展开成 spawn 尝试序：
//   - 健康行（ok/unknown）＝正常尝试——按 priority 序轮转（rotate 位起
//     循环，T20 健康加权分摊：同角色多壳均摊不再恒吃首选；单壳瘸自动
//     让位——非健康行排尾懒探）；
//   - 非健康行＝懒探（每轮每行至多一枚；探成即由调用方复标 ok）；
//   - exclude 行键（本票已失败的壳）整行跳过——换壳=下一行（票级重做）；
//   - limited 行的尝试带 Stagger——②a 限流错峰。
// rotate<0 退回 priority 静态序（兼容旧调用面）。
func Candidates(rows []db.Shell, exclude map[string]bool, stagger time.Duration, rotate int) []Attempt {
	sorted := append([]db.Shell(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})
	var healthy, probes []db.Shell
	for _, r := range sorted {
		if exclude[r.ShellKey()] {
			continue
		}
		switch r.Health {
		case "", HealthOK, HealthUnknown:
			healthy = append(healthy, r)
		default:
			probes = append(probes, r)
		}
	}
	if rotate >= 0 && len(healthy) > 1 {
		rotate %= len(healthy)
		healthy = append(healthy[rotate:], healthy[:rotate]...)
	}
	var out []Attempt
	for _, r := range healthy {
		out = append(out, Attempt{Shell: r.Name, Model: r.Model})
	}
	for _, r := range probes {
		a := Attempt{Shell: r.Name, Model: r.Model, Probe: true}
		if r.Health == HealthLimited {
			a.Stagger = stagger
		}
		out = append(out, a)
	}
	return out
}

// NextHealth 失效分级→健康态迁移（§7）：
//   - ExitSpawn（①起不来/秒退/外部杀死）→ 不变——机器级失效不标壳；
//   - ExitTransient（②a）→ limited；
//   - ExitFatal（②b）→ dead；
//   - ExitOK（跑成）→ ok（懒探恢复同径）。
//
// 返回 (新态, 是否变化)。
func NextHealth(cur string, class adapter.ExitClass) (string, bool) {
	if cur == "" {
		cur = HealthUnknown
	}
	switch class {
	case adapter.ExitTransient:
		return HealthLimited, cur != HealthLimited
	case adapter.ExitFatal:
		return HealthDead, cur != HealthDead
	case adapter.ExitOK:
		return HealthOK, cur != HealthOK
	}
	return cur, false // ① 不迁移
}
