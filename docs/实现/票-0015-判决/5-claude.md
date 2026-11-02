# 票 0015 判决：通过（审计席·claude 独立实例 #5，2026-11-02）

- 审计 run：`91bce729763f65ee`（shell=claude，本席 `/proc` 亲证父链
  `meta serve 3255348`→`meta seat 3367193`→`claude 3367201 -p --output-format
  stream-json`，seat fd/8＝claude fd/1＝`pipe:[71324290]`＝扫描管）。
- 触发＝**audit_dead**（第四席 `d03c404c2b8e1a50` 有档无哨兵死席）。环境注记：
  `meta events --ticket 0015` 现报 `grpc: error while marshaling: string field
  contains invalid UTF-8`（已知 DB spec text 化债，D-42 族）——事件史本席不可读，
  判决史以前档＋派发前缀为据；`meta phase` 可用：`0015 审计中 frozen=false`。
- 前情：判决 1（1-devin）／2／3／4（2-4=claude）均结论「通过」但四席皆
  「有档无哨兵」死席（D-42 壳失明：哨兵嵌 stream-json 串内，外壳
  `CutPrefix` 不命中——seat.go:467）。本档＝第五席**从零独立复算**（锚
  1/2/11 全部本席亲跑，不引前档结论）。
- 施工 run：`e0b4355c7faac3a4`（devin/swe-2-high），交付＝支
  `t/0015-project-link` @ `a5b3aaa`（码面双 commit b7335de＋a5b3aaa，格式
  `t0015:` ✓），22 文件 +1378/−314。
- 复算基：交付 worktree @`3539acb`（＝a5b3aaa＋四判决档；`git diff --stat
  a5b3aaa..3539acb`＝4 个 md 档纯 docs，码面等同 a5b3aaa）。
- 形制：`git ls-remote origin t/0015-project-link`＝`3539acb` 已推 ✓；
  `git merge-base --is-ancestor a5b3aaa master` 假＝未并 master ✓；交付摘要
  五节（新增/修改面、根因、验证引、下一步全量遗留申报、可沉淀打法候选）
  ＋`META_DELIVER:a5b3aaa` 哨兵——本席直读施工 run 日志尾
  （`run/logs/e0b4355c7faac3a4.log`）独立核实 ✓。

## 逐锚对账（票面 11 锚，全绿；锚 1/2/11 本席独立机器复算，锚 3-10 测试断言本席亲读＋全量绿）

