package gateway

import (
	"testing"

	"meta/internal/db"
)

// 自捕率分类器锚用例（票0038-p1 §6 ↔ 设计-0038 §5 锚映射）——
// fixture 直喂不触 DB；行序按 CatchRows 契约 id DESC 手排。

func cev(id int64, tid, kind, note string) *db.Event {
	return &db.Event{ID: id, TicketID: tid, Kind: kind, Note: note}
}

// 锚1：注入一拒判→分类输出必变且方向正确（分母+1/率降）。
func TestClassifyCatch_RejectShiftsRate(t *testing.T) {
	rows := []*db.Event{
		cev(2, "t1", "deliver", "abc123 坑档：docs/经验库/x.md"),
	}
	before := classifyCatch(rows).line()
	if before != "100%（近 1 判）" {
		t.Fatalf("单自捕判例率错: %s", before)
	}
	rows = append([]*db.Event{cev(3, "t2", "reject", "reject 理由")}, rows...)
	v := classifyCatch(rows)
	if v.total != 2 || v.self != 1 || v.line() != "50%（近 2 判）" {
		t.Fatalf("拒判注入后率未降: %+v line=%s", v, v.line())
	}
	t.Logf("演练对照：拒判前行「%s」→ 拒判后行「%s」", before, v.line())
}

// 锚2：同 fixture 两次调用→输出恒等（事件重放幂等——构造保证）。
func TestClassifyCatch_ReplayIdem(t *testing.T) {
	rows := []*db.Event{
		cev(5, "t1", "reject", "驳回"),
		cev(4, "t1", "deliver", "abc 坑档：a.md"),
		cev(3, "patrol-9", "created", ""),
		cev(2, "t1", "deploy_failed", "META_DEPLOY:failed"),
		cev(1, "t3", "deliver", "def 坑档：b.md"),
	}
	a, b := classifyCatch(rows), classifyCatch(rows)
	if a != b || a.line() != b.line() {
		t.Fatalf("重放不幂等: %+v vs %+v", a, b)
	}
}

// 锚3：空判例 fixture→「暂无」不显示 0%；非判例行（audit_pass/
// 无标 progress/非 patrol- created）同不入分母。
func TestClassifyCatch_Empty(t *testing.T) {
	if got := classifyCatch(nil).line(); got != "暂无" {
		t.Fatalf("空判例非暂无: %s", got)
	}
	rows := []*db.Event{
		cev(4, "t1", "audit_pass", "pass ref"),
		cev(3, "t1", "progress", "写码 落表"),
		cev(2, "0038-p1", "created", ""), // 非 patrol- 前缀自立案不计（口径偏差写死）
		cev(1, "t1", "deliver", "abc 交付摘要"),
	}
	v := classifyCatch(rows)
	if v.total != 0 || v.line() != "暂无" {
		t.Fatalf("非判例行混入分母: %+v", v)
	}
}

// 锚4：坑档标签＋同票后驳→该坑档失格不计（整行出分母——拒判自身
// 已计外捕一判）；reject 后再报新路径→计；他票驳不误伤。
func TestClassifyCatch_Disqualify(t *testing.T) {
	rows := []*db.Event{
		cev(4, "t1", "reject", "驳回"),
		cev(3, "t1", "deliver", "abc 坑档：a.md"), // 后驳失格
		cev(2, "t2", "deliver", "def 坑档：b.md"), // 他票无驳→计
	}
	v := classifyCatch(rows)
	if v.total != 2 || v.self != 1 {
		t.Fatalf("失格判错: %+v", v)
	}
	// 驳后新径再报→计（同票失格不连坐新报）。
	rows = append([]*db.Event{cev(5, "t1", "deliver", "ghi 坑档：c.md")}, rows...)
	if v = classifyCatch(rows); v.total != 3 || v.self != 2 {
		t.Fatalf("驳后新径未计: %+v", v)
	}
	// 夹在两驳间的标签：最新驳仍失格（rejected 须保最大 id）。
	rows = []*db.Event{
		cev(6, "t1", "reject", "再驳"),
		cev(5, "t1", "deliver", "ghi 坑档：d.md"),
		cev(4, "t1", "reject", "先驳"),
	}
	if v = classifyCatch(rows); v.total != 2 || v.self != 0 {
		t.Fatalf("两驳夹标未失格: %+v", v)
	}
}

// 锚5：同 (ticket_id,坑档路径) 两行→只计一次（按最新行）；同径
// 驳后重报→最新行在驳后→计。
func TestClassifyCatch_Dedup(t *testing.T) {
	rows := []*db.Event{
		cev(5, "t1", "deliver", "new 坑档：a.md"),
		cev(4, "t1", "reject", "驳回"),
		cev(3, "t1", "deliver", "old 坑档：a.md"), // 同径去重——最新行已纳
	}
	v := classifyCatch(rows)
	if v.total != 2 || v.self != 1 {
		t.Fatalf("同径双报未去重: %+v", v)
	}
	// 多坑逗号列：一行两径→两判（中英文逗号同分列）。
	rows = []*db.Event{
		cev(2, "t1", "deliver", "abc 坑档：a.md,b.md"),
		cev(1, "t1", "deliver", "def 坑档：c.md，d.md"),
	}
	if v = classifyCatch(rows); v.total != 4 || v.self != 4 {
		t.Fatalf("多坑逗号列分列错: %+v", v)
	}
}

// 锚6 映射面：外捕三类各命中一判（删任一映射行→本条必红）；
// progress 携坑档标签同纳（kind 不拘——deliver/adjunct_deliver/
// progress 均入自捕径）。
func TestClassifyCatch_ExtKinds(t *testing.T) {
	rows := []*db.Event{
		cev(5, "t1", "progress", "自测 坑档：e.md"),
		cev(4, "patrol-7", "created", ""),
		cev(3, "t2", "deploy_failed", "META_DEPLOY:failed 回滚"),
		cev(2, "t1", "reject", "驳回"),
		cev(1, "t3", "created", ""), // 非 patrol-→非判例
	}
	v := classifyCatch(rows)
	if v.total != 4 || v.self != 1 {
		t.Fatalf("外捕三类映射错: %+v", v)
	}
}

// 窗口钦定 20：并集超窗按 id DESC 截（最老一判出窗）。
func TestClassifyCatch_WindowCap(t *testing.T) {
	rows := []*db.Event{cev(1, "t1", "deliver", "老 坑档：old.md")}
	for i := 2; i <= 21; i++ {
		rows = append([]*db.Event{cev(int64(i), "t2", "reject", "驳回")}, rows...)
	}
	v := classifyCatch(rows)
	if v.total != catchWindow || v.self != 0 {
		t.Fatalf("窗口未截 20: %+v", v)
	}
}
