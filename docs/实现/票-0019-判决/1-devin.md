# 票 0019 判决：通过（审计席·devin 独立实例，2026-11-02）

- 审计 run：`a4ce680b1e9af0ae`（shell=devin/swe-2-max），触发＝deliver（事件 806，`09bde93 devin exit=0`）＋spawn_needed 对账补发（事件 888）→ audit_start（事件 1145）
- 施工 run：`703706505f350e31`（shell=devin/swe-2-high），worktree `~/meta-worktrees/t0019-utf8`
- 交付物：支 `t/0019-utf8-sanitize` @ `09bde93`（`git ls-remote origin` 核实已推同哈希；父 `e453124` 在 master 链上，`git log` 单 commit 未并 master），23 文件 +626/−75
- 机器记录：施工未落 `/tmp/t0019*.machine-record.txt`（验证声明在席位日志 `run/logs/703706505f350e31.log` 自述）；网关 `cmd_run` 表对本票零行（机械前置闸未建）；本席照票-0012/0017 先例在交付 worktree **原命令复算**，全量记录 `/tmp/t0019-audit-recompute.txt`
- 票面说明：`docs/实现/票-0019-*.md` 未落盘——spec 存 `run/meta.db` ticket.spec BLOB，本席经 `sqlite3` 直读八栏全文。票面引用「设计全案＝`docs/实现/设计-0018-入账消毒三层.md`」不在 master——**实存于 `c/0018-design` 支**（票 0018 施工中未并）；施工申报「仓内不存在」对 master/交付树成立、对整仓不全准。设计内容经票面八栏内联＋设计支档双重可核，本判决逐条对设计支档。

## 逐锚对账（票面五锚，全绿）

| 锚 | 机器证据 | 判 |
|---|---|---|
| 1 build/vet | 复算记录：`go build ./... && go vet ./...` 链式静默过→`BUILD_VET_OK`；交付 worktree `git status --porcelain` 零行（干净树） | 绿 |
| 2 具名测试×11 | 复算 `go test ./internal/db ./internal/adapter ./internal/gateway ./internal/seat ./internal/inject -v -run '<11名>'`：全 PASS——`TestClean_Cargs`（string→�/int64/[]byte/bool/nil 五型判别）、`TestSanitize_AppendEvent`（毒 note 回读合法＋含�＋行在）、`TestSanitize_SpecBlob`（伪 wire 拒写且列空＋合法 spec 回读 unmarshal＋CreateTicketProv 同闸）、`TestPreview_UTF8Boundary`（毒→合法＋30×「界面」→≤20B＋…＋不残 rune）、`TestDevinParse_BadLine`（{0xff,'x'}→Note 合法）、`TestLogTail_Clean`（毒尾文件→合法串）、`TestEventsAfter_BadRowMarked`（真 socket，二连接直插毒→无 Internal＋全字段合法＋Kind=progress 非 bad_row）、`TestObserve_BadEventMarked`（EventCh 段锚 emit＋真 socket 段锚流发送）、`TestScanSentinel_BadLine`（DELIVER+VERDICT 双哨兵毒行）、`TestParseTicketMD_BadBytes`（十字段合法＋marshal 成）、`TestSendPump_SendErrLogged`（伪流恒败→stderr 有痕） | 绿 |
| 3 全量+selfcheck | 复算 `go test ./...`：14 包全 ok 零 FAIL（`ok meta/internal/gateway 17.987s` 等）；`bash scripts/selfcheck.sh`→`SELFCHECK_RC=0` | 绿 |
| 4 对抗锚六提名行（本席 scratch `/tmp/t0019-mut` 突变复算，交付支未动） | ①`ticket.go` appendEvent `cargs(`→裸实参：`TestSanitize_AppendEvent` FAIL「回读 note 仍含毒 "x\xff\xfea\x80y"」；②`ticket.go` UpdateSpec `validSpecBlob(` 闸删：`TestSanitize_SpecBlob` FAIL「伪 spec wire 应拒写」；③`adapter.go` preview `ToValidUTF8` 删：`TestPreview_UTF8Boundary` FAIL「preview 出毒」＋`TestDevinParse_BadLine` FAIL「帧 Note 含毒」双红；④`admin.go` eventItem 净化行删：`TestEventsAfter_BadRowMarked` FAIL「毒行被顶替（应净化非 bad_row）」——净化径/顶替径区分断言实生效；⑤`gateway.go` emit 净化行删：`TestObserve_BadEventMarked` FAIL「emit 出毒」；⑥`seat.go` scanSentinel 首行净化删：`TestScanSentinel_BadLine` FAIL「deliverRef 含毒 "ref\xff tail"」 | 绿 |
| 5 喂毒端到端（决策 68 钦定） | 票载钦定＝`TestEventsAfter_BadRowMarked`（二 sql 连接绕净化直插毒 note 模拟存量毒→真 socket EventsAfter 无 Internal＋note 合法）＋`TestSanitize_AppendEvent`（入账口净化→库内合法 UTF-8 含�）——两层各一，已含且复算 PASS | 绿 |