| 锚 | 机器证据 | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 本席复跑 @3539acb（`/tmp/t0015-audit5-full.log`，2026-11-02T06:42:37+08:00）：`go build ./... && go vet ./...`→BUILD_VET OK；`go test -count=1 ./...`→15 包全 `ok`（gateway 18.160s）TEST_EXIT=0；`bash scripts/selfcheck.sh`→SELFCHECK_EXIT=0（R2/R3 零命中）。包集与施工机记 `/tmp/t0015.machine-record.txt`（15 包 ok）一致 | 绿 |
| 2 pb 三件逐字节 | 本席隔离场（detached worktree `/tmp/t0015-audit5-mut` @3539acb）原位再生：`protoc 3.21.12`（/usr/bin）＋`~/go/bin` 三插件（gen-go v1.36.12 文件头同版）`-I proto`＋`module=meta` 再生三件→`git status --porcelain -- internal/pb proto` **零行**＝byte-identical，无手改 pb | 绿 |
| 3 NewTicket.project 立票径 | admin.go 前置登记校验（未登记→`Ack{Ok:false}` 票不立）＋`CreateTicketProv(..., nt.Project)` 同 INSERT 原子落＋emitProjectSet；`TestAttach_CreateTicket`（t0015_test.go:82-109）断言未登记拒＋票不立＋落 P＋留痕 1 行——本席亲读断言＋全量绿 | 绿 |
| 4 派发径挂接＋锁前序 | dispatch.go:66-74 attach 块位于 :63 BudgetFrozen 后、:103-104 lockGate（`acquireProjectLocks(t.Project)`）/:123 designLocks **之前**（本席亲读行位）；`TestAttach_Dispatch`（:114-175）断言 project=="P"、事件 actor=="gateway"/prov=="test:disp"、待部署 CTO 票后 `projectLocks["P"]` 占 integrate+deploy、`""` 键桶空、同值幂等零事件 | 绿 |
| 5 席内 create_ticket | ceo.go handleAdmin：空→继承本票 `pt.Project`、非空→GetProject 校验拒（票不立）、原子落＋留痕；`TestAttach_SeatAdmin`（:224-250）三分支断言 | 绿 |
| 6 席内 set_project | ceo.go 越票判（`sp.TicketId != st.run.TicketID`→false）→相位闸（非施工中拒）→attachProject；`TestAttach_SeatAdmin`（:195-223）越票 false＋本票落 P＋project_set 事件在 | 绿 |
| 7 SetTicketProject RPC 五判 | admin.go 新 RPC→attachProject（project.go 五判：不存在/未登记/部署中·终态/幂等/换挂，拒亦 `denied:` 留痕）；`TestAttach_SetTicketProjectRPC`（:256-316）七情形全断言（含幂等后成功行仍 1） | 绿 |
| 8 Requirement.project | ceo.go CeoSubmitRequirement 未登记→RPC 层 FailedPrecondition 需求不立；`TestAttach_Requirement`（:320-343）落 P＋留痕 1 行 | 绿 |
| 9 CLI 四面 | main.go:424-521 ticket 子命令拆分＋`set-project` 新子命令＋new/dispatch/require 三旗＋用法行同步；`TestCLI_ProjectLink`（main_test.go:230-326）真网关 socket 四径落库断言——强于票面「桩断言 RPC 载荷」；seat 五段哨兵 `TestScanSentinel_AdminProject`（seat_test.go:137） | 绿 |
| 10 事件＋emit＋caster 映射 | `TestAttach_ObserveEmit`（:347-364）waitEvent 观察者断言＋ticket_event 行；`TestDispatchAction_Project`（caster_test.go:514-547）DispatchAction.project→DispatchReq.Project 映射＋project_set notify；caster.textproto project_set→notify 行在 diff | 绿 |
| 11 对抗锚变异必红 | **本席五变异全红全回退全回绿**（票面要求 ≥3；明细下节） | 绿 |

## 变异复算明细（隔离场 `/tmp/t0015-audit5-mut` @3539acb；每枚：python 整块变异→go vet 过→定向测试→`git checkout` 回退；交付 worktree 全程未染，场毕销场 prune）

- **a**. project.go 换挂判 `t.Project != ""`→`==` → `TestAttach_SetTicketProjectRPC` FAIL `t0015_test.go:283: 空→P 应放: note:"票 tp 已挂 ——换挂非本径"`
- **b**. dispatch.go attach 块（注释＋6 行）整体挪至 EnsureBudgetDefaults 前（全部锁闸后）→ `TestAttach_Dispatch` FAIL `:158: projectLocks[P] 应占 integrate+deploy: map[]`（锁落旧 "" 键——与票面预测逐字吻合）
- **c**. project.go 删闸内 GetProject 登记校验块 → `TestAttach_SetTicketProjectRPC` FAIL `:277: 未登记应拒`
- **c2**. admin.go 删 CreateTicket 立票复制件校验块 → `TestAttach_CreateTicket` FAIL `:93: 未登记项目应拒`
- **d**. ceo.go 删 set_project 越票判块 → `TestAttach_SeatAdmin` FAIL `:199: set_project 越票未拒`

回绿实证：五枚回退后 `go test -run 'TestAttach_' ./internal/gateway` → `ok 0.410s` exit 0；场内 porcelain＝0（唯一残物＝本席自建脚本，移出后净）。

## 范围核对

码面 22 件全落票面「范围内」清单（proto＋pb 三件、db/ticket.go、gateway 四件 admin·ceo·dispatch·project、seat.go、cmd/meta/main.go、caster.go、caster.textproto、cto-design.md、事件模型.md、九件测试）。清单外三件均义务件非蔓延：`债务台账.md` D-44 行＝设计 §5 钦定记债（diff +1 行 open，本席核文）；经验库新文＝令#E1 收口门（内容与变异坑实证相符）；`ceo_test.go` SetProject→SetTicketProject 改名＝票面「去重留一」随行（build 绿＝`db.SetProject` 尸体零残调用的编译级证明）。禁线零触：`META_ADMIN:dispatch` 哨兵格式零改（seat.go dispatch case 未动，SplitN 4→5 只影响第 5 段解释）、路由表 dispatch 行未配 project（textproto 仅 +project_set notify 行）、sudoers 未碰、未并 master、diff 无回填执行件、无 E17 批闸/O 项目登记触碰。

