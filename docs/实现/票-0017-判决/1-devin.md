# 票 0017 判决：通过（审计席·devin 独立实例，2026-11-02）

- 审计 run：`9ebc8fe93bfeb871`（shell=devin/swe-2-max），触发＝deliver（事件 547，`5e9ce28 devin exit=0`）＋spawn_needed 对账补发×2（事件 577/593）→ audit_start（事件 596）
- 施工 run：`d10ca1b4c6565b7b`（shell=devin/swe-2-high），worktree `~/meta-worktrees/t0017-d42-sentinel`
- 交付物：支 `t/0017-d42-sentinel` @ `5e9ce28`（`git ls-remote origin` 核实已推＝`5e9ce288…`；父 `accab10` 在 master 链上，`git log --merges master..HEAD` 零合并未并），单 commit `t0017: D-42 判决哨兵 JSON 失明修复——…`，16 文件 +326/−35
- 机器记录：施工未落 `/tmp/t0017*.machine-record.txt`（其验证声明仅在席位日志自述）；本席照票-0012 先例在交付 worktree **原命令复算**，全量记录 `/tmp/t0017-audit-recompute.txt`（含首跑抖动与复跑裁决原文）

## 逐锚对账（票面五锚，全绿）

| 锚 | 机器证据 | 判 |
|---|---|---|
| 1 build/vet | 复算记录：`go build ./... && go vet ./...` 链式静默过→`BUILD_VET_OK`；worktree `git status --porcelain` 零行（干净交付树） | 绿 |
| 2 具名测试 | 复算 `go test ./internal/adapter ./internal/seat ./internal/inject -v -run 'Sentinel\|FrameText\|JSONShell\|MergedLine\|RawLine\|Dedup'`：10 用例全 PASS——票载六新名逐一落且断言形对上：`TestClaudeAdapter_FrameText`（assistant/result 正常+is_error/不可解析三形 `Text` 全长、360B>200B）、`TestScanSentinel_TextLines`（多行块内嵌命中、行中提及不命中、全哨兵段不报活性）、`TestRun_JSONShellSentinel`（真 claude 适配器＋sh 吐 assistant JSONL 内嵌 `\nMETA_VERDICT:pass:vr1\n`→ExitClean＋Deliver{Verdict:pass,Ref:vr1}）、`TestRun_MergedLineFileFallback`（`前缀粘文META_DELIVER:polluted` 流源不中＋sentinel.log 预置→Ref=ref7 文件净行胜）、`TestSentinelFileDedup`（流→文件双发同文 CreateTicket 帧仅 1、对称序消化、污染行不配对各发各的=2）、`TestRun_RawLineFallback`（claude 适配器遇不可解析行 Text=原行→rawref 捕获） | 绿 |
| 3 全量+selfcheck | 复算全量首跑：`--- FAIL: TestVerdictFlow`（`t0004_test.go:444` tv3 相位 10s 未达——SIGKILL 后 audit_dead 死侦计时点，属 supervise/reconcile 径，**本 diff 未触**：改面仅 adapter/seat/inject＋ceo.go 字符串字面量）；复跑全量 `RERUN_RC=0` 含 `ok meta/internal/gateway 17.607s`（共 150 用例 15 包全绿）；孤立 `-run TestVerdictFlow -count=3` 连过＋master 同试 1 过——定论**既有计时抖动非本交付回归**（该测试文件 diff 未碰）。`bash scripts/selfcheck.sh` → `SELFCHECK_RC=0` | 绿 |
| 4 契约 grep | `grep -c 'sentinel.log'`：inject.go=1、roles/审计=1、roles/施工=1、config/caster/audit-review=1（票面钦定四处全≥1）；另核 build-seat=1、cto-design=1、cto-deploy=2、ceo.go=1、三件套=1。三件套第 10 行旧表述「哨兵约定行不进 adapter」已改写为「双通道扫描」（diff 亲见旧行删新行立） | 绿 |
| 5 对抗锚三提名行（本席突变复算，scratch `/tmp/t0017-mut`，交付支未动） | a. `seat.go:520` `s.scanSentinel(seg, false)` 调用翻 `if false`→`TestRun_JSONShellSentinel` FAIL（exit 2 非 Clean）；b. `claude.go` 三处 `Text:` 赋值删→`TestClaudeAdapter_FrameText` FAIL（`len=0 预期 380`）且 `TestRun_JSONShellSentinel` 同红；c. `seat.go:300/303` 两处 `s.pumpSentinelFile()` 调用行删→`TestRun_MergedLineFileFallback` FAIL（exit 2）。删之必红，三锚俱实证 | 绿 |

## 交付形制与范围

- 支名 `t/0017-d42-sentinel` ✓；commit `t0017: <为什么>` ✓；已推（ls-remote）✓；不并 master ✓；`META_DELIVER:5e9ce28` 经旧契约径捕获（deliver 事件 ref 与 commit 短哈希对上——附录 fd-1 直写绕行本次实生效）。
- 交付摘要五节（席位日志 `run/logs/d10ca1b4c6565b7b.log` L4–36 实录）：新增/修改面、根因、验证（引机器口径）、下一步＝全量遗留申报、可沉淀打法候选——形齐。
- **范围复核**（网关 B2 前置闸未建，本席机械比对 diff 文件清单 vs 票面「范围内」）：16 文件逐一落名——adapter 四件＋adapter_test、seat.go＋seat_test、inject.go、ceo.go（仅 `renderRedirectPrompt` 内字符串字面量；「不改网关任何文件」指 Deliver/SeatAdmin 帧语义——本行是票面 §3 钦列的契约文本面）、roles×2、caster×4、三件套。范围外零蔓延。
- 禁区自查：diff 扫 `time.Ticker/time.Sleep/for{/sleep/encoding/json` 增面零命中（唯一 `for{` 命中＝测试有界 drain 循环 `select{…default:return}`，非周期源）；文件泵全挂 stdout 活性事件＋`cmd.Wait()` 后，零定时器合规（设计 §3.3/禁区#1）；`encoding/json` 仅在 adapter 包既有面（禁区#3）；go.mod/go.sum 零改动（不引第三方依赖）；无票态副本/第二写者/凭证面；devin CLI 用法零变（Build 未触）；哨兵名/相位边/判决史格式未动。