## 交付形制与范围

- 支名 `t/0019-utf8-sanitize` ✓；commit `t0019: <为什么>` ✓；已推（ls-remote 同哈希）✓；未并 master ✓；`META_DELIVER:09bde93` 哨兵于席位日志末行亲见，deliver 事件 ref 与 commit 短哈希对上。
- 交付摘要五节（席位日志实录）：新增/修改面、根因、验证、下一步＝全量遗留申报、可沉淀打法候选——形齐。
- **范围复核**（B2 前置闸未建，本席机械比对 diff 23 文件 vs 票面方案栏）：`internal/db/` sanitize.go（新）＋ticket/shell/run/project/budget/secret/memory 写径、`internal/gateway/` sanitize.go（新）＋admin/ceo/dispatch/exec/gateway、`internal/adapter/`、`internal/seat/`、`internal/inject/`、`internal/board/`＋四件测试档——逐一落名票面钦列点，范围外零蔓延。边界外项全守：历史数据未动、角色卡/注入模板/路由表/proto/schema 零碰、文件原文照存面（Classify tail/relayExecResp）未触、go.mod/go.sum 零改动（stdlib `ToValidUTF8`）、零定时器零轮询、未部署未并 master。
- 票面钦定「26 点」实证：本席 python 扫 `internal/db` 全部 `ExecContext/QueryRowContext/Exec` 写径站点＝26（含 `IncrementDefer` 写式 `UPDATE…RETURNING` 与 Transition 两笔 `tx.ExecContext`），**26/26 实参过 cargs**；schema/ALTER/VACUUM 三站于 db.go 排除合规；残余 `QueryRowContext` 全为 SELECT 读径（票面只管写径）。
- 覆盖一致性补查（同族面）：票面字面清单外 `PhaseMsg.Phase` 亦过 clean——同层 DB→proto 面主动覆盖，判为合理同族收口非蔓延。`MemoryOp_Put` 回显 `b.Put.*` 未净化：proto 入侧 unmarshal 已强验 UTF-8（bytes Content 不验亦不炸 marshal），不构成漏面。`ceo_say` 事件 `Note:string(Put.Content)` bytes→string 毒道经 `emit()` 收口覆盖。

## 设计对账（c/0018-design 支 设计-0018 §2/§3）

- §3.1 层一：`sanitize.go` `clean/cargs/validSpecBlob` 三件套逐字落（空 spec→原样、unmarshal 败→err、成→canonical marshal 回写）；26 写径全覆盖同上；`PutMemory` content `[]byte(clean(string))` 按文本净化而 BLOB 形保真 ✓。
- §3.2：`ParseTicketMD` 十字段逐一 `ToValidUTF8`（8 栏＋appendix＋verdicts）✓。
- §3.3：`preview` 净化先行＋≤n 字节内 `utf8.ValidString` 逐字节回退 rune 边界＋「…」✓。
- §3.4：scanSentinel 首行净化 ✓；sendPump Send 败 `Fprintln(os.Stderr)` 后 return ✓；adminSentinel create_ticket `err==nil` 才赋 Spec、败→nil＋stderr 留痕（不存部分 wire）✓。
- §3.5：logTail/tailOfFile 出口净化 ✓；eventItem 八字段净化＋`eventItemChecked` 逐 item marshal 预检→`{Id,Kind:"bad_row",Note:"bad_row: 事件行 marshal 败（存量毒）"}` 顶替 ✓；Runs RunItem 净化＋预检→`{RunId:"bad_row",Dead:true}` ✓；Observe ObsEvent 净化＋预检→bad_row 顶替照发不返错 ✓；emit 入口六字段净化（进程内单收口，pending 重放同免疫）✓；CeoEvents `send` 闭包预检→bad_row 帧（pending 重放同径）✓；CeoMemory get MemoryRow 字段净化（Content bytes 免，与设计逐字对）✓；board seatRow/ticketRow DB 源字段净化 ✓。
- §6 测试母本：11 用例全落名且断言形对上（净化径 vs 顶替径区分、喂毒直插、真 socket 两端）。

