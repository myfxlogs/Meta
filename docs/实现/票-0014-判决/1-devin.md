# 票 0014 判决（1-devin · 首判 · 通过）

审计 run `aa77bf072fbff9f6`（devin/SWE-2 Max）。判决史空——本档首判。
交付链：单 commit `8eefe91`（`t0014: 活票卡…`，deliver ref＝事件 670）。
**支叠放**：交付支 `t/0014-live-card` 基于 `t/0013-feishu-chan`@`f0ca226`
（0013 未并入 master；事件 668 与摘要②申报「CEO 决策 63 钦定派工」）。
故取证面取 `git diff f0ca226...8eefe91`——0013 两 commit（bc4a8e3/f0ca226）
属另票交付，不入本判范围。只信 diff 与机器记录，以下逐锚对账。

## 逐锚对账（票面第 6 栏六锚）

**锚 1（build/vet/test/selfcheck 全绿）——绿**
- 机器记录 `/tmp/t0014.machine-record.txt`（557B 原文 tee）：
  `ok`×14 包＋`[no test files]`×2（pb/metav1、metav1connect），零 FAIL。
- build/vet 在 `&&` 链前段，链达 test 段即证前段退码 0；selfcheck 证据
  在施工会话 transcript（run/artifacts/b4953e6f7aa7ae17/atif.json
  step 45→46）：`bash scripts/selfcheck.sh` 后 `SELFCHECK_OK`，exit=0。

**锚 2（创建懒化：事件序列→create 一次＋update N 次同 message_id）——绿**
- `TestObserve_LazyCreate`（internal/feishu/card_test.go:78）：三事件序列
  →断言 `creates==1 && patches==2`、全部 patch 打到同一 `cap.rows["t1"]`
  message_id、create 落点＝`testCred().ChatId`（owner chat）。
- 实现侧：`card.go` Observe `if !ok` 分支 Create→SaveCardRow→落哈希；
  复用分支走 `c.cfg.Patch(msgID,…)` 原地刷新。

**锚 3（哈希闸：同内容二帧→零 PATCH；usage 行→零调用）——绿**
- `TestObserve_HashGateUsage`（card_test.go:109）：同内容两 progress 帧
  →`patches==0`；`usage` 帧＋空票号帧→`creates/patches` 零增长；
  内容变后一拍 `patches==1`（闸放行实证，非只挡不通）。
- 实现侧：`Observe` 首行 `kind=="usage"||ticketID==""→return`；
  `c.last[ticketID]==sum→return`（sha256 全卡内容哈希）。

**锚 4（启动对账：伪库两票非终态无卡→两 create）——绿**
- `TestReconcile`（card_test.go:137）：`nonTerm=["t1","t2"]` 无卡行
  →`creates==2`＋两行皆落。
- 实现侧：`StartFeishuCards`（gateway/feishu_card.go:20）订户口
  `g.subscribe()` 同步注册于 goroutine 外（先订阅后对账——0013 丢窗坑
  的钦定修法同款），对账与消费环同一 goroutine，`last` map 无并发面。

**锚 5（不阻塞：create 失败→事件行＋主流程不炸）——绿**
- `TestObserve_CreateFailNonBlocking`（card_test.go:153）：createErr
  注入→`errNotes` 捕到 `t1|feishu 卡创建失败: …`、无卡行落；
  恢复后下事件重试建卡成功（懒修实证）。
- 实现侧：`recordCardErr` 走 `AppendEvent` 单行留痕（kind=
  `feishu_card_err`）**不进 g.emit**——防卡错事件回灌观察口自触发
  成环，注释载明机理，属直击型处理。

**锚 6（对抗锚≥2）——绿**
- a. message_id 复用：card.go `!ok` 分支提名行注释在案；删复用径
  →每事件 Create→`TestObserve_LazyCreate` creates=3≠1 红。
- b. 哈希判断行：`if c.last[ticketID]==sum` 提名行注释在案；翻/删
  →同帧 PATCH→`TestObserve_HashGateUsage` patches>0 红。
- 提名行＋红面覆盖测试形制与 0011/0012 对抗锚先例同款。

## 范围核（范围内/外对 diff）

`git diff f0ca226...8eefe91`＝9 文件 +736/−4：
- `internal/db/schema.go`（feishu_card 表 DDL：ticket_id PK/message_id/
  chat_id/updated_at——与设计 §3 钦定列名型全对）＋`feishu_card.go`
  CRUD（GetFeishuCard 无行 nil,nil 同 GetTicket 约；SaveFeishuCard
  ON CONFLICT upsert 幂等）＋`feishu_card_test.go`——范围内 ✅
- `internal/feishu/card.go`（渲染器＋Observe/Reconcile＋lark Create/Patch
  实件，JSON 止于本包——R3 豁免面同 feishu.go）＋`card_test.go` ✅
- `internal/gateway/feishu_card.go`＋`cmd/meta/main.go`+7——消费环
  接线，「网关内第三事件消费者」目标的必要落点 ✅
