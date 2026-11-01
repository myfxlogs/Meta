# 票 0015 判决：通过（审计席·claude 独立实例 #4，2026-11-02）

- 审计 run：`d03c404c2b8e1a50`（shell=claude），触发＝audit_dead（事件 765——第三审计席 `640edddf0e7d8148` exit=2 净退无哨兵，事件 764/765）。
- 前情：判决 1（`1-devin.md`，run `01436fbea2bb76ef`）、判决 2（`2-claude.md`，run `e0cf7e5923a12296`）、判决 3（`3-claude.md`，run `640edddf0e7d8148`）均结论「通过」但三席皆「有档无哨兵」死席＝半交付不采信。本档＝第四席**从零独立复算**（未引前档结论，仅对同一 diff 与机器证据；锚 1/2/11 全部本席亲自复跑）。
- 施工 run：`e0b4355c7faac3a4`（devin/swe-2-high），交付＝支 `t/0015-project-link` @ `a5b3aaa`（码面双 commit b7335de＋a5b3aaa，格式 `t0015:` ✓），22 文件 +1378/−314。
- 复算基：交付 worktree @`5cbe85b`（＝a5b3aaa＋三死席判决档；`git diff --stat a5b3aaa..5cbe85b` 仅 3 个 md 档＝docs-only，码面等同 a5b3aaa）。
- 形制：`git ls-remote origin t/0015-project-link`＝`5cbe85b` 已推 ✓；master 未并（`git merge-base --is-ancestor a5b3aaa master` 假）✓；交付摘要五节（新增/修改面、根因、验证引、下一步全量遗留申报、可沉淀打法候选）＋`META_DELIVER:a5b3aaa` 哨兵——本席直读施工 run 日志尾（`run/logs/e0b4355c7faac3a4.log`）独立核实 ✓。

## 逐锚对账（票面 11 锚，全绿）

| 锚 | 机器证据（锚 1/2/11 本席独立复跑；锚 3-10 测试源码逐条核断言＋逐测单跑全 PASS） | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 本席复跑 @5cbe85b（`/tmp/t0015-audit4-full.log`，2026-11-02T06:36:11+08:00）：`go build ./... && go vet ./...`→BUILD_VET:OK exit 0；`go test -count=1 ./...`→15 包全 `ok`（gateway 18.473s）TEST_EXIT=0；`bash scripts/selfcheck.sh`→SELFCHECK_EXIT=0。包集与施工机记 `/tmp/t0015.machine-record.txt`（15 包 ok）一致 | 绿 |
| 2 pb 三件逐字节 | 本席在隔离场（detached worktree `/tmp/t0015-audit4-mut` @5cbe85b）原位再生：`protoc 3.21.12`（/usr/bin）＋`~/go/bin` gen-go v1.36.12/gen-go-grpc/gen-connect-go，`-I proto --go_opt=module=meta` 等三插件再生→`git status --porcelain` **零行**＝byte-identical，无手改 pb | 绿 |
| 3 NewTicket.project 立票径 | admin.go 前置登记校验（未登记→Ack{Ok:false} 票不立）＋`CreateTicketProv(..., nt.Project)` 同 INSERT 原子落；`TestAttach_CreateTicket`（t0015_test.go:82-109）本席 `-run` 单跑 PASS：未登记拒＋票不立＋落 P＋留痕 1 行 | 绿 |
| 4 派发径挂接＋锁前序 | dispatch.go attach 块位于 BudgetFrozen 后、lockGate（acquireProjectLocks(t.Project)）/designLocks **之前**；`TestAttach_Dispatch`（:114-175）单跑 PASS：project=="P"、事件 actor=="gateway"/prov=="test:disp"、待部署 CTO 票后 `projectLocks["P"]` 占 integrate+deploy、`""` 键桶空、同值幂等零事件 | 绿 |
| 5 席内 create_ticket | ceo.go handleAdmin：空→继承本票 pt.Project、非空→GetProject 校验拒；`TestAttach_SeatAdmin`（:224-250）三分支断言全 PASS（继承 P／显式 Q／未登记 false 票不立） | 绿 |
| 6 席内 set_project | ceo.go 越票判→相位闸（非施工中拒）→attachProject；`TestAttach_SeatAdmin`（:195-223）PASS：越票 admin_resp false＋本票落 P＋project_set 事件在 | 绿 |
| 7 SetTicketProject RPC 五判 | admin.go RPC→attachProject 五判（不存在/未登记/部署中·终态/幂等/换挂）；`TestAttach_SetTicketProjectRPC`（:256-316）单跑 PASS：七情形全断言（含幂等后成功行仍 1） | 绿 |
| 8 Requirement.project | ceo.go CeoSubmitRequirement 未登记 RPC 层拒需求不立；`TestAttach_Requirement`（:320-343）单跑 PASS：落 P＋留痕 1 行 | 绿 |
| 9 CLI 四面 | main.go ticket 子命令拆分＋`set-project` 新子命令＋new/dispatch/require 三旗＋用法行；`TestCLI_ProjectLink`（main_test.go:226-326）单跑 PASS：真网关 socket 四径落库断言——强于票面「桩断言 RPC 载荷」 | 绿 |
| 10 事件＋emit＋caster 映射 | `TestAttach_ObserveEmit`（:347-364）单跑 PASS：waitEvent 观察者断言＋ticket_event 行；`TestDispatchAction_Project`（caster_test.go）单跑 PASS：DispatchAction.project→DispatchReq.Project 映射＋project_set notify；caster.textproto project_set→notify 行在 diff | 绿 |
| 11 对抗锚变异必红 | **本席五变异全红全回退全回绿**（明细见下节），票面要求 ≥3（票面列 4 枚） | 绿 |

