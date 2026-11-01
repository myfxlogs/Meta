# 票 0015 判决：通过（审计席·claude 独立实例 #3，2026-11-02）

- 审计 run：`640edddf0e7d8148`（shell=claude），触发＝audit_dead（事件 683——第二审计席 e0cf7e5923a12296 exit=2 净退无哨兵）
- 前情：判决 1（`1-devin.md`，run `01436fbea2bb76ef`）与判决 2（`2-claude.md`，run `e0cf7e5923a12296`）均结论「通过」，但两席皆死无哨兵＝半交付不采信。本档＝第三席**从零独立复算**（未引前两档结论，仅对同一 diff 与机器证据）。
- 施工 run：`e0b4355c7faac3a4`（devin/swe-2-high），交付＝支 `t/0015-project-link` @ `a5b3aaa`（码面双 commit b7335de＋a5b3aaa，格式 `t0015:` ✓），22 文件 +1378/−314
- 复算基：交付 worktree @`485051d`（＝a5b3aaa＋两死席判决档；`git diff --stat a5b3aaa..485051d` 仅 2 个 md 档＝docs-only，码面等同 a5b3aaa）
- 形制：`git ls-remote origin` 远端 HEAD＝485051d 已推 ✓；master 未并 ✓；交付摘要五节＋`META_DELIVER:a5b3aaa` 哨兵在施工 run 日志尾实证 ✓

## 逐锚对账（票面 11 锚，全绿）

| 锚 | 机器证据（锚 1/2/11 为本席独立机器复算；锚 3-10 为测试源码逐条核断言＋全量绿） | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 本席复跑（`/tmp/t0015-audit3-full.log`）：`go build ./... && go vet ./...`→BUILD_VET OK；`go test -count=1 ./...`→15 包全 `ok` TEST_EXIT=0；`bash scripts/selfcheck.sh`→SELFCHECK_EXIT=0。包集与施工机记 `/tmp/t0015.machine-record.txt`（15 包 ok）一致 | 绿 |
| 2 pb 三件逐字节 | 本席原位再生：protoc 3.21.12（/usr/bin）＋gen-go v1.36.12＋gen-go-grpc 1.6.1＋connect 2.0.1（版本与三件文件头逐一相符）→`git status --porcelain -- internal/pb proto` **零行**＝byte-identical，无手改 pb | 绿 |
| 3 NewTicket.project 立票径 | admin.go:20-31 前置登记校验（未登记→Ack{Ok:false} 票不立）＋CreateTicketProv(..., nt.Project) 同 INSERT 原子落；`TestAttach_CreateTicket`（t0015_test.go:82-109）断言未登记拒＋票不立＋project=="P"＋留痕 1 行 | 绿 |
| 4 派发径挂接＋锁前序 | dispatch.go:66-75 attach 块位于 :63 BudgetFrozen 后、:103-104 lockGate（acquireProjectLocks(t.Project)）/:123 designLocks **之前**；`TestAttach_Dispatch`（:114-175）断言 project=="P"、事件 actor=="gateway"/prov=="test:disp"、待部署 CTO 票后 projectLocks["P"] 占 integrate+deploy、"" 键桶空、同值幂等零事件 | 绿 |
| 5 席内 create_ticket | ceo.go:658-677 空→继承本票 pt.Project、非空→GetProject 校验拒、原子落＋留痕；`TestAttach_SeatAdmin`（:224-250）三分支断言（继承 P／显式 Q／未登记 false 票不立） | 绿 |
| 6 席内 set_project | ceo.go:684-686 越票判→:687-690 相位闸（非施工中拒）→attachProject；`TestAttach_SeatAdmin`（:195-223）越票 admin_resp false＋本票落 P＋project_set 事件在 | 绿 |
| 7 SetTicketProject RPC 五判 | admin.go:198-208→attachProject（project.go 五判：不存在/未登记/部署中·终态/幂等/换挂）；`TestAttach_SetTicketProjectRPC`（:256-316）七情形全断言（含幂等后成功行仍 1） | 绿 |
| 8 Requirement.project | ceo.go:365-375 未登记 RPC 层拒需求不立；`TestAttach_Requirement`（:320-343）落 P＋留痕 1 行 | 绿 |
| 9 CLI 四面 | main.go:424-521 ticket 子命令拆分＋set-project 新子命令＋new/dispatch/require 三旗＋用法行；`TestCLI_ProjectLink`（main_test.go:226-326）真网关 socket 四径落库断言——强于票面「桩断言 RPC 载荷」要求 | 绿 |
| 10 事件＋emit＋caster 映射 | `TestAttach_ObserveEmit`（:347-364）waitEvent 观察者断言＋ticket_event 行；`TestDispatchAction_Project`（caster_test.go:512-547）DispatchAction.project→DispatchReq.Project 映射＋project_set notify；caster.textproto:52 notify 行在 diff | 绿 |
| 11 对抗锚变异必红 | 本席五变异**全红全回退**（明细见下节），票面要求 ≥3 | 绿 |

