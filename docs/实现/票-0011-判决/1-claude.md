# 票 0011 判决：通过（审计席·claude 独立实例，2026-11-02）

- 审计 run：`10d1d68228645450`（shell=claude，独立实例），触发＝deliver（事件 291，
  `claude exit=0（有 result 帧）`，施工 run `835973da60bafd26`）→ 待审→审计中（事件 292）。
- 交付链：支 `t/0011-ceo-face` 两 commit——`4a1e5e4`（前任 devin 全量实现，run
  `d31d5bab` 中途死未交付）＋`6ea7bc9`（接替 claude 逐锚复核＋三修，deliver 即此）。
  **未并 master**；`origin/t/0011-ceo-face`＝`6ea7bc9`（已推）；工作区净。
- 机器记录：`/tmp/t0011.machine-record.txt`（12 包 `ok` 原文 tee，mtime 00:18:59，
  尾 commit 00:19:07——同轮再生后即提交，时序咬合）。

## 逐锚对账（全部指向 diff 行或机器输出）

| 锚 | 证据 | 判 |
|---|---|---|
| 1 全量绿 | 机器记录 12 包 ok（build/vet 经 `&&` 前置于 test，链语义＝绿）；`bash scripts/selfcheck.sh` 审计席复跑 exit=0 零禁式命中；pb 再生＝审计席以票载同款命令（`--go_opt=module=meta`）再生成于临时目录，`meta.pb.go`/`meta_grpc.pb.go` 逐字节 `cmp` **全同**；`gofmt -l` 全仓仅 7 文件＝摘要申报的 master 基线债（经 `git show master:` 逐一复跑 gofmt 证实**基线已脏非本批带入**），本批新增/所改文件零不齐 | **绿** |
| 2 锁 | `ceo.go:47-57` registerCEO 拒分支＋`ceo.go:61-73` EOF 除名；`TestCeoLock`（B 订阅收 `FailedPrecondition`、A ctx 断→B 得）；`TestCeoSpawn`（spawn_kind 无订阅→`ActiveRunForRole(CEO)` 出 run 且 `provenance=system_event`、二事件 CEO runs=1 不重复 spawn）；`TestCeoSpawn_LastResort`（连败 3→`ceo_last_resort` 行落 `t-y`）。**对抗①变异真跑（审计席独立）**：禁用拒分支→`TestCeoLock` 挂起至 600s 超时 `FAIL meta/internal/gateway 600.084s` | **绿** |
| 3 provenance | `ceo.go:360` `prov := run.Provenance`（只取 run 行）；`TestCeoProvenance`（prov=`feishu:owner-1` 的 run→SubmitRequirement→`created`/`requirement` 行均 `feishu:owner-1`，**席自报 `terminal` 被盖**）；`dispatch.go:49-52` `role==CEO` 永拒＋测试双径（RPC `Dispatch` 与 `DispatchSeat`）皆拒；admin 子票同戳＝`TestAdminGate`（`child-1` created 行 `feishu:o` 承父 run 戳，`ceo.go:647`） | **绿** |
| 4 Rule | `TestCeoRule`：void 无 confirm→`NeedConfirm=true` 拒、confirm→相位 作废＋`ruling` 行；budget_raise→`frozen` 内存键删＋`SetBudgetFrozen(false)`（`ceo.go:439-444`）；redirect 泊车票（`req_pending` 最新事件闸 `ceo.go:448-451`）→`ActiveRunForTicketRole(trp,CTO)` run 起；`TestCeoRule_RedirectPrompt`：缺栏票落 CTO-A 内联提示（断言含 `CTO-A` 且不含「无头化身」）、全栏票 `PromptFile` 空走注入机径 | **绿** |
| 5 Inquire | `TestCeoInquire`：adjunct CTO run 起（prov=terminal 承戳）＋`adjunct_deliver` 行落票 `ti` | **绿** |
| 6 Memory | `TestCeoMemory`：put 回 row `EventRef=run_id`＋`Provenance=terminal`；`kind=say`→`ceo_say` 事件＋DB 行；get 往返 title 一致 | **绿** |
| 7 req_pending | `attach.go:339-343` handleYield CTO→`已立`+`EdgeReqPending`；`TestReqPending`：Yield→相位 已立＋`req_pending` 行、`Reconcile` 后无 `spawn_needed`（对抗③抑制闸 `reconcile.go:45-49`）、对照票 `tp2` 照补；redirect 重派见锚4 | **绿** |
| 8 import-legacy＋D-22 | `TestImportLegacy_Parser`（cmd/meta，**解析器本体直测**：fixture md→2 行、provenance 抽取 `terminal`/`feishu:o1`、`EventRef=legacy-import`、二跑零增）＋`TestImportLegacy`（库层幂等）；`TestCeoGap`：订阅收 1 条事件→`ceoGapCheck`→`ceo_gap` 行（`ceo.go:260-276` 零写且派发过才落） | **绿** |
| 9 admin 帧闸 | `ceo.go:610-613` role 闸；`TestAdminGate`：施工席发 admin→`PermissionDenied` 断流、adjunct CTO→拒、`fill_spec` 越票→`ok=false`、本票施工中→spec 落库；`TestAdmin_DispatchCEO硬拒`。**对抗④变异真跑（审计席独立）**：禁用 role 闸→`TestAdminGate` 0.06s 即红「施工席 admin 未断流」 | **绿** |
| 10 债包 | D-36 `dispatch.go:262-267` err→return＋`TestD36_BudgetErrReturn`（budgetOf 注错→hard_cap 999 不被零值盖）；D-32 `inject.go:135-141` 模板末行＋`TestAssemble_DeliverSentinelLastLine`＋`TestRun_NoSentinelNoDeliver`（净退→`ExitSeatErr` 且 Deliver 不出）/`TestRun_DeliverSentinel`/`TestRun_AuditVerdictSentinel`（审计席无 verdict 同死席径）；D-34 `attach.go` 两处先 AppendEvent 后 emit＋`TestD34_Order`（源序断言）；D-15 `gateway.go:365-366`→`onTerminal` 四表＋闹钟，`TestD15_TerminalCleanup`；D-14 `TestSpecFromEnv`/`TestSpecFromEnv_Missing`（CEO 空 ticket 放行）/`TestScanSentinel`（含 META_ADMIN 三帧）；D-38 三缺口：`TestAudit_RedispatchReject`＋`TestCLI_GlueE2E`＋变异真跑（①④ 审计席自跑红；②③ 施工申报＋守卫行/断言结构核）；D-37 `t0004_test.go` 大括号归位 | **绿** |
| 11 对抗锚≥4 | ①`TestCeoLock` ②`TestCeoProvenance` ③`TestReqPending` ④`TestAdminGate` 四锚俱在；①④ 经审计席独立变异真跑证红，②③ 守卫行（`ceo.go:360`/`reconcile.go:45-49`）与断言面核立 | **绿** |

