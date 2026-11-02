package feishu

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// cardCap 伪 sender/伪库捕获面（票0014 锚 2-6 捕器）：视图/卡行/对账面
// 全注入，create/patch/记错全留痕。
type cardCap struct {
	mu        sync.Mutex
	views     map[string]*CardView // 伪库票视图
	rows      map[string]string    // feishu_card 伪表：tid→message_id
	nonTerm   []string             // 伪库非终态票列
	creates   []string             // 每次 create 的 chatID
	patches   []string             // 每次 patch 的 message_id
	patchBody []string             // 每次 patch 的卡内容（票0036 显示值断言面）
	errNotes  []string             // RecordErr 捕获 "tid|note"
	createErr error
	createN   int
}

func (c *cardCap) create(_ context.Context, chatID, _ string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.createN++
	if c.createErr != nil {
		return "", c.createErr
	}
	c.creates = append(c.creates, chatID)
	return fmt.Sprintf("om_%d", c.createN), nil
}

func (c *cardCap) patch(_ context.Context, messageID, content string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.patches = append(c.patches, messageID)
	c.patchBody = append(c.patchBody, content)
	return nil
}

func (c *cardCap) counts() (creates, patches int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.creates), len(c.patches)
}

func newCap() *cardCap {
	return &cardCap{views: map[string]*CardView{}, rows: map[string]string{}}
}

func newTestCards(cap *cardCap) *Cards {
	return NewCards(CardConfig{
		Cred: testCred(),
		View: func(_ context.Context, tid string) (*CardView, error) {
			return cap.views[tid], nil
		},
		CardRow: func(_ context.Context, tid string) (string, bool, error) {
			mid, ok := cap.rows[tid]
			return mid, ok, nil
		},
		SaveCardRow: func(_ context.Context, tid, mid, _ string) error {
			cap.rows[tid] = mid
			return nil
		},
		NonTerminal: func(context.Context) ([]string, error) {
			return cap.nonTerm, nil
		},
		RecordErr: func(_ context.Context, tid, note string) {
			cap.errNotes = append(cap.errNotes, tid+"|"+note)
		},
		Create: cap.create,
		Patch:  cap.patch,
		Logf:   func(string, ...any) {},
	})
}

// testView 固定渲染输入（At 固定——时间随输入不随墙钟，哈希才可判）。
// 票0036 三件同步固定：RenderAt=At、PhaseSince=At−12m（非终态恒
// 「已 12 分」）、CreatedAt=At−2h（终态恒「总 1.8 时」）。
func testView(tid, phase, stage, note string) *CardView {
	at := time.Unix(1762000000, 0)
	return &CardView{
		TicketID: tid, Title: "标题甲", Phase: phase,
		Role: "施工", Shell: "devin", Stage: stage, Note: note,
		Used: 100, UsedTotal: 250, Rejects: 1,
		Terminal: phase == "完成" || phase == "作废",
		At:         at,
		PhaseSince: at.Add(-12 * time.Minute),
		RenderAt:   at,
		CreatedAt:  at.Add(-2 * time.Hour),
	}
}

// TestObserve_LazyCreate 锚2＋对抗锚 a：事件序列→create 一次＋
// update N 次且同 message_id（复用行删＝每事件新卡→本条红）。
func TestObserve_LazyCreate(t *testing.T) {
	cap := newCap()
	cap.views["t1"] = testView("t1", "已派", "", "")
	cards := newTestCards(cap)
	ctx := context.Background()

	cards.Observe(ctx, "claim", "t1") // 首见非终态→懒创建
	cap.views["t1"].Phase = "施工中"
	cap.views["t1"].Note = "调查 读码"
	cards.Observe(ctx, "progress", "t1") // 内容变→PATCH 同卡
	cap.views["t1"].Note = "写码 落表"
	cards.Observe(ctx, "progress", "t1") // 再变→再 PATCH 同卡

	creates, patches := cap.counts()
	if creates != 1 || patches != 2 {
		t.Fatalf("create/update 次数错: create=%d patch=%d", creates, patches)
	}
	mid := cap.rows["t1"]
	if mid == "" {
		t.Fatalf("message_id 未落行: %v", cap.rows)
	}
	for i, p := range cap.patches {
		if p != mid {
			t.Fatalf("PATCH 打到了别的卡: patches[%d]=%s want %s", i, p, mid)
		}
	}
	if cap.creates[0] != testCred().ChatId {
		t.Fatalf("卡落点非 owner chat: %v", cap.creates)
	}
}

