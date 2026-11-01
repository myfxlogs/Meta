审计席启动。按材料清单并行读取所有证据件。材料已读四件。增量 diff 显示 R1 接线（spawn 前清零+warned 复位+失败摆位）、R2 两处处置、R3 脚本改 main|tee 原文落盘。现核全量 diff 上下文、工作树实况与 git 态。施工日志揭示关键情节：返修轮 claude CLI 未登录→exit=63→②b 标死→opencode 递补；摘要如实申报缺口，CEO 补登录后复跑拿到 exit=0 报告（rundir uT7OtQ，用量数值与上轮不同→真会话）。现核代码实况与 git 态。代码实况已核：reset 位于候选循环内 trySpawn 前（dispatch.go:137-149），warned 删除在 g.mu 下与 attach.go:183 同锁；ExitCodeOf 四格映射与 ClassOfExit 互逆，EnsureBudget 全仓零引用。git HEAD=t/0002-shell-adapters。

还剩一个关键质疑点：测试里第二次 `waitEvent(budget_warn)` 是否会命中**第一次尝试的旧警事件**而假绿？须查 `waitEvent`/`collectEvents` 实现的时间界。`waitEvent` 实读 `g.EventCh()` 活流且消费式匹配（helper_test.go:278-295）——第二次等 `budget_warn` 只能命中**新发出**的事件，旧警已在第一次 waitEvent 被消费；`warned` 不复位→无新警→超时即红。第二次警的断言是真鉴别，非恒真。同理 collectEvents 负向断言盯死 `budget_stop`/`budget_frozen`。

再补三处：`AddUsage` 语义、`emit`/`EventCh`、reconcile 的 spawn_needed 补发径是否可能触碰活席。收养径核实：adopt 成功续同一尝试（不走 dispatchOnce→不清零，语义正确）；adopt 失败回队→spawn_needed→新尝试清零。再核 `AddUsage` 语义与 `emit` 冻票过滤。全部证据核讫。整理判决。

---

判决：通过

## 逐锚核对（b3ccd8a · 机器记录四段 rc=0）

| 锚 | 判定 | 依据 |
|---|---|---|
| 1 build/vet/test/selfcheck | ✓ | record 行1–16 全 rc=0；test 全包 ok，gateway 9.998s（含新测 TestBudget_AttemptReset） |
| 2 适配器解析×3 | ✓ 无回归 | adapter/ 不在增量 diff 范围，一审已核断言矩阵；`internal/adapter ok`（record 行7） |
| 3 路由逐格 | ✓ 无回归 | route.go/dispatch 既有路径未动（增量仅插入 13 行接线段）；`internal/route ok` |
| 4 预算三闸＋冻结序 | ✓ 无回归＋加固 | TestBudget_ThreeGates/冻结测保留（delta 行76–78 上下文）；attach.go:177–181 三比较原样；新增跨尝试用例补上盲区 |
| 5 闹钟 | ✓ 无回归 | 闹钟件不在增量；attach.go:170 用量帧重置径原样 |
| 6 对抗性锚 | ✓ | 提名行（route Priority 比较、attach.go:177/179/181 三阈值比较）增量未触仍被原测试盯；新接线行自身亦被新测盯（测试注释声明删接线/删 warned 复位必红，鉴别力核实见必核项4） |
| 7 真壳冒烟×3 | ✓ | 报告行2–10：三壳各具名收尾 exit=0，用量实读 devin=23682/claude=17959/opencode=5370——与上轮值（23229/—/5368）及 testdata 值均不同，系新真会话非重放 |
| 8 换壳一镜到底 | ✓ | 报告行11–13：devin 席 pid=2626693 SIGKILL→exit=-1 dead=true→claude pid=2626732 exit=0 收尾，2 runs；终止行 `smoke: exit=0` |

## 驳回项闭合

