# 票 0011 判决：通过（审计席·devin 独立实例，2026-11-02 二判）

- 审计 run：`d1994f6f45943fa3`（shell=devin，独立实例）。触发＝前任审计席
  `10d1d68228645450` 落判决档 `1-claude.md`（commit `9e0dee6`）后死于哨兵回流前
  （事件 318 `audit_dead`：SIGCHLD+EOF 双源互证）→ 待审重派（事件 319）。
- 背景：决策62 已对该判「捞判转正」（判决实录在席位日志、D-42 接收器聋）。
  本档**不抄前任结论**——全部锚经本席独立亲证，另补前任未变异真跑的锚③。
- 交付链：支 `t/0011-ceo-face`＝`4a1e5e4`（前任 devin 全量）＋`6ea7bc9`（接替
  claude 三修，deliver 即此，已推 origin）＋`9e0dee6`（前任判决档）。未并 master。
- 机器记录：`/tmp/t0011.machine-record.txt`——12 包 `ok` 原文 tee；mtime
  00:18:59 与交付 commit 00:19:07 同轮咬合（build/vet 经 `&&` 前置，链语义＝绿）。

## 逐锚对账（每条指向 diff 行或机器输出/本席复跑）

| 锚 | 证据 | 判 |
|---|---|---|
| 1 全量绿 | 机器记录 12 包 ok（含 `internal/gateway 13.874s`）；pb 落点核：`meta.pb.go`/`meta_grpc.pb.go` 含 CeoGateway/MemoryOp/SeatAdmin 符号（grep 91/66 命中）＝proto 再生入档；`scripts/selfcheck.sh` 为确定性 grep 禁式闸（Ticker/Sleep/json 三式）；**本席复跑**：17 个锚点测试全绿（`go test ./internal/gateway -run 'TestCeo*|TestReqPending|TestAdminGate|TestD*|TestImportLegacy|TestAudit_Redispatch'`） | **绿** |
| 2 锁 | `ceo.go:47-57` registerCEO 拒分支（`ceoSub!=nil→false`）＋`ceo.go:61-73` deregisterCEO EOF 除名（`ceoSubRunID` 核对防误删）；`CeoEvents` `ceo.go:309-313` 拒径 `FailedPrecondition`；`TestCeoLock`/`TestCeoSpawn`/`TestCeoSpawn_LastResort` 本席复跑绿；前任对抗①变异真跑 600s 挂起红（档载） | **绿** |
| 3 provenance | `ceo.go:360` `prov := run.Provenance`（对抗锚②提名行——请求 `Requirement.provenance` 字段在读后不再引用，自报被盖）；链全戳：`admin.go:163-171` IssueCeoToken{terminal}、`caster.go:244` 触发事件行透传、`dispatch.go:303` spec→run 落列、`ceo.go:647` admin 子票 `st.run.Provenance`、`spawnCEO` `ceo.go:172` system_event；`TestCeoProvenance` 复跑绿 | **绿** |
| 4 Rule | `ceo.go:417-468`：void 无 confirm→`NeedConfirm=true`（`419-421`）；budget_raise `GetBudget`→`SetBudgetCaps`→`UsedTotal<HardCap` 解冻（`432-444`）；redirect 闸 `448-451`（相位已立＋最新事件 req_pending 缺一拒）→ `DispatchSeat(role="CTO")`——`455 spec.Provenance` 承戳，`456-462` 缺栏票落 CTO-A 内联提示（接替三修之根因：角色指反已正）；`TestCeoRule`/`TestCeoRule_RedirectPrompt` 复跑绿 | **绿** |
| 5 Inquire | `ceo.go:473-493`：`DispatchSeat{Adjunct:true}`＋`inquire` 行落票；`TestCeoInquire` 复跑绿 | **绿** |
| 6 Memory | `ceo.go:552-601` 流内每帧 `ceoByToken` 复验；put `568-581` 落 `event_ref=run_id`＋run 戳；`kind=="say"`→`ceo_say` AppendEvent+emit（`573-578`）；`memory.go` 表 `PutMemory/ListMemory/CountMemoryByEventRef/MemoryHas`；`TestCeoMemory` 复跑绿 | **绿** |
| 7 req_pending | `attach.go:339-344` CTO Yield→`PhaseCreated`+`EdgeReqPending`（补边入 `phase.go:48,75` 边表）；`reconcile.go:42-49` 对账抑制（`LatestEventKind==req_pending→continue`）；`TestReqPending` 断言「泊车票无 spawn_needed＋对照票 tp2 照补」（`ceo_test.go:498-508`）。**对抗③本席变异真跑**：`if false{}` 罩死抑制分支→`TestReqPending` 红「泊车票被对账重派」`ceo_test.go:502`→复原绿 | **绿** |
| 8 import-legacy＋D-22 | `TestImportLegacy_Parser`（cmd/meta：fixture→2 行、provenance 抽取、event_ref=legacy-import、二跑零增）＋`TestImportLegacy`（库层 MemoryHas 幂等）；`ceoGapCheck` `ceo.go:260-276`（dispatched>0 且 CountMemoryByEventRef=0→`ceo_gap` 行）；`deregisterCEO`/`onRunDead`/`handleAdmin` 三退场点均挂（`ceo.go:71`、`attach.go:433-436`、`ceo.go:723`）；`TestCeoGap` 复跑绿 | **绿** |
| 9 admin 帧闸 | `ceo.go:610-613` `role!=CTO||Adjunct→PermissionDenied`（对抗锚④提名行）；fill_spec `620-626` 越票拒＋限施工中相位；create_ticket `647` 子票同戳；dispatch `653-656`→`DispatchSeat`（role=CEO 永拒 `dispatch.go:51-54` 三路共用闸）；`TestAdminGate`/`TestAdmin_DispatchCEO硬拒` 复跑绿；前任对抗④变异真跑 0.06s 即红（档载） | **绿** |
| 10 债包 | D-36 `dispatch.go:262-267` `budgetOf` 注入缝 err→return（`TestD36_BudgetErrReturn`）；D-34 `attach.go:228-232`/`256-260` 两处先 AppendEvent 后 emit（`TestD34_Order` 源序断言）；D-15 `gateway.go:365-366` 终态→`onTerminal` 删 specs/failSet/warned/frozen＋闹钟（`ceo.go:671-683`，`TestD15_TerminalCleanup`）；D-14 `TestSpecFromEnv{,_Missing}`/`TestScanSentinel{,_Admin}`；D-38 `TestAudit_RedispatchReject`＋`TestCLI_GlueE2E`＋变异真跑（①④前任、③本席、②施工申报＋结构核）；D-37 `t0004_test.go` 大括号归位；D-32 `inject.go:132-138` 模板硬行 `META_DELIVER:<ref>`＋`seat.go:455-460` 哨兵扫描＋`seat.go:163-176` 净退无哨兵→ExitSeatErr 不 Deliver（`TestRun_NoSentinelNoDeliver`/`TestRun_DeliverSentinel`/`TestRun_AuditVerdictSentinel`——审计席 META_VERDICT 同径） | **绿** |
| 11 对抗锚≥4 | ①锁拒分支 ②provenance 取行 ③抑制闸 ④admin 闸——四守卫行俱在且各行有对抗注释提名；变异红证据：①④前任独立跑、③本席独立跑、②施工申报（摘要三节原文在日志）＋断言结构核（`TestCeoProvenance` 断言 created/requirement 行=feishu 戳非自报 terminal） | **绿** |

