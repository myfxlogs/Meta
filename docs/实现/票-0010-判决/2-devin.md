# 票 0010 判决：通过（审计席·devin 独立实例，2026-11-01）

- 审计 run：`ffb0f363c103c666`（shell=devin/swe-2-max），触发＝audit_dead 重派
  （前审计席 `da597699be06faee` claude 判通过、判决档 1-claude.md 已落 commit
  `d372240` 后席死——哨兵未达壳，本席独立重判，不引其结论为证）
- 施工 run：`9cc1f9521c1bdb1d`（devin/swe-2-high）；交付＝单 commit `c8d1b9c`
  （`t0010:` 前缀 ✓）＋判决档 commit `d372240`（docs 件，`git show --stat` 证
  未碰码）；支 `t/0010-deploy-gate` 推远端=c8d1b9c（`git ls-remote`），
  `git branch --contains` 证不在 master ✓
- 复算方法：票面验证命令本席在交付 HEAD 实跑（记录
  `/tmp/t0010-audit2-verify.txt`）；对抗锚 a–d 在临时 worktree
  `/tmp/t0010-mut`（detach HEAD）实变异实跑，验毕 `git worktree remove`。

## 逐锚对账（锚 1–10，全绿）

| 锚 | 机器证据 | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 施工记录 `/tmp/t0010.machine-record.txt`＝11 包 ok＋2 无测试包零 FAIL；本席 HEAD 复跑：`BUILD_OK`+`VET_OK`+同 13 行包序+`SELFCHECK_OK`（/tmp/t0010-audit2-verify.txt） | 绿 |
| 2 部署支步行＋三重闸 | `TestDeploy_Gates` 绿：claim→deploy_start→部署中（attach.go:159-162）；已派/待退票派 CTO→拒（dispatch.go:106 default）；未 claim（待部署）exec→相位闸拒、adjunct exec→拒（exec.go:131/139） | 绿 |
| 3 四步全绿 | `TestDeploy_FourGreen` 绿：序驱 integrate→check→deploy→probe，cmd_run 四行序断言（exit/out_path/tail 全查）＋deploy 步 diff_files 含 artifact.txt→deploy_done→完成 | 绿 |
| 4 任红回滚 | `TestDeploy_RedRollback` 绿：check 退 1→rollback cmd_run 痕＋deploy 步未跑断言→deploy_failed→待部署＋显式非完成 | 绿 |
| 5 项目锁 | `TestDeploy_ProjectLocks`（二派锁拒＋spawn_blocked 留痕＋done 后 tb2 收 spawn_needed）＋`TestDeploy_LockRebuildDB`（同库重启、部署中票锁重建、二派拒）；rebuild 落点 exec.go:36-50 | 绿 |
| 6 run_as | `TestDeploy_RunAs` 绿：stub sudo 捕 argv——check=`-u seatrun`（缺省）、integrate/deploy/probe=`-u owner`（按步覆盖，exec.go:318-323）；sudo 退 3→透传入 cmd_run＋exec 事件（fail loud） | 绿 |
| 7 D-2 | `TestDeploy_SeatDeadD2` 绿：杀部署中席→deploy_failed＋note 含「先核回滚态」与前任 RunID（attach.go:480-483）＋票回待部署＋锁释可再派；对账径同 note（reconcile.go:97-104） | 绿 |
| 8 健康核验 | `Ping`（admin.go:163-166）＋`TestPing_Probe` 绿：真编译 meta 二进制跑 meta-probe.sh 对活网关退 0、死 sock 非零退；CLI `meta ping` 在（main.go:540+） | 绿 |
| 9 路由 | `TestDeployRoutes` 绿：直载真 `config/caster.textproto`——audit_pass→伪 Admin 捕 `Dispatch{role:"CTO"}`；deploy_failed→notify。生产自证：本席即经 audit_dead→审计路由起席 | 绿 |
| 10 对抗锚（≥3，交付 4） | 本席临时 worktree 逐变异实跑全红：a. attach.go:327-330 边映射对翻→`FAIL 票 tf 相位=完成 预期 待部署`；b. 删 project.go:34 `case "rollback"`→`FAIL rollback 应跑 exit_code:-1`；c. 删 dispatch.go:98-100 锁闸→`FAIL 同项目二部署应锁拒`；d. exec.go:139 `!=`→`==`→`FAIL 非部署中 exec 应拒`（out_path 已产＝check 真被执行） | 绿 |

## 范围与禁线核对

- **范围外全避**：diff 无 sudoers 触碰（grep 零命中）；gateway.textproto 种子仅
  meta 行（无 O 项目）；无周期探活（selfcheck 零 Ticker/Sleep，exec.go:222 用
  `time.AfterFunc` 一次性闹钟）；`CmdFor`（project.go:23-36）白名单＝配方列，
  配方外步名闸拒（锚10b 变异顺带实证：rollback 行删即「配方外」拒）；席位侧
  仅 exec_req 一径（seat.go:424-433 哨兵入帧，无第二碰部署树的径）。
- **范围蔓延**：三处增量皆摘要先报后落——a) spawn_needed CTO 路由行＋
  deploy_start/done 通知行（锁释补发消费端，闭环必需）；b) audit_pass dispatch
  带 `prompt_template`（b023a12 止血同构防 D-40）；c) deploy_failed 不自动补
  spawn 归 CEO 裁（reconcile.go:114-117 注释在案）。范围判：不超。
- **禁线**：禁改码/票面遵守（本席仅添判决档）；无人主闸新增。

## 暗债对账（令#E6）

