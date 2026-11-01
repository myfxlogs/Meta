# 票 0015 判决：通过（审计席·claude 独立实例，2026-11-02）

- 审计 run：`e0cf7e5923a12296`（shell=claude），触发＝deliver（事件 419，`a5b3aaa devin exit=0`）
- 前情：判决 1（`1-devin.md`，run `01436fbea2bb76ef` shell=devin/swe-2-max）出档后席位死亡（事件 643，SIGCHLD+EOF——有档无哨兵＝半交付）。本档＝重派席**从零独立复算**，非续写；结论与判决 1 一致属两席互证。
- 施工 run：`e0b4355c7faac3a4`（shell=devin/swe-2-high），worktree `~/meta-worktrees/t0015-project-link`
- 交付物：支 `t/0015-project-link` @ `a5b3aaa`（码面）＋ `b811001`（判决 1 档，死席产物）；`git ls-remote origin` 实证远端 HEAD＝b811001（已推）；master `80b6d7e` 未并 ✓；commit 格式 `t0015: <为什么>` ✓×2
- 交付摘要五节原文在 run 日志尾（`run/logs/e0b4355c7faac3a4.log`）：新增/修改面、根因、验证引、下一步＝全量遗留申报五条、可沉淀打法候选＋`META_DELIVER:a5b3aaa` 哨兵 ✓

## 逐锚对账（票面 11 锚，全绿；1/2/11 为本席独立机器复算）

| 锚 | 证据 | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 本席在交付 worktree 原命令复跑：`go build ./... && go vet ./...`→OK；`go test -count=1 ./...`→15 包全 `ok`（gateway 18.070s，tee `/tmp/t0015-audit2-recompute.txt`）TEST_EXIT=0；`bash scripts/selfcheck.sh`→SELFCHECK_EXIT=0。与施工机记 `/tmp/t0015.machine-record.txt`（15 包 ok）及死席复算 `/tmp/t0015-audit-recompute.txt` 三方一致 | 绿 |
| 2 pb 三件逐字节 | 本席原位再生：`protoc 3.21.12`（/usr/bin）＋`~/go/bin` gen-go v1.36.12/gen-go-grpc v1.6.1/gen-connect-go，`-I proto --go_opt=module=meta` 再生三件→`git status --porcelain` **零行**＝byte-identical，无手改 pb | 绿 |
| 3 NewTicket.project 立票径 | admin.go:22-31 前置登记校验（未登记→Ack false 票不立）＋:33 `CreateTicketProv(..., nt.Project)` 原子落；`TestAttach_CreateTicket`（t0015_test.go:82-109）断言未登记拒+票不立+落 P+留痕 1 行 | 绿 |
| 4 派发径挂接＋锁前序 | dispatch.go:69-74 attach 块位于 :63 BudgetFrozen 后、:103 lockGate/:123 designLocks **之前**；`TestAttach_Dispatch`（:114-175）：actor=="gateway"/prov=="test:disp" 事件断言、`projectLocks["P"]` 占 integrate+deploy、`""` 键桶空、同值幂等零事件 | 绿 |
| 5 席内 create_ticket | ceo.go:658-677 空→继承 `pt.Project`、非空→GetProject 校验拒；`TestAttach_SeatAdmin`（:224-250）三分支断言（继承 P／显式 Q／未登记 false 票不立） | 绿 |
| 6 席内 set_project | ceo.go:684-686 越票判＋:687-690 相位闸（非施工中拒）→attachProject；`TestAttach_SeatAdmin`（:195-223）越票 false、本票落+事件在 | 绿 |
| 7 SetTicketProject RPC 五判 | admin.go:198-208→attachProject（project.go:34-62 五判：不存在/未登记/部署中·终态/幂等/换挂）；`TestAttach_SetTicketProjectRPC`（:256-316）七情形全断言（含幂等后成功行仍 1） | 绿 |
| 8 Requirement.project | ceo.go:365-375 未登记 RPC err 需求不立；`TestAttach_Requirement`（:320-343）落 P+留痕 1 行 | 绿 |
| 9 CLI 四面 | main.go:424-521 拆子命令+`set-project` 新子命令+四旗；`TestCLI_ProjectLink`（main_test.go:226-326）真网关 socket 四旗落库断言（强于票面「桩断言 RPC 载荷」） | 绿 |
| 10 事件＋emit＋caster 映射 | `TestAttach_ObserveEmit`（:347-364）waitEvent 观察者+ticket_event 行；`TestDispatchAction_Project`（caster_test.go:512-547）DispatchAction.project 映射+project_set notify；caster.textproto:52 notify 行 | 绿 |
| 11 对抗锚变异必红 | **本席五变异全红全回退**（每枚变异→跑测→FAIL→`git checkout`→porcelain 净）：a. project.go:58 `!=`→`==`→`TestAttach_SetTicketProjectRPC` FAIL `:283 空→P 应放`；b. dispatch.go attach 块挪至 lockGate/designLocks 后→`TestAttach_Dispatch` FAIL `:158 projectLocks[P] map[]`；c. project.go:42-49 删登记校验→RPC 测 FAIL `:277 未登记应拒`；c2. admin.go:22-31 删立票复制件校验→`TestAttach_CreateTicket` FAIL `:93 未登记项目应拒`；d. ceo.go:684-686 删越票判→`TestAttach_SeatAdmin` FAIL `:199 越票未拒` | 绿 |