## 交付形制与范围

- 支名 `t/0011-ceo-face`、commit `t0011: <为什么>`×2、推 origin、未并 master、工作区净。
- 交付摘要五节形在（run 日志尾部：新增修改面／根因／验证引／下一步＝全量遗留申报／可沉淀
  打法候选），尾带 `META_DELIVER:6ea7bc9`。
- diff 27 文件全落 §3 范围内；`docs/经验库/` 5 篇＝令#E1 流程件（3 篇接替席申报＋2 篇前任
  11-01 坑记，内容与 ceo.go 两处实现决策互证）；票面附注「路由行不写入表」遵行（diff 未触
  路由表）；无票外顺手改码。

## 债项节（暗债对账·令#E6）

diff 内 TODO/FIXME/绕过标记＝零（仅测试 fixture 票 id "hack"）。摘要 §4 六项申报逐一与码核对：

| 申报 | 复核 | 判 |
|---|---|---|
| 设计锁 requeue 径不重取（窄窗） | 真：锁只在 `DispatchSeat` 且 `Phase==已立` 分支取（`dispatch.go:88-101`），回队再派不经此 | 对上，记债 |
| project 标签本批无生产写径 | 真：`SetProject` 仅测试调用，`NewTicket.project` 归 0012 | 对上 |
| REPL 先求通 | 真（票面范围外已预声明） | 对上 |
| ceoDispatched 进程内存 | 真（`gateway.go:159` 内存 map） | 对上 |
| master 7 文件 gofmt 票外 | 审计席复跑证实 7 文件 master 基线已脏、本批未带入未顺手 | 对上 |
| 0013 两项（连败原文直推飞书；requirement 路由行不落表） | 与票面附注一致 | 对上 |

审计席补充记事项（同族于已申报第 4 项，非瞒报）：`ceoPending`/`ceoFails` 同为进程内存态，
网关重启丢排队——由 `Reconcile` 对已立票再生 `spawn_needed` 兜底，窗有限。台账侧
D-14/15/22/32/34/36/37/38 行仍 open/planned，销号随判决并流（0008 先例同构）。

## 质量三问（令#E5）

- **最优解**：`ceoAPI` 薄包装解 Events 名与 AdminGateway 撞车；`specComplete` 复用注入机
  `CheckSpec` 同一把「缺一不派」尺；provenance 一律盖 run 行不采信自报——无更简等价解可见。
- **第一性**：锁＝连接登记（非 DB 旗）、抑制＝读最新事件 kind、D-22＝事件数 vs 写数纯机械
  比对——皆直击设计钦定本质，无补丁摞补丁。
- **洁净**：D-34 两处序对齐余七处；未見同逻辑双写/死码/调试残渣。
无质量驳回举证（说不出具体更优替代形制）。

## 沉淀意见（令#E8，纯意见）

摘要候选「接替席逐锚变异真跑」**可沉淀**——本审计席以约二十分钟独立复现两枚变异红
（①600s 挂起红、④0.06s 即红），是对「绿测试≠活锚」最便宜的可信化手段；「锁类测试必须
断言持锁期间第二操作拒」同理可沉淀。

## 判决

**通过**。锚 1–11 全绿（机器记录＋审计席独立复算：selfcheck、pb 逐字节、两枚变异真跑、
干净树 gateway 全包 `ok 13.678s`）；范围净、形制齐、暗债全申报对账。接替复核三修
（redirect 角色指反／设计锁瞬态持锁／import-legacy 解析器空转）各有根因节与新测试锚，
属「接替席复核出真缺陷并修复」的正向样本。票 0013 的硬依赖（proto/锁/泵）地基成立。

META_VERDICT:pass:docs/实现/票-0011-判决/1-claude.md