## 变异复算明细（每枚：变异→对应测试 FAIL→`git checkout`→porcelain 净）

- **a**. project.go 换挂判 `t.Project != ""`→`==` → `TestAttach_SetTicketProjectRPC` FAIL `:283 空→P 应放`
- **b**. dispatch.go attach 块（6 行）整体挪至 CTO 锁段之后 → `TestAttach_Dispatch` FAIL `:158 projectLocks[P] 应占 integrate+deploy: map[]`（锁落旧 "" 键）
- **c**. project.go 删 GetProject 登记校验整块（7 行）→ `TestAttach_SetTicketProjectRPC` FAIL `:277 未登记应拒`
- **c2**. admin.go 删 CreateTicket 前置校验整块（10 行）→ `TestAttach_CreateTicket` FAIL `:93 未登记项目应拒`
- **d**. ceo.go 删 set_project 越票判整块（3 行）→ `TestAttach_SeatAdmin` FAIL `:199 set_project 越票未拒`

（方法注记：c/c2 首试以 sed 单行改条件致 unused-var 编译错＝非有效红，本席不计入；整块摘除重跑方为有效证据。）

## 范围核对

22 件全落票面「范围内」清单（proto/pb 三件/db ticket.go/gateway 四件（admin·ceo·dispatch·project）/seat.go/cmd/meta/caster.go/caster.textproto/cto-design.md/事件模型.md/九件测试）。清单外三件均义务件非蔓延：`债务台账.md` D-44 行＝设计 §5 钦定记债；经验库新文＝令#E1 收口门；`ceo_test.go` SetProject→SetTicketProject 改名＝票面「去重留一」随行（build 绿＝`db.SetProject` 零残调用的编译级证明）。禁线零触：seat.go dispatch case 零改（`META_ADMIN:dispatch` 哨兵格式未动）、路由表仅 +project_set notify 行（dispatch 行未配 project）、sudoers 未碰、未并 master、diff 无回填执行件、无 E17 批闸/O 项目登记触碰。

## 债项节（暗债对账：零瞒报）

码面增行 grep TODO/FIXME/XXX/HACK/占位/绕过/临时/调试/debug/Ticker/encoding-json **零命中**（本席实测）。摘要「下一步」五条逐条对账：①存量回填执行＝票面范围外明列＋设计 §4 自举序（diff 无执行件）✓；②D-44＝台账 diff +1 open 行 ✓；③立票径校验复制件＝经验库文在 diff ✓；④`denied:` 拒留痕＝project.go deny 闭包在 diff ✓；⑤0013/0014 rebase 义务＝票面附注惯例申报 ✓。账实符。

## 质量核三问（无驳回级发现）

- **最优解**：五判与设计 §2 逐字对应；project 与票行同 INSERT 原子落（非先立后 UPDATE）；立票径前置校验复制件有「票未生无 ticket_event 可挂」结构论证——未找到更简等价解。
- **第一性**：挂接点真在取锁闸前（变异 b 实证）；`SetProject`/`SetTicketProject` 同义重复去重留一。
- **洁净**：生成件零手改（锚 2）；尸体删净（编译级证明）；增行零残渣（grep 实证）。

观察项（不构成驳回）：①票面锚 11c 字面「登记校验删→**锚 3** 红」与实测指称偏差——attachProject 校验删红的是锚 7（`:277`），锚 3 红须删 admin.go 复制件（c2 实测红）——两处校验各钉各锚、变异必红实质达成，票面小瑕回 CTO 知悉；②变异 d 后写目标仍绑 `st.run.TicketID`（sp.TicketId 仅作判等输入）＝残余保护严于票面语义，判等仍必要且测试捕获删除。

## 可沉淀打法候选（意见，非否决依据）

**可沉淀**：「共享机械闸＋票未生径前置校验复制件＋两处各钉对抗锚」——本席 c/c2 双变异实测同一坑（跑错覆盖测试险假绿），叠加施工自曝＋判决 2 独立踩中＝三席互证坑普遍性；经验库一文准确可复用。

## 判决

**通过**。11 锚全绿（锚 1/2/11 本席独立机器复算：15 包 ok＋selfcheck 0、pb 再生 porcelain 零行、五变异全红全回退；锚 3-10 测试断言与票面逐条核对＋全量绿）；范围净；形制全；暗债零瞒报。与死席判决 1/2 结论一致＝三席互证。
