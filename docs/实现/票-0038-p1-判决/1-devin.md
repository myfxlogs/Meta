# 判决：票 0038-p1（自捕率机械表——判例分类器＋卡行渲染＋坑档标签接线）

- 审计席：devin（run e202ea30e59caac1）·首判（判决史空）
- 交付：支 `t/0038-p1-selfcatch-card` @ `a351f0f9c9f3230ac12130ae1360d5f75e1fb66f`（deliver 事件 3210，devin exit=0）
- 机器记录：无 `/tmp/t0038-p1*.machine-record.txt`；以交付 run 官方实录 `run/artifacts/4c3ffc5efdcdab61/atif.json`（harness 机录退码＋输出原文：step37「149 passed in 3 packages」、step39「234 passed in 16 packages＋SELFCHECK_OK」、step53「234 passed＋ALL_OK」、step55「VET_OK」）为准；**本席照票-0017/0022/0026 先例在交付 worktree @a351f0f 原命令复算**，全量记录 `/tmp/t0038-p1-audit-recompute.txt`；对抗锚突变实证于 scratch `/tmp/t0038-p1-mut`（`/tmp/t0038-p1-mut-a.txt`/`mut-b.txt`，已 `git worktree remove --force`，交付树未动）
- 网关前置闸：`cmd_run` 表对本票零行（机械前置闸未建，同 0019 先例）→ 范围核对由本席自 diff 复核

## 逐锚对账（票 §6 七锚）

| 锚 | 证据 | 判 |
|---|---|---|
| 1 注入拒判→率必变方向正确 | `catchrate_test.go:17-31` fixture 一自捕行（100%）注入 reject→total=2/self=1/「50%（近 2 判）」；复算 PASS＋日志「100%→50%」 | 绿 |
| 2 同事件重放→数值不变 | `catchrate_test.go:34-46` 同 fixture 两调输出恒等（纯函数构造幂等）；复算 PASS | 绿 |
| 3 无判例流→「暂无」非 0% | `catchrate_test.go:50-64` 空集＋audit_pass/无标 progress/非 patrol- created 均不入分母；`card.go:183-186` Catch 空→「暂无」由 `TestRenderCatchLine` 断言；复算 PASS | 绿 |
| 4 坑档＋同票后驳→失格；驳后新径→计 | `catchrate_test.go:68-92` 三子例含两驳夹标；`catchrate.go:60-62,70-72` rejected 保最大 id；复算 PASS | 绿 |
| 5 同(票,径)双报→去重 | `catchrate_test.go:96-114` seenPit 首见＝最新行＋多径逗号列；复算 PASS | 绿 |
| 6 删任一映射行→用例必红 | scratch 三突变实证：删 `deploy_failed`→ExtKinds FAIL（total:3）；删 `reject`→ExtKinds/WindowCap FAIL；删 `created`→ExtKinds FAIL | 绿 |
| 7 删 renderCard 自捕率行→用例必红 | scratch 突变删 `card.go:203` mdLine→TestRenderCatchLine FAIL（「卡面缺自捕率要素」） | 绿 |

## 验证节（票 §7）

`go build`→BUILD_OK、`go vet`→VET_OK、`go test -count=1 ./...`→16 包全 `ok`（含 2 无测试包）、`bash scripts/selfcheck.sh`→RC=0——本席复算与实录一致。演练实证档 `docs/实现/演练-0038-自捕率.md` 在支内，拒判前后卡 JSON 与本席测试日志输出逐字同。gofmt 净（新件零列出）。活库实测：同 WHERE 子句直查 `meta.db` 12 行命中，deliver 事件 3210 note 自带「坑档：」标签入流——读径产线自证。

## 范围核对

diff 10 件全在票 §3 授权面：`internal/db/ticket.go`（CatchRows）、`internal/gateway/catchrate.go`+`_test`、`feishu_card.go`、`internal/feishu/card.go`+`card_test.go`、`roles/施工.md`、`roles/CTO.md`、演练档、经验库坑档（令#E1 同生义务，即本票自报首判例）。零范围蔓延。

## 禁区核对

零新表/零时钟/零快照（selfcheck R2 闸零命中，全 diff grep `time.Sleep|Tick` 零行）；`ticket_event` 零写（CatchRows 纯 SELECT）；相位机/`reject`/`deploy_failed` 语义未触；无真 lark PATCH（fixture+renderCard 径）；卡既有六行未动（diff 纯追加）；无历史回填；无多项目分表。

## 交付形制

支名 `t/0038-p1-selfcatch-card` ✓；commit `t0038-p1:` 写为什么 ✓；已推 origin（ls-remote＝a351f0f）✓；哨兵 `META_DELIVER:` 末行独占＋`sentinel.log` 双发（atif step59＋文件原文）✓；交付摘要五节齐 ✓；不部署 ✓。

## 质量三问

- **最优解**：CatchRows 单趟 DESC 直取 vs EventsAfter 升序游标页两趟——定夺有据（ticket.go:217-221 记因）；分类纯函数 fixture 直喂零 DB 依赖。
- **第一性**：判例流本在 ticket_event，读行→纯函数→预格式化行内文注入既有 View——无新消费者/新状态/间接层；触发零新增（哈希闸挡非判例重算）。
- **洁净**：无重复实现/死码/TODO/调试残渣（diff grep 实证）；`CardView.Catch` 唯一装配点（`cardView` 为唯一构造处）。

## 债项（对账＋观察，均非否决级）

- 摘要「下一步」申报四项（近 M 判文案/1024 扫描上限启发值/坑档标签无机械闸/历史回填另票）——与 diff 对账全符，零瞒报。
- **观察①（口径边界，未申报——记此供 CTO 裁，不算瞒报，设计 §2 未言明）**：一行双籍——kind 命中外捕且 note 携坑档标签（如 CTO 部署失败 Deliver Ref 附坑档，新约定正产此形）→ 计两判（分母+1 分子+1）。实现取「谓词命中即计」，方向轻偏自捕；设计「两类行」未禁双籍。
- **观察②（同窗边界）**：双籍行在 total=19 时可至 21 →「近 21 判」超窗一判，装饰级。
- **观察③**：CatchRows 无 DB 层单测——列序与 scanEvents 契约靠同构＋本席活库实查补证。

## 可沉淀打法意见

「判例分类＝读件＋纯函数＋卡行」三段式——**可沉淀**：DB 单趟读件出候选→映射表/窗口/去重/失格纯函数可 fixture 直喂→预格式化行内文入 View，渲染零判断；凡「事件流统计上卡」径可复用。

## 判决

**通过**——七锚全绿（复算＋三映射行/一渲染行突变删之必红实证），验证四件机器记录一致，范围净、禁区零触、形制齐、暗债零瞒报。
