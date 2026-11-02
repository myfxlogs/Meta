// 活票卡（票0014/D-31/令#E14/§1.7）——网关内第三事件消费者：
// 一切非 usage 票事件→按票重渲全卡（无 diff 计算，全卡钦定）→
// 内容哈希变才 PATCH（天然合并刷帧——进度连发不连发 PATCH）；
// 创建懒化（首见非终态票事件→create 发卡→message_id 落 feishu_card 行）；
// 启动对账（扫非终态票无卡→补建，一次性 R2 合法）；创建失败→记事件
// 不阻塞（文本兜底在，卡面晚一拍不拖死事件环）。
// 卡片 JSON 止于本包（lark API 边界；R3 豁免面同 feishu.go）。
package feishu

import (
	"context"
	"sync"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	pbv1 "meta/internal/pb/metav1"
)

// CardView 渲染输入——票+相位+席+stage+用量+驳回数+时间（网关注入装配，
// 终判由 View 给出 Terminal；本包只做机械渲染与哈希比对）。
type CardView struct {
	TicketID  string
	Title     string
	Phase     string
	Role      string // 当前席角色（无活席空串）
	Shell     string // 当前席壳
	Stage     string // 最近进度首词（stage 列）
	Note      string // 最近进度 note 全文
	Used      int64
	UsedTotal int64
	Rejects   int
	Terminal  bool      // 终态票（完成/作废）——懒创建闸：终态无卡不补
	At        time.Time // 卡面「时间」＝票上最近活性时刻（状态派生——
	// 渲染墙钟会击穿哈希闸：内容永变则每事件 PATCH，合并刷帧失效）
	// 票0036 相位耗时三件（耗时唯一时间源＝RenderAt，装配收口——renderCard
	// 保持纯函数；显示粒度截断后同粒度窗内容逐字节同，哈希闸不失效）：
	PhaseSince time.Time // 当前相位进入时刻（零值→裸相位行降级）
	RenderAt   time.Time // 本次渲染时刻（非终态耗时＝RenderAt−PhaseSince）
	CreatedAt  time.Time // 票创建时刻（终态总时长＝PhaseSince−CreatedAt）
}

// CardConfig 卡消费者装配参数；Create/Patch/View 等为注入缝
// （缺省出站＝lark REST 实件；存取/视图由网关注入 db 径）。
type CardConfig struct {
	Cred *pbv1.FeishuApp // 卡落点＝Cred.ChatId（owner chat 同文本兜底）

	// View 按票装配渲染输入；票不存在返回 nil,nil。
	View func(ctx context.Context, ticketID string) (*CardView, error)
	// CardRow 取已落 message_id（对抗锚 a 复用源）；无行 ok=false。
	CardRow func(ctx context.Context, ticketID string) (messageID string, ok bool, err error)
	// SaveCardRow 懒创建成功后落 message_id 行。
	SaveCardRow func(ctx context.Context, ticketID, messageID, chatID string) error
	// TouchCard PATCH 成功后刷新 updated_at（T9）；nil 不记。
	TouchCard func(ctx context.Context, ticketID string)
	// NonTerminal 启动对账扫描面（非终态票号全列）。
	NonTerminal func(ctx context.Context) ([]string, error)
	// RecordErr create 败记事件行（DB 真源；不进事件总线——
	// 卡错事件回灌观察口会自触发重渲成环）。
	RecordErr func(ctx context.Context, ticketID, note string)

	// Create 发卡→message_id；Patch 原地刷新卡。nil＝lark 实件。
	Create func(ctx context.Context, chatID, contentJSON string) (messageID string, err error)
	Patch  func(ctx context.Context, messageID, contentJSON string) error
	Logf   func(format string, args ...any)
}

// Cards 卡渲染消费者：持最近成功落卡内容哈希（哈希闸）＋lark client
// （与 Channel 同源 REST 面，各持实例无碍——REST 无连接态）。
type Cards struct {
	cfg  CardConfig
	api  *lark.Client
	mu   sync.Mutex        // last 并发闸（T9 补建批并发）
	last map[string][32]byte // 票→上次成功落卡的内容哈希
}

