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

func (c *cardCap) patch(_ context.Context, messageID, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.patches = append(c.patches, messageID)
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
func testView(tid, phase, stage, note string) *CardView {
	return &CardView{
		TicketID: tid, Title: "标题甲", Phase: phase,
		Role: "施工", Shell: "devin", Stage: stage, Note: note,
		Used: 100, UsedTotal: 250, Rejects: 1,
		Terminal: phase == "完成" || phase == "作废",
		At:       time.Unix(1762000000, 0),
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
