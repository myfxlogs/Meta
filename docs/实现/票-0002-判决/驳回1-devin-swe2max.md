Understood — read-only tools only. Starting with the mechanical materials and workspace layout.Materials loaded. Now the primary evidence — the code diff.The diff is 5,504 lines. Reading the remainder in chunks.The pb.go generated section is large. Let me jump to the remaining file list in the diff.Full diff read. Now checking git state, schema (shell/budget tables aren't in the diff — must pre-exist), and supporting files.Schema confirms `shell`/`budget`/`budget_frozen` pre-exist — no schema drift. Checking git state via `.git` file pointer.Delivery summary found in the log tail — four sections present. Now checking whether the diff includes the claimed 经验库 files, and verifying pre-existing test scaffolding (alarm/stall tests, helper functions).Helper scaffolding confirmed. Now the alarm tests (anchor 5) — checking existing coverage in `gateway_test.go` and `alarm_test.go`, plus `onStall`.Checking `onStall`, `armAlarm` call sites, and whether a progress-reset path is covered.Core logic verified. Now the remaining mechanical sweeps: forbidden patterns (R2/R3), TODO/bypass markers in the diff, and reconcile/alarm wiring.R2/R3 sweeps are clean — no Ticker/Sleep anywhere; `encoding/json` strictly inside `adapter/`. Verifying the design-doc table context for the added row, and the Alarm primitive.Verifying delivery form — branch name and commit message — via the worktree gitdir files.Checking two potential defects: whether `ResetAttemptUsage` is ever invoked (new attempts should reset per-attempt `used`), and whether `seat.ExitCodeOf` is dead code.Two findings to chase: `ResetAttemptUsage` appears unwired. Checking all callers of the budget helpers and `warned` reset behavior.Confirmed: `ResetAttemptUsage` is never called — the per-attempt reset is unwired. Checking the builder's design doc for any disclosure, then finalizing.Let me check whether the builder's log or docs mention the unwired reset path.All evidence gathered. Composing the verdict.

判决：驳回

## 逐锚核对

| 锚 | 判定 | 依据 |
|---|---|---|
| 1 build/vet/test/selfcheck | ✓ | 机器记录四段 rc=0（record 行1–16）；test 全包 ok |
| 2 适配器解析×3（录制样本） | ✓ | diff 行432–644：三壳 Build argv 断言齐（devin `--prompt-file/--respect-workspace-trust false/--export`；claude `stream-json`+stdin；opencode `run --format json`+stdin）；实录样本解析断言用量 claude=15301（959–961 样本核对 3007+12288+6）、opencode=5535（5253+26+256）、devin ATIF=24748（937–951）；不可得→false（diff 605–607）；Classify 表 9 格含 -1→①/429→②a/401→②b。记录行7 `adapter ok` |
| 3 路由处置/懒探/健康迁移逐格 | ✓ | diff 4664–4755：优先序逐格、dead→probe 无错峰、limited→probe+错峰、exclude→耗尽、NextHealth 7 格（①不迁/②a→limited/②b→dead/OK→ok）；①换壳 E2E 由 TestDispatch_SeatLifecycle 覆盖（记录行10 `gateway ok`）；耗尽事件径 dispatch.go diff 1727–1732 |
| 4 预算三闸+冻结序+取消未决 | ✓（但见 R1） | diff 1939–2030：79<80 无警→80 警边沿→100 硬停杀席回队不重派；tb2 hardcap150/used160→budget_frozen+闹钟取消（AlarmDeadline 断言）+席死+相位不动+Dispatch 拒 |
| 5 闹钟超时→事件/帧重置/重启重算 | ✓ | 超时：gateway_test.go:232 `stall_suspect` 8s 内响；帧重置：diff 2033–2055 usage 帧重置 + attach.go:157 同径 armAlarm + alarm_test Reset；重算：gateway_test.go 194–271 双子测（deadline=LastActivityAt+30min±2s）；usage 入活性事件源 ticket.go diff 1162–1164 |
| 6 对抗性锚 | ✓ | route.go diff 4623 `Priority<` 被 TestCandidates_Priority（4688–4698）盯——删/翻则序错即红；attach.go diff 1367/1369/1371 三比较各被 TestBudget_ThreeGates 对应事件断言盯（任一翻转对应 waitEvent 超时即红） |
| 7 真壳冒烟×3 | ✓ | smoke 报告行2–4：三壳 exit0＋实读用量 devin=24756/claude=15371/opencode=5594（与 testdata 值不同→真会话非重放）；seat_run 机器行（exit/dead/attached/adopted）齐 |
| 8 换壳一镜到底 | ✓* | 报告行6：2 runs＝devin exit=-1（SIGKILL→①级）→claude exit=0 交付；机制亦经机器测试复现（diff 1867–1908）。瑕疵见质询3/债项8 |

## 六质询结论

1. **总设计改动**：✓ zongdiff 全 diff 仅 1 个 `+` 行＝§5.3 表「已立」行（总设计.md:136，列结构与表一致），与授权「补已立行」逐字相符，无多改；reconcile.go:38–43 与之同步（已立→spawn_needed 补发 CTO）。
2. **对抗性锚**：✓ 两处提名行均被真盯住（上表锚6）。
3. **用量诚实**：✓ 三数值实读；「不可得」如实径在 seat.go:4894–4898（FinalTokens false→Progress「用量不可得…未估算」，非估算）。**瑕疵**：报告行7「锚8 通过」与行8「锚8 未达待审」系脚本互斥分支（diff 5482–5495），同一执行不可能并出，且缺脚本终止行「smoke: 报告=…」（diff 5502）——报告为拼装/摘录件，非单次运行原文；内嵌机器行仍支持实质达成，记债项8。
4. **暗债对账**：摘要申报 4 项与 diff 全对上（seat/cmds 无单测、stall 内部径、smoke 有界轮询）；**但 diff 内有 3 处未申报的半成品/死码**——对不上 → 令#E6，驳回主因 R1/R2。
5. **adapter 豁免**：✓ `encoding/json` 全仓仅 adapter/{claude,devin,opencode}.go 三处；selfcheck 豁免=`--exclude-dir=adapter` 仅此目录（selfcheck.sh:26）。
6. **R2/R3 全量**：✓ 全仓 .go 零 `Ticker/Tick(/Sleep`（grep 零命中）；等待件全为 AfterFunc/After 有界（oneShot/waitOnce、drain 3s、attach 3 次退避、waitAdopt grace）；smoke wait_phase 为 seq 上界轮询（脚本驱动者非系统件，有界 deadline 合法）；网关 if 全为查表/token/相位/退码/阈值/正则机械判。

## 驳回理由

**R1｜半成品未申报＋范围内语义缺陷（令#E6）** — `db.ResetAttemptUsage`（diff 行1122–1128 → internal/db/shell.go:139-141）注释自许「新尝试开始：used 清零」，全仓**零调用点**（grep 仅命中定义处）。后果确定：`budget.used` 跨尝试累计不清零 → 换壳重做（本票锚8旗舰流）的新尝试首帧用量即在 attach.go:179 `b.Used >= b.Cap`（diff 1369）被判 100% 硬停秒杀；§5.5 钦定「每次尝试内的闸」与 `Budget.Used` 注释「本次尝试已计」（diff 1098）双双落空。同族伤：attach.go:183-184 `g.warned[tid]` 只置不清 → 80% 边沿警由「每尝试一次」退化为「每票一次」。更优形制（举证）：在 `dispatchOnce` 起席成功点（dispatch.go:137 `g.onSeatAlive` 调用前）调 `database.ResetAttemptUsage(ctx, req.ticketID)` 并 `delete(g.warned, tid)`——数行接线即还原钦定语义。摘要「下一步」未申报。

**R2｜死码未申报（令#E6）** — `seat.ExitCodeOf`（seat.go:473-485；diff 5234–5245）与 `db.EnsureBudget`（shell.go:120-127；diff 1103–1110）全仓零调用，摘要「下一步」未申报。

**R3｜证据件瑕疵（记债项，非否决依据）** — 冒烟报告互斥行并存、缺脚本终止行，系拼装件（详质询3）。

## 债项

| # | 项 | 出处/去向 |
|---|---|---|
| 1 | seat 无独立单测（已申报） | live+helper 覆盖，另立票 |
| 2 | stall 事件仅内部径（已申报） | 阶段2 CTO 裁决实例接 |
| 3 | smoke 脚本相位等待为有界 deadline 轮询（已申报，本席复核合法） | — |
| 4 | cmds 无单测（已申报，决策10⑤观察项） | — |
| 5 | **新**：跨尝试预算清零未接线（used+warned） | R1，下票必修 |
| 6 | **新**：EnsureBudget/ExitCodeOf 死码 | R2，删或接线 |
| 7 | **新**：g.specs/failSet/warned/frozen 终态不清理 | 进程级内存，另立票 |
| 8 | **新**：冒烟报告须为脚本原文（含终止行与退出码），抄录层不得混行 | R3 |