## 质量三问（令#E5）

- **最优解**：`cargs` 变参覆盖式收口＝新增写径忘消毒的敞口在调用点机械消灭（优于逐列挑的等价解——后者每加一列多一处可漏点）；`emit()` 单收口免疫全部订户与 pending 重放（优于逐订户净化）；`validSpecBlob` 结构闸在仅存两个 spec 写点。无更简等价解。
- **第一性**：直击根因链「外部字节→DB TEXT 不验→proto 强验必炸」的两个收口点（写面＋proto 构建面），不追无限上游源——与设计「净化必须在收口点，追源无解」逐字对。bad_row 顶替＝「跳过留痕不静默」结构性兜底，非 workaround。
- **洁净**：`clean` 三行同名件跨 db/gateway/board 三包各一（Go 包界使然，单点三行不引共享包属合理）；marshal 预检净化后结构性不可达＝防御纵深死径，摘要已申报；diff 增行扫 TODO/FIXME/HACK/调试残渣零命中（唯一「绕过」命中＝测试注释描述直插毒方法，非产品绕过）；无重复实现、无留尸。

## 债项节（暗债对账：diff 扫描 vs 摘要「下一步」）

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| D-0019a | 存量毒数据未动：live `meta.db` 仍含毒行，`meta events --ticket 0018` 此刻仍 Internal 崩——须部署本支后方解（票面边界外钦定不动历史数据） | 已申报（下一步#1） |
| D-0019b | 0018 与 0019 票题同域各挂活席（0018=CTO 席）——疑似重复派工 | 已申报（下一步#2，转 CEO/CTO 核） |
| D-0019c | 票面引用设计档不在 master（实在 `c/0018-design` 支） | 已申报（下一步#3，称「仓内不存在」——对 master/交付树成立，对整仓表述欠准，记此校正） |
| D-0019d | `seat.go:418` `_ = conn.Close` 方法值非调用，conn 实际不关（GC 兜底）——既有小疵，diff 未触该行进而在 diff 上下文内亲见 | 已申报（下一步#4） |
| D-0019e | 管理面应答 note 同族未净：`Ack{Note: err.Error()}` 多处、`ExecResp.Note`（deny 文本拼 DB 串 `t.Project` 等）、`CeoTokenResp` RunId/RunToken——err/应答文本含毒概率低但属同族 | 已申报（下一步#4「Runs/Phase/Budget 之外的管理面应答 note 未逐点净化」——本席补列 ExecResp.Note/CeoTokenResp 同属此族） |
| D-0019f | marshal 预检于净化后结构性不可达＝防御纵深死径 | 已申报（下一步#5，明示非缺陷） |
| 观察 | `TestSendPump_SendErrLogged` 全局换 `os.Stderr`（测内恢复，非并行测安全）；Observe 测 100 次有界重发等订户登记（10ms×100 上界 1s，测试域有界） | — |

**瞒报零起**：diff 内 TODO/绕过/半成品零命中；范围内降级/未做项零——六条申报与 diff 逐条对上。

## 沉淀意见（令#E8，纯意见不构成否决依据）

「收口点净化」三件套（写径 cargs 覆盖式＋BLOB validSpecBlob 结构闸＋流出面「净化＋逐 item marshal 预检 bad_row 顶替」双保险）＋对抗锚「净化径/顶替径分开锚才不互相掩护」——**可沉淀**：凡「弱校验存储→强校验传输」链路皆可用此三层形制；建议 CEO 收编打法库。

## 判决

**通过**。五锚全绿（11 具名测试逐一落名 PASS；六条对抗提名行经本席独立 scratch 突变复算删之必红）；26 写径 cargs 机械点验全覆盖；范围净、边界外项全守、暗债六条零瞒报（D-0019e 属申报族内补列非漏报）；成品与设计-0018 §2/§3/§6 逐条对上。

META_VERDICT:pass:docs/实现/票-0019-判决/1-devin.md