**R1 闭合——接线真、点位偏差裁定＝等价偏优**。dispatch.go:140–149：`ResetAttemptUsage` 位于候选循环内 `trySpawn` 之前、`delete(g.warned)` 与 attach.go:182–192 的 `warned` 写入同锁 `g.mu`。偏差裁定：一审举证点（`onSeatAlive` 前）残留窄竞——`trySpawn` 经 `attachedCh` 返回时快壳首帧用量可先落库（95+10=105≥cap→新席在清零前即被硬停，R1 病灶复现）；spawn 前清则语义同「新尝试起点」且窗口为零。**举证点位本身带残竞，施工点位严格更优，裁定采纳**。失败径（:140–145）：事件＋摆位不进 failSet——库错非壳错，合理。收养径（reconcileActor→tryAdopt）不经 dispatchOnce，续跑同尝试不清零，语义正确。

**R2 闭合**。`EnsureBudget` 删净：全仓 grep 零代码引用（仅经验库文档提及）；`SetBudgetCaps` 为真 upsert（ON CONFLICT DO UPDATE，shell.go:121–127），「无行则建」覆盖原语义，admin.go:23/46 两调用点自足。`ExitCodeOf` 接进 seat.go:155 收尾退码——非 OK 分级走单表，与 `ClassOfExit`(:483) 互逆；映射四格（Transient/Fatal/OK/default）与被删 switch 逐格等价。

**R3 闭合**。脚本改 `main | tee "$REPORT"` + `PIPESTATUS[0]`（smoke-shells.sh:115–116）；报告＝单次运行 stdout 原文：各行与脚本 echo 模板逐行对得上、rundir 自洽、终止行（含 `exit=0` 与三路径）在，无一审互斥行并存痕。锚7 判据收紧为「具名壳收尾」（:65 `shell=${SH}([/[:space:]]|$)` 兼容 `devin/swe-2-high` 形）——本轮实证价值：claude 未登录轮即靠此判据如实报未过。

## 必核项结论

1. **R1**：见上——接线真、偏差等价偏优、测试断言真（三点全断：首帧不秒杀/二警再发/used_total=175 单调）。
2. **R2**：删净＋接线合理，见上。
3. **R3**：原文报告，终止行与退出码在，无拼装痕。
4. **变异鉴别力**：真。`waitEvent` 消费式读 `g.EventCh()` 活流（helper_test.go:287–290），旧警已被首次调用消费——删 `delete(g.warned)` 则第二次 waitEvent 必超时红；删 `ResetAttemptUsage` 则 105≥cap→`budget_stop` 被 collectEvents 捕获红（且 `b.Used!=10`/`processAlive` 双备断）。施工自述的临时摘接线变异与测试设计独立吻合。
5. **暗债对账**：增量五文件均在范围内；无 TODO/绕过/新死码；「下一步」五项申报与 diff 全对上（claude 环境缺口如实报→CEO 补登录复跑补齐，证据即本报告）。无瞒报。
6. **全锚不回归**：抽验三处——route 优先序比较未触、attach.go:177–181 三阈值比较原样、attach.go:170 闹钟重置径原样；增量未碰任何一审已核面。
7. **形制**：支 `t/0002-shell-adapters`@b3ccd8a（父 816397c 与增量基吻合），commit `t0002: 返修驳回1——…` 合 `t0002: <为什么>`；摘要四节在；经验库坑文入库（令#E1）。

## 债项（令#E6）

| # | 项 | 状态 |
|---|---|---|
| 延续 | seat/cmds 无独立单测；stall 仅内部径；specs/failSet/warned/frozen 终态不清理 | 一审台账延续，已申报，另票 |
| 新 | 跨尝试边界残竞：死席在途 Usage 帧可能晚于下尝试清零落库（票级行按时间分尝试，非按 run 键）。窗极窄，一审举证点同患；按 run 键分账属设计层改动，出本票范围 | 观察项，建议下票评估 |
| 新 | smoke 脚本 `LAST` 未入 local 声明、`FIRST` 声明未用（:39,62） | 瑕疵级，顺手清即可 |
| 新 | `shell_recovered` 事件措辞（懒探成后再死先报恢复） | 施工已申报未入码，下票评 |
| 环境事实 | live 锚依赖本机壳登录态——claude 凭证失效则锚7[claude]/8 如实红（本轮已实证如实径生效） | 非码债，提示运营 |

## 根因节一句评（令#E4 落实度）

根因答到机制层（零调用静态隐形＋单尝试测试盲区＋自查缺 grep 步）并固化为经验库自查动作——非「疏忽」式搪塞，落实合格。