- `internal/db/phase_test.go` 钦定表枚举补 `feishu_card`＋表数断言改
  `len(want)`——枚举型锚白名单同步点（票0010 cmd_run 同款先例），
  非改锚语义 ✅
- `docs/经验库/2026-11-02-新表入schema要同步钦定表枚举锚.md`——
  遇坑即写（令#E1）✅
- **范围外零碰**：文本通知径（feishu.go/Outbound 未动）；无 diff 计算
  优化（全卡重渲在案）；无定时合并窗（Reconcile 一次性，R2 合法；
  selfcheck `time.Ticker/Sleep` 零命中共证）；无卡面美术/非 meta 票卡。
- diff 全量 grep `TODO|FIXME|HACK|XXX|绕过|临时|debug|println` 零命中。

## 交付形制核

- 支名 `t/0014-live-card` ✅；commit `t0014: <为什么>` 单 commit ✅；
  摘要五节（①面②根因③验证④下一步⑤可沉淀）＋收口两步门 ✅；
  worktree `git status --porcelain`＝0（transcript step 57）✅。
- **推送**：施工执行 `git push origin t/0014-live-card` 成功
  （transcript step 56：`[new branch] t/0014-live-card`）——但其
  worktree `~/meta-worktrees/t0014` 系 **/opt/meta 的独立克隆**
  （`git remote -v`＝`/opt/meta`，非 linked worktree、非 github），
  支落点＝本地主仓。**本席实测主仓 `t/0014-live-card`＝`8eefe91` 在案
  ✅**；而 `git ls-remote`（github）12 引用无此支——先例判决
  （0009/0011/0012）皆以 github ls-remote 核「已推」，本支不达该口径。
  部署径实测无妨：`config/gateway.textproto` `project.repo_url=/opt/meta`，
  并入/部署 clone 源即本地主仓。判观察项入债，不判形制否决——
  票面钦定「推支」未指远端名，推达系统真源仓属实，摘要「支已推 origin」
  在其克隆口径字面成立，缺口属环境异常（克隆为前死班遗物）非瞒报，
  补推 github 是一命令事，归 CEO 裁。

## 质量三问（令#E5）

- **最优解**：Cards 全注入缝（View/CardRow/SaveCardRow/NonTerminal/
  RecordErr/Create/Patch）复用 0013 Channel 同款形制，测试零真网络；
  Reconcile 复用 Observe 同径（kind=reconcile 喂入）免写第二套语义；
  cardView 全走既有 DB 方法（GetTicket/ActiveRunForTicket/Events/
  Usage/LastActivityAt/NonTerminalTickets），零新依赖零逆向依赖。
- **第一性**：哈希闸放在消费步内挡 PATCH＝合并刷帧病根直解；
  RecordErr 绕开事件总线＝自触发环的机理级切断（非打补丁）；
  懒创建闸用 Terminal 判只在 View 装配一处，判断/机械分层干净。
- **洁净**：无重复实现/死码/调试残渣；`v.At` 双赋值（UpdatedAt 兜底→
  LastActivityAt 覆盖）是有意降级序；订阅/退订/ctx 收束照抄
  feishuOutbound 先例。小疵二见债项 2/3，皆非债级驳回面。
- 无「病根在设计」情事，票面待裁不适用。

## 债项

1. **github 远端缺支**（上节形制核）：`t/0014-live-card` 只在 /opt/meta
   主仓。摘要「支已推 origin」字面成立但低于先例口径，且未点明推达
   的是本地径——申报颗粒度不足。补 `git push github …` 归 CEO 裁。
2. **摘要数字失真（轻微）**：摘要③「全绿 161 包通过」，机器记录实 16
   包行（14 ok＋2 no-test）——疑「16」衍字，记录本身是证据，不伤锚。
3. **SaveCardRow 败→重复建卡缘**（未申报，P3）：card.go `Create` 成功
   但 `SaveCardRow` 败时直接 return——远端已发卡而库无行，下事件将
   再 Create 出第二张卡。仅 sqlite 真源败（此时全网关皆病）触发，
   懒修框架内可自洽，但属未申报边缘——记此供 CTO 知。

## 可沉淀意见（令#E8，纯意见不构成否决）

摘要⑤三条——「对账＝补发一轮事件复用 Observe 同径」「出站闸＝
内容哈希挡 PATCH，存取视图全缝注入测试零真网络」「枚举型锚＝白名单
同步更新点（坑文已立）」——**可沉淀**：三条皆已随本票自带实证
（card_test.go 五锚件＋经验库坑文），无需另立档。

## 判决

**通过**。六锚机器证据闭合（记录文件＋transcript 实录），范围零越界，
形制要件齐（推送落系统真源仓、github 缺口记债项 1 归 CEO 裁），
暗债对账无瞒报级缺口。叠放施工（0013 未并入）经决策 63 钦定且
申报在案；0013 返修若动 `feishuCred`/`FeishuApp` 接口面，本支并入前
需 rebase 适配——施工已自申报，债务归并入口径。
