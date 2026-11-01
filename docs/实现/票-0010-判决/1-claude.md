# 票 0010 判决：通过（审计席·claude 独立实例，2026-11-01）

- 审计 run：`da597699be06faee`（shell=claude），触发＝deliver（事件 112，`devin exit=0`）
- 施工 run：`9cc1f9521c1bdb1d`（shell=devin/swe-2-high，工单 `/tmp/t0010-prompt.md`）
- 交付物：支 `t/0010-deploy-gate` 单 commit `c8d1b9c`（`t0010: 部署闸+B2 代跑——闭环最后一只手…`），
  基 `8c35293`（票面 commit），不并 master ✓，ref 在 /opt/meta 库 ✓（与前批票同形）。
- 复算方法：验证命令本席在交付 worktree 重跑（记录 `/tmp/t0010-audit-verify.txt`，
  不覆写施工机器记录）；对抗锚 a–d 以交付 commit 的临时 worktree 实变异实跑
  （验后已 `git worktree remove`，分支零接触）。

## 逐锚对账（锚 1–10，全绿）

| 锚 | 机器证据 | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 施工记录 `/tmp/t0010.machine-record.txt`＝11 包 ok＋2 无测试包零 FAIL；本席重跑同命令：`BUILD_VET_OK`＋包序与施工记录逐行一致＋`SELFCHECK_OK`（R2 零 Ticker/Sleep、R3 JSON 零命中——exec.go 用 `time.AfterFunc` 一次性闹钟，exec.go:222） | 绿 |
| 2 部署支步行＋三重闸 | `TestDeploy_Gates`：claim→`deploy_start`→部署中（attach.go:159-162）；已派/待审票派 CTO→拒（dispatch.go:106 default 支）；adjunct exec→拒、未 claim（待部署相位）exec→拒（exec.go:131/139），实测 `ok meta/internal/gateway` | 绿 |
| 3 四步全绿 | `TestDeploy_FourGreen`：四步序驱＋cmd_run 四行序断言（exit/out_path/tail/diff_files 全查，diff 清单含 deploy 步 artifact.txt）→`deploy_done`→完成 | 绿 |
| 4 任红回滚 | `TestDeploy_RedRollback`：check 退 1→rollback 执行痕（cmd_run 行）→`deploy_failed`→待部署、显式断言非完成 | 绿 |
| 5 项目锁 | `TestDeploy_ProjectLocks`（二派锁拒＋`spawn_blocked` 留痕＋首票 done→tb2 收 `spawn_needed`）＋`TestDeploy_LockRebuildDB`（同库重启→部署中票锁重建→二派拒）；锁重建落点 gateway.go:246 | 绿 |
| 6 run_as | `TestDeploy_RunAs`：stub sudo 捕 argv——check=`-u seatrun`（缺省）、integrate/deploy/probe=`-u owner`（按步覆盖，runAsFor exec.go:318-323）；sudo 退 3→退码透传入 cmd_run＋exec 事件（不静默） | 绿 |
| 7 D-2 | `TestDeploy_SeatDeadD2`：kill 部署中席→`deploy_failed`＋note 含「先核回滚态」与前任 run_id（attach.go:480）＋锁释可再派；对账径同 note（reconcile.go:103） | 绿 |
| 8 健康核验 | `AdminGateway.Ping`（admin.go:163）＋`TestPing_Probe`：真编译 meta 二进制→meta-probe.sh 对活网关退 0、对死 socket 非零退 | 绿 |
| 9 路由 | `TestDeployRoutes` 直载真 `config/caster.textproto`：audit_pass→伪 Admin 捕 `Dispatch{role:"CTO"}`（行 29-31）；deploy_failed→notify；caster 消费径＝EventsAfter DB 行精确匹配（本审计席自身即经同径 deliver 路由起席——生产实效自证） | 绿 |
| 10 对抗锚 a–d（≥3 要求，4 交付） | 本席逐一变异实跑，全红：a 交换 attach.go:327-330 哨兵边映射→`FAIL: 票 tf 相位=完成 预期 待部署`；b 删 project.go:34 rollback 步→`FAIL: rollback 应跑 exit_code:-1`；c 删 dispatch.go:98 锁闸→`FAIL: 同项目二部署应锁拒`；d 翻 exec.go:139 相位判等→`FAIL: 非部署中 exec 应拒`（且 tail 显示 check 真被执行——洞实测敞开） | 绿 |

## 范围与禁线核对

- **范围外全避**：diff 无 sudoers 任何触碰（`grep -rn sudoers` 零命中）；gateway.textproto
  项目种子仅 meta 行（无 O 项目）；无周期探活（selfcheck 绿）；exec 步名白名单＝配方列
  （exec.go:155-157 `CmdFor`），配方外不得代跑；席位侧仅 `exec_req` 一径（seat.go:426-433）。