## 范围核对

码面 22 件全落票面「范围内」清单：proto/pb 三件、db/ticket.go、gateway 四件（admin/ceo/dispatch/project）、seat.go、cmd/meta/main.go、caster.go、caster.textproto、cto-design.md、事件模型.md、九件测试。清单外三件均义务件非蔓延：`债务台账.md` D-44 行＝设计 §5 钦定记债；经验库新文＝令#E1 收口门；`票-0015-判决/1-devin.md`＝死席审计产物。`ceo_test.go` SetProject→SetTicketProject 改名＝票面「去重留一」随行。禁线零触：`META_ADMIN:dispatch` 格式未动（seat.go dispatch case 零改）、路由表 dispatch 行未配 project（textproto 仅 +notify 行）、sudoers 未碰、未并 master。

## 债项节（暗债对账：零瞒报）

diff 增行 grep TODO/FIXME/临时/绕过/半成品/占位**零命中**（本席实测）。摘要「下一步」五条逐条对账：存量回填执行（票面范围外明列＋设计 §4）／D-44 台账 open 行在（diff +1）／复制件结构（经验库文实证）／denied: 留痕（project.go deny 闭包实证）／0013·0014 rebase 义务（票面附注惯例）——账实符，零瞒报。

## 质量核三问（无驳回级发现）

- **最优解**：五判与设计 §2 逐字对应；立票径与票行同 INSERT 原子落（非先立后 UPDATE）；未找到更简等价解。
- **第一性**：`db.SetProject`/`SetTicketProject` 同义重复去重留一（尸体删净）；挂接点真在锁前（变异 b 实证）。
- **洁净**：生成件零手改（锚 2）；无死码残渣（全仓 `SetProject` 调用点随行改）。

观察项（不构成驳回）：①票面锚 11c 字面「登记校验删→**锚 3** 红」与实测有指称偏差——删 attachProject 校验红的是锚 7 测试（:277），锚 3 红须删 admin.go 复制件（c2 实测红）；两处校验各有独立钉锚、变异必红实质达成，票面小瑕回 CTO 知悉即可。②施工日志自曝变异 c 首跑误用 CreateTicket 测试得假绿、重验纠正——透明且已沉淀经验库。

## 可沉淀打法候选（意见，非否决依据）

**可沉淀**（同判决 1 意见）：「共享机械闸＋票未生径前置校验复制件＋两处各钉对抗锚」——本席独立复算时踩中同一假绿（先跑 CreateTicket 绿、换 RPC 测即红），实证该坑普遍性；`docs/经验库/2026-11-02-共享闸前置校验是复制件-变异要打在覆盖它的测试上.md` 一文准确可复用。

## 判决

**通过**。11 锚全绿（锚 1/2/11 本席独立机器复算：15 包 ok＋selfcheck 0、pb 再生 porcelain 零行、五变异全红）；范围净；形制全（支名/commit 格式/已推/未并 master/摘要五节+哨兵）；暗债零瞒报。判决 1（死席半交付）结论获本席独立佐证。