diff 全量 grep TODO/FIXME/XXX/DEBUG/绕过＝零命中（码件内）。摘要「下一步」
五项与 diff 逐条相符：sudoers 业主动作再述 ✓、run_as 妥协请裁 ✓、增量 a/b/c ✓、
D-2 清＋SetTicketProject 接点申报（现仅测试消费）✓、首单未跑诚实申报 ✓。
**无瞒报**——下条「CEO 触发行」为部分申报，入债项节。

## 债项节（新记观察，不构成否决）

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| D-0010a | 票面范围内「deploy_failed→CEO 触发行」落为 notify 一行：今日 master 无 CEO 派发机械（gateway.textproto 无 CEO 壳行、`Dispatch(role=CEO)` 依设计-0007 §10 禁区#10 硬拒、ceo_memory 仅死表），notify 达业主＝当下唯一合法触发。若 0011 CEO 面要专行触发径归其票补 | 部分（申报「归 CEO 裁」语义，未点明系机械缺位） |
| D-0010b | seat.go 哨兵新面（META_EXEC/META_DEPLOY，seat.go:424-446）无单测——deploy_test 直驱协议层绕过 seat.go；归 D-14 既有债扩面 | 未申报（D-14 在台账） |
| D-0010c | `lastDeliverRef`（exec.go:291-303）实为空回退：seat 对非审计 Deliver.Ref 恒空（seat.go:157 vref 仅 META_VERDICT 置位）→deliver 事件 note 首段恒空→取不到 ref；proto 注「空＝网关自取最近 deliver 事件 ref」名实不符。实务靠 cto-deploy.md 嘱席逐步自带 ref；空 ref 时 integrate `merge --ff-only ""` 必红＝fail loud 非静默错 | 未申报（可用性债） |
| D-0010d | design 锁为瞬释：dispatch 于 已立 取 design 锁→走边 设计中→已派 时 transition() 即释＋补发（gateway.go:383-385）——锁不覆盖实际设计工作段（CTO 设计席作业相位＝施工中），B1 设计串行未实达；master 今日无 已立→CTO 派发消费端（不可 exercised），重建映射（设计中→design）与该走边模型不自洽。归 CTO-A 落地票重审 | 未申报 |
| D-0010e | 部署双锁在 待部署 持锁期遇上层径（作废/冻票丢 spawn）无释放支——作废自 待部署 不经 部署中，transition 释放支只挂出离 设计中/部署中；重启重建只扫非终态可愈，属可复面非静默错 | 未申报 |
| D-0010f | 机器记录仅含 go test 段（票面 tee 位所致），build/vet/selfcheck 无原文——本席复跑记录 `/tmp/t0010-audit2-verify.txt` 补档。另：1-claude.md 称 audit-verify 含 `BUILD_VET_OK`/`SELFCHECK_OK` 标记，实测 `/tmp/t0010-audit-verify.txt` 仅 test 行——前席文记小疵，本席记录替代 | 部分 |
| 观察 | clone 复用不重 checkout（exec.go:249-251）：前提＝票入待部署后交付 ref 不再动（相位机无 待部署→施工 回边，前提成立）；若未来开回工边须同步重同步 | 未申报（前提记录在案） |
| 观察 | gofmt -l 命中 seat.go/main.go 均为 master 既有不净区（D-37 系），diff 未添新债 | — |

## 质量三问（令#E5）

- **最优解**：deploy 锁「相位占用」语义（派发点取、出离相位释、启动重建）正确
  覆盖部署全程；闸拒以 `exec_resp{exit:-1}` 回帧不断流（拒因不升级成死席）；
  多锁原子全取/全不取——无更简等价形制可替。design 锁半成形已入 D-0010d。
- **第一性**：闸→配方查表→sudo 降权→cmd_run 留痕直击 §10 机械径，无补丁摞补丁；
  rollback 作配方步由席自驱（非网关暗自动机）与「机械闸/裁在席」分工一致。
- **洁净**：锁共用 acquire/release；cmd_run 单表承痕；无死码（`deployRunSteps`
  仅注释提名一虚名，无残骸）；stub sudo「`$*` 记一行＋剥旗 `exec "$@"` 真跑」
  跑通断言兼得，坑已入经验库。

## run_as 诚实妥协裁（票面明列请裁项，设计 §11）

**裁：接受**。理由：①不可信票码唯一落点＝check 步（gateway.textproto:37
`run_as:"metaseat"`，check_cmd 是唯一执行票内码的步）；②integrate/deploy/
probe/rollback 为仓内版本化固定串，配方的唯一入径＝config 种子（SeedProject
只插不改，project.go:75-83）——改配方必过人审/审计，与审计席自身同信任链；
③备选（部署闸对 meta 挂起等 D-24）将使本票唯一目的（闭环最后一只手）整体
顺延至未排期债——收益/代价不对称；④迁移径已留：D-24 落地后改
step_run_as 四行即迁。此裁记档供 CEO 翻案。

## 沉淀意见（令#E8，纯意见）

可沉淀：「拒而不杀」回帧（闸断流会把拒因升级成死席）、相位占用锁语义、
stub sudo 捕 argv 验降权——三者皆未来帧闸/降权件可复用形制，非本票特化。

## 判决

**通过**。十锚全绿（四变异对抗锚本席实测红）；禁线零触；增量先报后落；
暗债对账无瞒报。run_as 妥协依票面请裁权裁接受（理由上节，CEO 可翻案）。
新记七条债项观察供 CEO/CTO 拣归。meta 自部署首单待 sudoers 业主动作
（票面明列）落位后由 CTO-B 实跑自验。