- **范围蔓延**：无。三处增量全在摘要「增量申报」节先报后落——a) `spawn_needed` CTO 路由行
  （锁释补发的消费端，闭环必需）；b) audit_pass dispatch 带 `prompt_template`
  （b023a12 止血同构，防 D-40 空提示）；c) deploy_failed 不自动补 spawn（归 CEO 裁，
  与重启对账两径一致，reconcile.go:114-117 注释在案）。票面明列的差量（rollback_cmd 入
  种子、`ExecReq.ref` 字段）均为锚 4/设计的必要件。
- **run_as 诚实妥协（设计 §11 请裁项）——本席裁：接受**。理由：不可信票码唯一落点＝
  check 步（metaseat，种子行 37 明证）；integrate/deploy/probe 三步命令为仓内版本化
  固定串（config/gateway.textproto:31-49），运行时无改配方径（SeedProject 只插不改，
  project.go:75-83），改动必过人审/审计——与审计席自身同信任链。D-24 落地后改
  step_run_as 一处即迁。此裁记入判决档供 CEO 翻案。

## 暗债对账（令#E6）

diff 全量 grep TODO/FIXME/XXX/临时/绕过＝零命中。摘要「下一步」五项与本席 diff 复核
逐条相符：sudoers 业主动作（票面要求再述 ✓）、run_as 待裁、增量 a/b/c、D-2 清＋
`SetTicketProject` 为 0011 备接点（现仅测试消费，已申报）、首单未跑（须过审＋sudoers
落位后 CTO-B 实跑——诚实）。**无瞒报**。

## 债项节（新记观察，不构成否决）

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| D-0010a | 票面范围内「deploy_failed→CEO 触发行」未落——CEO 面属被驳回的 0008，master 无 `ensureCEO/ceo_spawn_kinds` 机械可挂；锚 9 仅要求 notify（已绿），现 deploy_failed 经 notify 达业主＝过渡 CEO。归 0008 返修票补 | 部分（申报了「不自动补 spawn 归 CEO 裁」，未点明系 0008 依赖所阻） |
| D-0010b | `internal/seat` 哨兵扫描新增面（META_EXEC/META_DEPLOY，seat.go:424-446）无单测——deploy_test 直驱协议层绕过 seat.go；D-14 既有债扩大面，归 D-14（0008 返修） | 未申报（D-14 在台账已有条） |
| D-0010c | 机器记录只含 go test 段（tee 位所致），build/vet/selfcheck 输出未入档——本席重跑复核全绿，形制债轻 | 部分（摘要称绿，未附原文） |
| 观察 | `lastDeliverRef`（exec.go:291-303）对 Ref 空的 deliver 取不到 ref（施工席 `Deliver.Ref` 恒空，note 首段是 shell 名）→ fallback 实退化为空 ref；空 ref 时 integrate `merge --ff-only ""` 必红＝fail loud，实务靠 cto-deploy.md 嘱席自带 ref 兜住 | 未申报（可用性债，非正确性债） |
| 观察 | 交付新文件 gofmt 全净；`gofmt -l` 13 文件均为 master 既有（D-37 系，0008 被驳未清），本票未添新债 | 未申报（既有债非本票面） |
| 观察 | clone 复用不重 checkout（exec.go:249-251）——安全前提＝票入 待部署 后交付 ref 不再动；相位机无 待部署→施工 回边保证之。若未来开回工边须同步改 clone 重同步 | 未申报（前提成立，记录在案） |

## 质量三问（令#E5）

- **最优解**：锁取「相位占用」语义（启动按相位重建）严于设计的纯内存闸；闸拒以
  `exec_resp{exit:-1}` 回帧不断流（拒因不升级成死席）——均无更简等价形制可替。
- **第一性**：闸→查配方→降权代跑→留痕，直击 §10 机械径；无补丁摞补丁。
- **洁净**：锁三处共用 acquire/release；无死码（`deployRunSteps` 仅注释提名，无残骸）；
  测试 stub sudo「`$*` 记一行＋剥旗真跑」一并解了跑通与断言，且坑已入经验库。

## 沉淀意见（令#E8，纯意见）

「代跑闸三板斧」可沉淀：拒而不杀的回帧模式、「相位占用」锁语义、stub sudo 捕 argv
验降权——三者皆是可复用于未来帧闸/降权件的形制，非本票特化。

## 判决

**通过**。十锚全绿（含四变异对抗锚实测红）；禁线零触；增量先报后落；暗债对账无瞒报。
设计-0007 引用可达（c/0007-design 支已在 /opt/meta，0008 判决返修定向 1 已执行），
施工收口节如实申报了跨克隆读取。新记六条观察入债项节供 CEO/CTO 拣归。部署后首单
（meta 自部署）待 sudoers 业主动作落位后由 CTO-B 实跑自验。

META_VERDICT:pass:docs/实现/票-0010-判决/1-claude.md