// NewCards 装配卡消费者（不起环——Observe/Reconcile 由调用方驱动）。
func NewCards(cfg CardConfig) *Cards {
	c := &Cards{cfg: cfg, last: map[string][32]byte{}}
	if c.cfg.Logf == nil {
		c.cfg.Logf = func(f string, a ...any) { log.Printf("feishu-card: "+f, a...) }
	}
	if cfg.Cred != nil {
		c.api = lark.NewClient(cfg.Cred.AppId, cfg.Cred.AppSecret)
	}
	if c.cfg.Create == nil {
		c.cfg.Create = c.larkCreateCard
	}
	if c.cfg.Patch == nil {
		c.cfg.Patch = c.larkPatchCard
	}
	return c
}

// Observe 消费步：网关事件观察口逐事件喂入——
// 一切非 usage 票事件→重渲全卡→哈希变才 PATCH。
// usage 帧与无票事件（feishu_inbound/ceo_* 等空票号帧）过手不动作
// （锚：usage 行→零调用）。
func (c *Cards) Observe(ctx context.Context, kind, ticketID string) {
	if kind == "usage" || ticketID == "" {
		return
	}
	v, err := c.cfg.View(ctx, ticketID)
	if err != nil || v == nil {
		c.cfg.Logf("卡视图装配败 tid=%s: %v", ticketID, err)
		return
	}
	content := renderCard(v)
	sum := sha256.Sum256([]byte(content))
	msgID, ok, err := c.cfg.CardRow(ctx, ticketID)
	if err != nil {
		c.cfg.Logf("卡行读取败 tid=%s: %v", ticketID, err)
		return
	}
	if !ok {
		if v.Terminal {
			return // 懒创建闸——只对在途票补卡，终态票不追建
		}
		newID, err := c.cfg.Create(ctx, c.cfg.Cred.ChatId, content)
		if err != nil {
			// 创建失败→记事件不阻塞（锚：事件行＋主流程不炸；
			// 不落行不落哈希——下个非终态事件重试）。
			c.cfg.Logf("卡创建败 tid=%s（记事件，下事件再试）: %v", ticketID, err)
			if c.cfg.RecordErr != nil {
				c.cfg.RecordErr(ctx, ticketID,
					fmt.Sprintf("feishu 卡创建失败: %v", err))
			}
			return
		}
		if err := c.cfg.SaveCardRow(ctx, ticketID, newID, c.cfg.Cred.ChatId); err != nil {
			c.cfg.Logf("卡行落库败 tid=%s mid=%s: %v", ticketID, newID, err)
			return
		}
		c.last[ticketID] = sum
		return
	}
	// 对抗锚 a 提名行＝下行复用判断（删之＝每事件新卡）；复用已落
	// message_id 原地刷新——一卡到底（E14 钦定）。
	c.mu.Lock()
	same := c.last[ticketID] == sum
	c.mu.Unlock()
	if same {
		return // 内容未变——哈希闸挡 PATCH（对抗锚 b 提名行：翻之＝同帧也 PATCH）
	}
	if err := c.cfg.Patch(ctx, msgID, content); err != nil {
		// PATCH 败不落哈希——下个票事件重试（超频懒修 R2 条款）。
		c.cfg.Logf("卡 PATCH 败 tid=%s mid=%s（下事件再试）: %v", ticketID, msgID, err)
		return
	}
	c.mu.Lock()
	c.last[ticketID] = sum
	c.mu.Unlock()
	// T9/D-63 后半：PATCH 成功写 updated_at（DB 留痕——有凭据才可追卡龄）。
	if c.cfg.TouchCard != nil {
		c.cfg.TouchCard(ctx, ticketID)
	}
}

// Reconcile 启动对账（一次性 R2 合法）：扫非终态票→各票走 Observe 同径——
// 无卡补建（懒创建）、有卡按哈希闸刷新（重启后 last 空＝每卡至多补一拍）。
// T9/D-67：补建批并发（8 工位扇出）——首航逐张缓建滞后数十秒的实证，
// 同步于对账不再串排；last 写锁保护并发安全。
func (c *Cards) Reconcile(ctx context.Context) {
	ids, err := c.cfg.NonTerminal(ctx)
	if err != nil {
		c.cfg.Logf("启动对账扫描败: %v", err)
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, tid := range ids {
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			c.Observe(ctx, "reconcile", tid)
		}(tid)
	}
	wg.Wait()
}