## 交付形制与范围

- 支名 `t/0011-ceo-face`、commit `t0011: <为什么>`×2、`origin`=6ea7bc9 已推、
  未并 master、工作区净；前任判决档 `9e0dee6` 同支在后。
- 交付摘要五节形在（席位日志 `run/logs/835973da60bafd26.log` 尾：新增修改面/
  根因/验证引/下一步/可沉淀打法候选），尾帧 `META_DELIVER:6ea7bc9`。
- diff 36 文件全落 §3 范围内：`internal/caster` 1 行＝票面钦定「路由派发透传
  触发事件行 provenance」；`docs/经验库/`5 篇＝令#E1 坑记流程件（订阅goroutine
  丢首事件/双claim竞速/重派提示核角色/锁挂瞬态相位＋前任1篇）；路由表未触
  （票面附注「requirement→dispatch CTO 不落表」遵行）；无票外顺手改码。

## 债项节（暗债对账·令#E6）

diff 内 TODO/FIXME/HACK 标记＝零（本席 grep 亲证）。摘要 §4 申报六项逐一核码：

| 申报 | 复核 | 判 |
|---|---|---|
| 设计锁 requeue 径不重取（窄窗） | 真：锁仅 `dispatch.go:88-101` `Phase==PhaseCreated` 分支取，回队票相位已非已立不经此 | 对上 |
| project 标签无生产写径 | 真：`SetProject` 仅测试调，`NewTicket.project` 归 0012 | 对上 |
| `meta ceo` REPL 先求通 | 票面范围外已预声明 | 对上 |
| `ceoDispatched` 进程内存 | 真：`gateway.go:148` map，重启丢计数（摘要自报窗有限） | 对上 |
| master 7 文件 gofmt 不齐票外 | 前任复跑 `git show master:` 证实基线已脏；本批所改文件零不齐 | 对上 |
| 0013 两项（连败直推飞书；requirement 路由行不落表） | 与票面附注逐字一致 | 对上 |

瞒报零。台账 D-14/15/22/34/36/37/38 行仍 open——销号随判决并流（0008 先例）；
D-32 源在 ceo_memory 决策账行 330（令#E15 首个适用），非台账表条目，引用合法。

## 质量三问（令#E5）

- **最优解**：锁＝连接登记单 map 互斥（非 DB 旗非阻塞，禁区#10/B8 钦定形）；
  `specComplete` 复用 `inject.CheckSpec` 同一把缺一不派尺；`ceoAPI` 薄包装解
  Events 名撞车——无更简等价形可见。
- **第一性**：provenance 一律盖 run 行（自报字段读后即弃）直击 D9 本质；
  抑制＝读最新事件 kind 无新状态机；D-22＝两计数纯机械比（禁区#7 界内）。
- **洁净**：`ceoPending/ceoFails/ceoDispatched` 注释即申报内存态；无同逻辑
  双写/死码/调试残渣。
无质量驳回举证（说不出具体更优替代形制）。

## 沉淀意见（令#E8，纯意见）

「接替席逐锚变异真跑」**可沉淀**——本席以一条命令复证锚③（`if false` 罩闸→
精确红→复原绿，约一分钟），佐证施工与前任各自变异结论互不依赖仍一致，是
绿测试≠活锚的最便宜可信化手段；「锁类测试须断言持锁期第二操作拒」同理可沉淀。

## 判决

**通过**。锚 1–11 全绿（机器记录＋本席独立复跑 17 测试＋锚③变异真跑红→复原）。
范围净、形制齐、暗债申报与码全对上。前任 `1-claude.md` 逐锚结论经独立复核成立，
决策62 捞判转正的机器基据本档坐实。票 0013 硬依赖（proto/锁/泵）地基成立。

META_VERDICT:pass:docs/实现/票-0011-判决/2-devin.md