// TestObserve_HashGateUsage 锚3＋对抗锚 b：同内容二帧→零 PATCH
// （哈希判断行翻＝内容不变也 PATCH→本条红）；usage 行→零调用；
// 空票号事件→零调用。
func TestObserve_HashGateUsage(t *testing.T) {
	cap := newCap()
	cap.views["t1"] = testView("t1", "施工中", "写码", "写码 落表")
	cards := newTestCards(cap)
	ctx := context.Background()

	cards.Observe(ctx, "claim", "t1")    // create
	cards.Observe(ctx, "progress", "t1") // 同内容→哈希闸挡 PATCH
	cards.Observe(ctx, "progress", "t1") // 再同内容→仍零 PATCH
	creates, patches := cap.counts()
	if creates != 1 || patches != 0 {
		t.Fatalf("哈希闸失效: create=%d patch=%d", creates, patches)
	}

	// usage 行与无票事件：零调用（出站不增）。
	cards.Observe(ctx, "usage", "t1")
	cards.Observe(ctx, "feishu_inbound", "")
	if c, p := cap.counts(); c != 1 || p != 0 {
		t.Fatalf("usage/无票事件触发了出站: create=%d patch=%d", c, p)
	}

	// 内容真变→放行一拍。
	cap.views["t1"].Note = "自测 跑锚"
	cards.Observe(ctx, "progress", "t1")
	if _, p := cap.counts(); p != 1 {
		t.Fatalf("内容变未 PATCH: patch=%d", p)
	}
}

// TestReconcile 锚4：启动对账——伪库两票非终态无卡→两 create；
// 已有卡票（对账前手工落行）不重建只按闸刷新。
func TestReconcile(t *testing.T) {
	cap := newCap()
	cap.views["t1"] = testView("t1", "施工中", "写码", "写码")
	cap.views["t2"] = testView("t2", "待审", "", "")
	cap.nonTerm = []string{"t1", "t2"}
	cards := newTestCards(cap)

	cards.Reconcile(context.Background())
	creates, _ := cap.counts()
	if creates != 2 {
		t.Fatalf("启动对账补建数错: create=%d want 2", creates)
	}
	if cap.rows["t1"] == "" || cap.rows["t2"] == "" {
		t.Fatalf("对账后卡行未落: %v", cap.rows)
	}
}

// TestObserve_CreateFailNonBlocking 锚5：create 败→事件行（RecordErr
// 留痕）＋主流程不炸（Observe 正常返回，后续事件照走重试）。
func TestObserve_CreateFailNonBlocking(t *testing.T) {
	cap := newCap()
	cap.views["t1"] = testView("t1", "已派", "", "")
	cap.createErr = errors.New("lark 限频 429")
	cards := newTestCards(cap)
	ctx := context.Background()

	cards.Observe(ctx, "claim", "t1") // 主流程不炸——能回来即证
	if len(cap.errNotes) != 1 || !strings.Contains(cap.errNotes[0], "t1|") {
		t.Fatalf("create 败未记事件: %v", cap.errNotes)
	}
	if cap.rows["t1"] != "" {
		t.Fatalf("败局落了卡行: %v", cap.rows)
	}

	// 下个事件重试；恢复后建卡成功（懒修）。
	cap.createErr = nil
	cards.Observe(ctx, "progress", "t1")
	creates, _ := cap.counts()
	if creates != 1 || cap.rows["t1"] == "" {
		t.Fatalf("败后重试未建卡: creates=%d rows=%v", creates, cap.rows)
	}
}

// TestObserve_TerminalNoCreate 懒创建闸：终态票无卡不追建
// （重启后遇完成帧不补发新卡——非终态才补）。
func TestObserve_TerminalNoCreate(t *testing.T) {
	cap := newCap()
	cap.views["t1"] = testView("t1", "完成", "", "")
	cards := newTestCards(cap)

	cards.Observe(context.Background(), "deploy_done", "t1")
	if c, p := cap.counts(); c != 0 || p != 0 {
		t.Fatalf("终态票建了卡: create=%d patch=%d", c, p)
	}
}

// TestPhaseElapsed_Show 票0036 锚2：相位行耗时后缀存在——
// 施工中＋PhaseSince=T−12min＋RenderAt=T→「施工中（已 12 分）」。
func TestPhaseElapsed_Show(t *testing.T) {
	if s := renderCard(testView("t1", "施工中", "", "")); !strings.Contains(s, "施工中（已 12 分）") {
		t.Fatalf("相位行缺耗时后缀: %s", s)
	}
}

// TestObserve_PhaseChangePatch 票0036 锚3（CEO 锚a）：相位变迁→
// 卡内容 hash 必变→PATCH 恰 1 次且同 message_id。
func TestObserve_PhaseChangePatch(t *testing.T) {
	cap := newCap()
	cap.views["t1"] = testView("t1", "已派", "", "")
	cards := newTestCards(cap)
	ctx := context.Background()

	cards.Observe(ctx, "claim", "t1") // 首见→懒创建
	cap.views["t1"].Phase = "施工中"
	cap.views["t1"].PhaseSince = cap.views["t1"].RenderAt.Add(-time.Minute)
	cards.Observe(ctx, "transition", "t1") // 相位变→PATCH 同卡

	creates, patches := cap.counts()
	if creates != 1 || patches != 1 {
		t.Fatalf("相位变迁未触 PATCH: create=%d patch=%d", creates, patches)
	}
	if cap.patches[0] != cap.rows["t1"] {
		t.Fatalf("PATCH 打到了别的卡: %s want %s", cap.patches[0], cap.rows["t1"])
	}
	if !strings.Contains(cap.patchBody[0], "施工中（已 1 分）") {
		t.Fatalf("PATCH 内容缺相位耗时: %s", cap.patchBody[0])
	}
}