## 设计对账（设计-0016 §3）

- §3.1 Frame.Text：`adapter.go` 增字段 ✓；claude 三源（assistant text→`p.Text`、result 含 is_error→`m.Result`、不可解析→原行）✓；opencode 三源（text→`m.Part.Text`、error→`Message+Reason`、不可解析→原行）✓；devin 一切非空行 `Text=s` ✓；tool_use/system/step_finish/usage 帧不出 Text（扫描域界定防自吞）✓。
- §3.2 扫描点移位：`scanFrameText` 对 `fr.Text` 按 `\n` 逐段 `scanSentinel(seg,false)`，行内 `TrimSpace`+`CutPrefix` 语义不变、行中提及不中；哨兵段不进活性计数（`!live` 且 `Text!=""`→skip `textSeen++`，tool_use 等 Text 空帧活性不损）；`sc.Buffer` 1MB→16MB（`1<<24`）✓。
- §3.3 文件泵：`pumpSentinelFile` offset 增量读、仅 `\n` 收口完整行（`LastIndexByte` 切整，尾残留下次）、`ArtifactDir==""`/文件缺→直返；泵点＝每 stdout 行后＋`cmd.Wait()` 后净退闸判前——两调用行亲见 seat.go:300/303 ✓。
- §3.4 优先序：值类 VERDICT/DELIVER/DEPLOY 文件源落值后流源不覆盖（`verdictFile/deliverFile/deployFile` 三标记，文件后到亦可覆盖流先值＝文件权威）；动作类 RULE/ADMIN/EXEC `pairDedup` 原文整体配对（流+1/文件−1，出现序自配对）——污染流行与文件净行不配对各发各的，与设计逐字对 ✓。
- §3.5 契约面：inject 末行钦定 `META_DELIVER:<ref>` 保持字面末行（`TestAssemble_DeliverSentinelLastLine` PASS），双发表述立于末行之前；ceo.go 交付指引、caster 四件（audit-review 判决出口/build-seat 交付节/cto-design admin 节/cto-deploy EXEC+DEPLOY 节＋「写文件后继续输出」防饿泵句）、roles 两件、三件套证错修订——九面全落 ✓。

## 质量三问（令#E5）

- **最优解**：单匹配器 `scanSentinel` 两源共用、单去重器 `pairDedup` 三动作类共用、泵点复用既有 stdout 活性事件零新增事件源；`ArtifactDir`/env 注入机制现成（dispatch.go 既有）。无更简等价解。
- **第一性**：直击根因——扫解码后文本面（原错＝扫传输字节），文件信道旁路渲染层（原错＝唯一径被渲染合并/JSON 包裹）；非 workaround 摞补。文件源优先＝净行天然权威，配对键＝原文整体＝污染行天然不配对——两条判据都是结构性的非补丁式的。
- **洁净**：旧原始行 `scanSentinel(l)` 调用删净非留尸；diff 增行扫 TODO/FIXME/XXX/HACK/绕过/调试残渣零命中；值类优先序同构重复三处（verdict/deliver/deploy 各持私有状态，与设计逐字对应，可接受）；测试伪适配器改直通帧与 devin 适配器同形，测试件诚实。

## 债项节（暗债对账：diff 扫描 vs 摘要「下一步」）

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| D-0017a | 新二进制未部署前，席位仍走旧契约径——devin 壳交付哨兵须 fd-1 直写绕行（本次交付已实耗） | 已申报（部署换装不归本票） |
| D-0017b | 历史丢判不回捞（决策已定）；devin CLI ACP 接入属厂商面另案 | 已申报 |
| 观察 | `pumpSentinelFile` 每 stdout 行 `os.Open` 一次（设计钦定「零成本」径）；`pending` 对未配对动作行有界增长（哨兵条数级）；文件中途截断重写则 offset 失效（契约＝只追加，不构成面） | — |

**瞒报零起**：diff 内 TODO/绕过/半成品零命中；范围内降级/未做项零——摘要「范围内无降级/绕过/未做项」申报与本席扫描逐条对上。

## 沉淀意见（令#E8，纯意见不构成否决依据）

「解析层契约 vs 传输层契约」＋「文件兜底信道旁路一切壳渲染」——**可沉淀**：与 exec-resp.log 先例同构、offset 增量读挂既有事件源零定时器；凡对壳输出做行首/格式匹配处皆可用此双件。建议 CEO 收编打法库。

## 判决

**通过**。五锚全绿（锚 3 首跑单发既有计时抖动，复跑＋孤立×4＋master 对照全绿，与 diff 无关已裁决）；对抗锚三提名行经本席独立 scratch 突变复算删之必红；范围净、形制齐、暗债零瞒报；成品与设计-0016 §3 逐条对上。

META_VERDICT:pass:docs/实现/票-0017-判决/1-devin.md