## 变异复算明细（每枚：变异→编译→定向测试 FAIL→`git checkout` 回退；隔离场 `/tmp/t0015-audit4-mut`，交付 worktree 全程未染）

- **a**. project.go 换挂判 `t.Project != ""`→`==` → `TestAttach_SetTicketProjectRPC` FAIL `:283 空→P 应放`
- **b**. dispatch.go attach 块（注释＋6 行）整体挪至 CTO 锁段之后 → `TestAttach_Dispatch` FAIL `:158 projectLocks[P] 应占 integrate+deploy: map[]`（锁落旧 "" 键——与票面预测逐字吻合）
- **c**. project.go 删闸内 GetProject 登记校验块 → `TestAttach_SetTicketProjectRPC` FAIL `:277 未登记应拒`
- **c2**. admin.go 删 CreateTicket 立票复制件校验块 → `TestAttach_CreateTicket` FAIL `:93 未登记项目应拒`
- **d**. ceo.go 删 `sp.TicketId != st.run.TicketID` 越票判 → `TestAttach_SeatAdmin` FAIL `:199 set_project 越票未拒`

回绿实证：五枚回退后 `go test -run 'TestAttach_' ./internal/gateway` 全 ok exit 0；隔离场 porcelain 0 后销场。

## 范围核对

码面 22 件全落票面「范围内」清单（proto/pb 三件、db/ticket.go、gateway 四件 admin·ceo·dispatch·project、seat.go、cmd/meta/main.go、caster.go、caster.textproto、cto-design.md、事件模型.md、九件测试）。清单外两件均义务件非蔓延：`债务台账.md` D-44 行＝设计 §5 钦定记债（diff +1 行 open，本席核文）；经验库新文＝令#E1 收口门。`ceo_test.go` SetProject→SetTicketProject 改名＝票面「去重留一」随行。禁线零触：`META_ADMIN:dispatch` 段格式零改（seat.go dispatch case 未动）、路由表 dispatch 行未配 project（textproto 仅 +project_set notify 行）、sudoers 不碰、未并 master；selfcheck R2/R3 零命中（exit 0）。

## 债项节（暗债对账：零瞒报）

diff 增行 grep TODO/FIXME/XXX/HACK＝**零命中**（本席实测）。摘要「下一步」五条逐条对账：

| 摘要「下一步」申报 | 对账 | 判 |
|---|---|---|
| 存量票回填执行（部署后 set-project→meta） | 票面范围外明列＋设计 §4 自举序，非本票码面（diff 无回填执行） | 账实符 |
| D-44 按票项目注入配方 | 台账 open 行 diff 实证在账 | 账实符 |
| 立票径登记校验复制件 | 结构必然（票未生无 ticket_event 可挂、不能过闸），经验库文在 a5b3aaa | 账实符 |
| denied: 拒留痕 | 设计 §5 落点，project.go deny 闭包 diff 实证 | 账实符 |
| 0013/0014 合并面 rebase 义务 | 票面附注惯例申报，本支未并 master（merge-base 非祖先） | 账实符 |

## 质量核三问（无驳回级发现）

- **最优解**：attachProject 五判与设计 §2 逐字对应，六径共用一把闸；立票三径「先校验后同 INSERT 原子落」无更简等价解（备选先立后挂＝多一次写径＋非原子）。
- **第一性**：挂接点真在取锁闸前（变异 b 实证）；`SetProject`/`SetTicketProject` 同义重复去重留一，尸体删净。
- **洁净**：生成件零手改（锚 2）；零 TODO 残渣（本席 grep）；无死码（旧 SetProject 调用点随行改）。

观察项（不构成驳回）：①票面锚 11c 字面「登记校验删→锚 3 红」指称偏差——删**闸内**校验红的是锚 7（:277），锚 3 红须删 admin.go 复制件（c2 实测）；两处校验各有独立钉锚、变异必红实质达成（判决 2 已 flag 同点，票面小瑕回 CTO 知悉即可）。②`set_project` 越票判删后写目标仍绑 `st.run.TicketID`——残余保护严于票面语义，判等仍必要且已钉锚。③前三任审计席皆「有档无哨兵」死席——流程债不在本票施工面，本席以此档＋stdout 末行哨兵收口。

## 可沉淀打法候选（意见，非否决依据）

**可沉淀**（同判决 1/2 意见，本席独立复算再证其普遍性——变异 c 若只打闸内校验而跑 CreateTicket 测即假绿）：「共享机械闸＋票未生径前置校验复制件＋两处各钉对抗锚」双层模式，`docs/经验库/2026-11-02-共享闸前置校验是复制件-变异要打在覆盖它的测试上.md` 一文准确可复用。

## 判决

**通过**。11 锚全绿（锚 1/2/11 本席独立机器复算：15 包 ok＋selfcheck 0、pb 再生 porcelain 零行、五变异全红全回退全回绿）；范围净；形制全（支名/commit 格式/已推/未并 master/摘要五节＋META_DELIVER 哨兵）；暗债零瞒报。三任死席「通过」结论获本席第四席独立佐证。