// TestObserve_ElapsedSnapshot 票0036 锚4（CEO 锚b）：耗时单调但重渲才
// 更新——同 view（RenderAt 不变）连帧→零 PATCH；仅 RenderAt 前进→PATCH
// 恰 1 次且 floor 值正确；RenderAt 更大→显示值不减（单调断言）。
func TestObserve_ElapsedSnapshot(t *testing.T) {
	cap := newCap()
	cap.views["t1"] = testView("t1", "施工中", "", "")
	cards := newTestCards(cap)
	ctx := context.Background()

	cards.Observe(ctx, "claim", "t1")    // create（已 12 分）
	cards.Observe(ctx, "progress", "t1") // 同 RenderAt→同内容→零 PATCH
	cards.Observe(ctx, "progress", "t1")
	if _, p := cap.counts(); p != 0 {
		t.Fatalf("同快照重渲触发 PATCH: patch=%d", p)
	}
	base := cap.views["t1"].RenderAt
	cap.views["t1"].RenderAt = base.Add(10 * time.Minute) // 已 22 分
	cards.Observe(ctx, "progress", "t1")
	creates, patches := cap.counts()
	if creates != 1 || patches != 1 {
		t.Fatalf("RenderAt 前进未 PATCH: create=%d patch=%d", creates, patches)
	}
	if !strings.Contains(cap.patchBody[0], "（已 22 分）") {
		t.Fatalf("floor 显示值错: %s", cap.patchBody[0])
	}
	cap.views["t1"].RenderAt = base.Add(11 * time.Minute) // 已 23 分——单调不减
	cards.Observe(ctx, "progress", "t1")
	if _, p := cap.counts(); p != 2 {
		t.Fatalf("RenderAt 再前进未 PATCH: patch=%d", p)
	}
	if !strings.Contains(cap.patchBody[1], "（已 23 分）") {
		t.Fatalf("单调断言败（显示值应≥22 分）: %s", cap.patchBody[1])
	}
}

// TestPhaseElapsed_Format 票0036 锚5：格式边界表——<60min「已 N 分」
// （floor）；≥60min「已 X.Y 时」；负 d 钳 0；PhaseSince 零值→裸相位行。
func TestPhaseElapsed_Format(t *testing.T) {
	base := time.Unix(1762000000, 0)
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"59分尾", 59*time.Minute + 59*time.Second, "施工中（已 59 分）"},
		{"60分整", 60 * time.Minute, "施工中（已 1.0 时）"},
		{"72分", 72 * time.Minute, "施工中（已 1.2 时）"},
		{"负钳零", -5 * time.Minute, "施工中（已 0 分）"},
	}
	for _, c := range cases {
		v := testView("t1", "施工中", "", "")
		v.PhaseSince = base
		v.RenderAt = base.Add(c.d)
		if s := renderCard(v); !strings.Contains(s, c.want) {
			t.Fatalf("%s: want %q: %s", c.name, c.want, s)
		}
	}
	v := testView("t1", "施工中", "", "")
	v.PhaseSince = time.Time{}
	if s := renderCard(v); strings.Contains(s, "施工中（") {
		t.Fatalf("PhaseSince 零值应裸相位行: %s", s)
	}
}

// TestPhaseElapsed_Terminal 票0036 锚6（对抗锚 c 提名）：终态总时长——
// 完成＋Terminal＋CreatedAt/PhaseSince 定差→「（总 X）」（终态分支翻用
// RenderAt 者在此红）；同终态 view 换 RenderAt 逐字节同（固定快照→
// 后续重渲零 PATCH）。
func TestPhaseElapsed_Terminal(t *testing.T) {
	v := testView("t1", "完成", "", "")
	v.CreatedAt = time.Unix(1762000000, 0)
	v.PhaseSince = v.CreatedAt.Add(2*time.Hour + 18*time.Minute)
	s1 := renderCard(v)
	if !strings.Contains(s1, "完成（总 2.3 时）") {
		t.Fatalf("终态缺总时长: %s", s1)
	}
	v.RenderAt = v.RenderAt.Add(3 * time.Hour)
	if s2 := renderCard(v); s2 != s1 {
		t.Fatalf("终态卡随 RenderAt 漂移（应固定快照）:\n%s\n%s", s1, s2)
	}
}

// TestRenderFace 卡面六要素全在 JSON 内（渲染器自检——票号/标题/相位/
// 席 role·shell·stage/进度/用量/驳回/时间）。
func TestRenderFace(t *testing.T) {
	s := renderCard(testView("0014", "施工中", "写码", "写码 落表"))
	for _, want := range []string{
		"票 0014", "标题甲", "施工中", "施工·devin（写码）",
		"写码 落表", "100（累计 250）", "**驳回**：1",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("卡面缺要素 %q: %s", want, s)
		}
	}
}