## 债项节（暗债对账：零瞒报）

diff 增行 grep `TODO|FIXME|XXX|HACK|占位|绕过|临时|调试`＝**零命中**（本席实测，grep rc=1）。摘要「下一步」五条逐条对账：

| 摘要「下一步」申报 | 对账 | 判 |
|---|---|---|
| 存量票回填执行（部署后 set-project→meta） | 票面范围外明列＋设计 §4 自举序，非本票码面（diff 无执行件） | 账实符 |
| D-44 按票项目注入配方 | 台账 open 行 diff 实证在账 | 账实符 |
| 立票径登记校验复制件 | 结构必然（票未生无 ticket_event 可挂、不能过闸），经验库文在 a5b3aaa | 账实符 |
| denied: 拒留痕 | 设计 §5 落点，project.go deny 闭包 diff 实证 | 账实符 |
| 0013/0014 合并面 rebase 义务 | 票面附注惯例申报，本支未并 master（merge-base 非祖先） | 账实符 |

## 质量核三问（无驳回级发现）

- **最优解**：attachProject 五判与设计 §2 逐字对应，六径共用一把闸；project 与票行同 INSERT 原子落（非先立后 UPDATE）；未找到更简等价解。
- **第一性**：挂接点真在取锁闸前（变异 b 实证）；`SetProject`/`SetTicketProject` 同义重复去重留一，尸体删净。
- **洁净**：生成件零手改（锚 2）；增行零残渣（grep 实证）；无死码（旧调用点随行改）。

观察项（不构成驳回）：①票面锚 11c 字面「登记校验删→**锚 3** 红」指称偏差——删**闸内**校验红的是锚 7（:277），锚 3 红须删 admin.go 复制件（c2 实测红）；两处校验各有独立钉锚、变异必红实质达成（判决 2-4 同点已 flag，票面小瑕回 CTO 知悉即可）。②`set_project` 越票判删后写目标仍绑 `st.run.TicketID`——残余保护严于票面语义，判等仍必要且已钉锚。③四任审计席皆「有档无哨兵」死席——流程债（D-42 族）不在本票施工面，本席送达径见文末注记。

## 可沉淀打法候选（意见，非否决依据）

**可沉淀**（同判决 1-4 意见，本席五变异再证）：「共享机械闸＋票未生径前置校验复制件＋两处各钉对抗锚」双层模式；`docs/经验库/2026-11-02-共享闸前置校验是复制件-变异要打在覆盖它的测试上.md` 一文准确可复用——本席 c/c2 双变异各自红在各自的锚上，实证该打法有效。

## 判决

**通过**。11 锚全绿（锚 1/2/11 本席独立机器复算：15 包 ok＋selfcheck 0、pb 再生 porcelain 零行、五变异全红全回退全回绿；锚 3-10 测试断言亲读＋全量绿）；范围净；形制全（支名/commit 格式/已推/未并 master/摘要五节＋META_DELIVER 哨兵）；暗债零瞒报。四任死席「通过」结论获本席第五席独立佐证。

## 送达注记（本席机械实事，非判决依据）

哨兵回流路径：本席最终消息末行输出 `META_VERDICT:pass:docs/实现/票-0015-判决/5-claude.md`；因现行外壳扫描面对 stream-json 壳失明（D-42，四任死同因），本席另将同一哨兵行**直写 claude CLI stdout 扫描管**（`/proc/3367201/fd/1`＝`pipe:[71324290]`，前后带换行独占一行）——同 req-9ce5 第九席（8-claude.md 送达注记）与施工席 ca93f994 先例。落管回执＝席位日志 `run/logs/91bce729763f65ee.log` 内 `^META_VERDICT:` 独占一行。判决内容与档内一致，无虚报。