// ---------- 渲染（票+相位+席+stage+用量+驳回数+时间→卡片 JSON） ----------

// renderCard 全卡重渲（无 diff——哈希闸在外层挡 PATCH）。
// 经典卡片结构：header 票号·标题＋六行要素 div。
func renderCard(v *CardView) string {
	seat := "—"
	if v.Role != "" {
		seat = v.Role
		if v.Shell != "" {
			seat += "·" + v.Shell
		}
		if v.Stage != "" {
			seat += "（" + v.Stage + "）"
		}
	}
	note := v.Note
	if note == "" {
		note = "—"
	}
	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"template": "blue",
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "票 " + v.TicketID + " · " + v.Title,
			},
		},
		"elements": []any{
			mdLine("**相位**：" + v.Phase + phaseSuffix(v)), // 对抗锚 a 提名行（票0036）：删后缀→锚2/3 红
			mdLine("**当前席**：" + seat),
			mdLine("**进度**：" + note),
			mdLine(fmt.Sprintf("**用量**：%d（累计 %d）", v.Used, v.UsedTotal)),
			mdLine(fmt.Sprintf("**驳回**：%d", v.Rejects)),
			mdLine("**时间**：" + v.At.Format("2006-01-02 15:04:05")),
		},
	}
	b, err := json.Marshal(card)
	if err != nil {
		return `{"elements":[{"tag":"div","text":{"tag":"plain_text","content":"卡渲染失败"}}]}`
	}
	return string(b)
}

// phaseSuffix 相位行耗时后缀（票0036）：非终态 d=RenderAt−PhaseSince→
// 「已 N 分/X.Y 时」；终态 d=PhaseSince−CreatedAt→「总 …」（固定快照——
// 终态后重渲逐字节同，不再随渲染时刻漂移）。d<0 钳 0；PhaseSince 零值
// →裸相位行（畸形视图安全降级）。<60min 整分 floor；≥60min %.1f 时。
func phaseSuffix(v *CardView) string {
	if v.PhaseSince.IsZero() {
		return ""
	}
	d := v.RenderAt.Sub(v.PhaseSince)
	prefix := "已"
	if v.Terminal {
		d = v.PhaseSince.Sub(v.CreatedAt)
		prefix = "总"
	}
	if d < 0 {
		d = 0
	}
	if d < time.Hour {
		return fmt.Sprintf("（%s %d 分）", prefix, int64(d.Minutes()))
	}
	return fmt.Sprintf("（%s %.1f 时）", prefix, d.Hours())
}

// mdLine 一行 lark_md div（卡面文案全经此行——note/stage 用户源文本
// 由 lark_md 渲染，JSON 层已转义）。
func mdLine(text string) map[string]any {
	return map[string]any{
		"tag":  "div",
		"text": map[string]any{"tag": "lark_md", "content": text},
	}
}

// ---------- lark 实件（msg_type=interactive 发卡；PATCH 原地更新） ----------

// larkCreateCard 实发：im.v1 Message.Create（msg_type=interactive，
// 卡 JSON 为 content）→message_id。
func (c *Cards) larkCreateCard(ctx context.Context, chatID, content string) (string, error) {
	resp, err := c.api.Im.V1.Message.Create(ctx, larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.CreateMessageV1ReceiveIDTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).MsgType("interactive").Content(content).Build()).
		Build())
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("lark 发卡被拒: code=%d %s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.MessageId == nil || *resp.Data.MessageId == "" {
		return "", fmt.Errorf("lark 发卡响应缺 message_id")
	}
	return *resp.Data.MessageId, nil
}

// larkPatchCard 实补：im.v1 Message.Patch（message_id 定点＋新卡 JSON——
// 仅 interactive 型可 PATCH，SDK 语义）。
func (c *Cards) larkPatchCard(ctx context.Context, messageID, content string) error {
	resp, err := c.api.Im.V1.Message.Patch(ctx, larkim.NewPatchMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewPatchMessageReqBodyBuilder().Content(content).Build()).
		Build())
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("lark 补卡被拒: code=%d %s", resp.Code, resp.Msg)
	}
	return nil
}
