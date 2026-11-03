package gateway

import (
	"fmt"
	"strings"
	"time"

	"meta/internal/db"
)

// 自捕率判例分类器（票0038-p1/设计-0038 §2 口径机械表）——纯函数：
// CatchRows 候选行（id DESC）→ 近 catchWindow 判窗口统计。零时钟零
// 快照零写（R2/D-56）；同 rows 重放→同输出（幂等由构造保证，锚2）。

// catchWindow 判例窗口（票面钦定「近 20 判」——外捕∪自捕并集按
// event id DESC 取 20 为分母）。
const catchWindow = 20

// catchScanLimit CatchRows 扫描上限：候选行数上限（非判例数——
// 每票一条 created 稀释行占额）。卡视图每次装配一趟有界读。
const catchScanLimit = 1024

// extCatchRule 外捕判例映射表（设计 §2——锚6 对抗提名面：删任一
// 映射行→对应用例必红）。kind→谓词（ticketID 参判）：
//   - reject：审计驳回
//   - deploy_failed：部署失败
//
// T34：created→patrol-* 外捕行随巡检立案径切除（P5 下席位不立票）。
var extCatchRule = map[string]func(ticketID string) bool{
	"reject":        func(string) bool { return true },
	"deploy_failed": func(string) bool { return true },
}

// pitTag 坑档自报标签（设计 §4 同生标签约定——META_DELIVER ref 自由
// 文本/部署支 Deliver Ref 位携带，落 ticket_event.note；kind 不拘）。
const pitTag = "坑档："

// catchVerdict 判例窗口统计输出。
type catchVerdict struct {
	total int // 窗口内判例数（分母）
	self  int // 其中自捕判例数（分子）
}

// classifyCatch 候选行（id DESC 序，CatchRows 保证）→ 窗口统计。
// 逐行机械分类：外捕映射命中即计一判；「坑档：」标签按 (票,路径)
// 去重取最新行、同票后驳失格（失格行整行不计——拒判自身已作外捕
// 计一判，设计 §2「不计」直读）。窗口满即止。
func classifyCatch(rows []*db.Event) catchVerdict {
	rejected := map[string]int64{} // tid → 已见最新 reject 行 id（DESC 首见即最大）
	seenPit := map[string]bool{}   // tid\x00path → 已纳（同票同径多行去重，首见＝最新行）
	var v catchVerdict
	for _, e := range rows {
		if v.total >= catchWindow {
			break
		}
		if rule := extCatchRule[e.Kind]; rule != nil && rule(e.TicketID) {
			v.total++
			// 同票多驳须保最大 id（DESC 后到的是更老的驳——无遮则
			// 夹在两驳间的标签会误判有效）。
			if e.Kind == "reject" && rejected[e.TicketID] < e.ID {
				rejected[e.TicketID] = e.ID
			}
		}
		for _, p := range pitPaths(e.Note) {
			key := e.TicketID + "\x00" + p
			if seenPit[key] {
				continue
			}
			seenPit[key] = true
			if e.ID < rejected[e.TicketID] {
				continue // 同票后驳失格——不计（锚4）
			}
			v.total++
			v.self++
		}
	}
	return v
}

// line 卡面行内文（预格式化）：零判例→「暂无」（锚3：不显示 0%）；
// 有判例→「N%（近 M 判）」，M＝实纳判例数（窗口未填满时如实示 M，
// 不写死 20——窗口上限在卡附注口径）。
func (v catchVerdict) line() string {
	if v.total == 0 {
		return "暂无"
	}
	return fmt.Sprintf("%d%%（近 %d 判）", v.self*100/v.total, v.total)
}

// pitPaths 坑档路径抽取：note 中每处「坑档：」后取至空白/行尾的段，
// 中英文逗号分列多径；段尾句读剥除（防「坑档：x.md。」吃标点成另径）。
func pitPaths(note string) []string {
	var out []string
	rest := note
	for {
		i := strings.Index(rest, pitTag)
		if i < 0 {
			return out
		}
		seg := rest[i+len(pitTag):]
		if j := strings.IndexAny(seg, " \t\n\r"); j >= 0 {
			seg = seg[:j]
		}
		for _, p := range strings.FieldsFunc(seg, func(r rune) bool {
			return r == ',' || r == '，'
		}) {
			if p = strings.TrimRight(strings.TrimSpace(p), "。；;、）)"); p != "" {
				out = append(out, p)
			}
		}
		rest = rest[i+len(pitTag):]
	}
}

// MonthlyCatch 月聚合行（T23 月报件）：月份/判例数/自捕数——同一
// extCatchRule/pitTag 口径按月分桶。
type MonthlyCatch struct {
	Month string
	Total int
	Self  int
}

// classifyMonthly 全量候选行→按 created_at 月分桶聚合（逐月窗口
// 不设 20 判帽——月报是全量口径，与卡面「近20判」并列不替）。
func classifyMonthly(rows []*db.Event, now func(time.Time) string) []MonthlyCatch {
	byMonth := map[string]*MonthlyCatch{}
	var order []string
	bucket := func(e *db.Event) *MonthlyCatch {
		m := now(e.CreatedAt) // 注入缝：测试钉死
		if byMonth[m] == nil {
			byMonth[m] = &MonthlyCatch{Month: m}
			order = append(order, m)
		}
		return byMonth[m]
	}
	rejected := map[string]int64{}
	seenPit := map[string]bool{}
	for _, e := range rows {
		mc := bucket(e)
		if rule := extCatchRule[e.Kind]; rule != nil && rule(e.TicketID) {
			mc.Total++
			if e.Kind == "reject" && rejected[e.TicketID] < e.ID {
				rejected[e.TicketID] = e.ID
			}
		}
		for _, p := range pitPaths(e.Note) {
			key := e.TicketID + "\x00" + p
			if seenPit[key] {
				continue
			}
			seenPit[key] = true
			if e.ID < rejected[e.TicketID] {
				continue
			}
			mc.Total++
			mc.Self++
		}
	}
	out := make([]MonthlyCatch, 0, len(order))
	for i := len(order) - 1; i >= 0; i-- {
		out = append(out, *byMonth[order[i]])
	}
	return out
}
