# 票 0015 判决：通过（审计席·devin 独立实例，2026-11-02）

- 审计 run：`01436fbea2bb76ef`（shell=devin/swe-2-max），触发＝deliver（事件 419，`a5b3aaa devin exit=0`）
- 施工 run：`e0b4355c7faac3a4`（shell=devin/swe-2-high），worktree `~/meta-worktrees/t0015-project-link`
- 交付物：支 `t/0015-project-link` @ `a5b3aaa`（ls-remote origin 核实已推、与 deliver ref 同；master=`42a646e` 未并），双 commit `t0015:`（b7335de 码面＋a5b3aaa 台账/经验库），22 文件 +1378/−314
- 交付摘要：run 日志五节齐（新增/修改面、根因、验证引、下一步全量遗留申报、可沉淀打法候选）＋`META_DELIVER:a5b3aaa` 哨兵在

## 逐锚对账（票面 11 锚，全绿）

| 锚 | 机器证据 | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 施工机记 `/tmp/t0015.machine-record.txt`＝15 包全 `ok`；本席交付 worktree @a5b3aaa 原命令独立复算（`/tmp/t0015-audit-recompute.txt`）：`BUILD+VET: OK`、`go test -count=1 ./...` 15 包全 `ok` TEST_EXIT=0、`bash scripts/selfcheck.sh` SELFCHECK_EXIT=0——与施工记录包集相同 | 绿 |
| 2 pb 三件逐字节 | 本席跑 `protoc 3.21.12 + protoc-gen-go v1.36.12 + grpc v1.6.1 + connect v2`（版本与 pb 文件头一致）原位再生三件 → `git status --porcelain` 对 internal/pb＋proto 零行——无手改 pb | 绿 |
| 3 NewTicket.project 立票径 | `TestAttach_CreateTicket`（t0015_test.go:82-109）：`{project:nope}`→`ack.Ok=false`＋`GetTicket` nil（票不立）；`{project:P}`→`tk.Project=="P"`＋project_set 留痕 1 行 | 绿 |
| 4 派发径挂接＋锁前序 | `TestAttach_Dispatch`（t0015_test.go:114-175）：派发后 `tk.Project=="P"`、`project_set` 事件 `Actor=="gateway"`/`Provenance=="test:disp"`；待部署 CTO 票挂接后 `projectLocks["P"]` 占 integrate+deploy、`""` 键桶空。本席变异 b（attach 挪至锁取后）实测 FAIL `:158 projectLocks[P] map[]`——顺序真实 | 绿 |
| 5 席内 create_ticket | `TestAttach_SeatAdmin`（:224-250）：project 空→子票继承本票 `P`；显式 `Q`→落 `Q`；未登记→`admin_resp false`＋票不立 | 绿 |
| 6 席内 set_project | 同测试 :195-223：`ticket_id≠本票`→`admin_resp false`（拒在 attach 前，事件无写结构性保证）；本票→落＋project_set 事件在 | 绿 |
| 7 SetTicketProject RPC 五判 | `TestAttach_SetTicketProjectRPC`（:256-316）：未登记拒（project 保持空）→空→P 放→同值幂等 ok 且成功行仍 1→X→Y 换挂拒（仍 P）→部署中/终态/不存在票全拒 | 绿 |
| 8 Requirement.project | `TestAttach_Requirement`（:320-343）：未登记→RPC 层 err 需求不立；`P`→需求票 `tk.Project=="P"`＋留痕 1 行 | 绿 |
| 9 CLI 四面 | `TestCLI_ProjectLink`（main_test.go:+101）：真网关 socket——`ticket new` 未登记 CLI 报错＋票不立；`new/set-project/dispatch/require --project` 四旗全落库断言（强于票面桩断言要求） | 绿 |
| 10 事件＋emit＋caster 映射 | `TestAttach_ObserveEmit`（:347-364）：`waitEvent("project_set")` 观察者断言＋ticket_event 行在；`TestDispatchAction_Project`（caster_test.go:+39）：`DispatchAction.project`→`DispatchReq.Project` 映射＋`project_set`→notify 路由行通 | 绿 |
| 11 对抗锚 ≥3 变异必红 | 票面列 4 枚，**本席逐一真跑变异复算全红**：a. project.go 换挂判 `!=`→`==`→`TestAttach_SetTicketProjectRPC` FAIL `:283 空→P 应放`；b. dispatch.go attach 挪至 `g.specs` 登记前（锁取后）→`TestAttach_Dispatch` FAIL `:158`；c. project.go 删 GetProject 登记校验→同测 FAIL `:277 未登记应拒`；d. ceo.go 删 `sp.TicketId != st.run.TicketID` 越票判→`TestAttach_SeatAdmin` FAIL `:199`。四变异均 `git checkout` 回退，`git status` 净 | 绿 |

## 范围核对

22 件全落票面「范围内」清单（proto/db/gateway×4/seat/cmd/caster/caster.textproto/cto-design.md/事件模型/九件测试）。两件清单外均为义务件非蔓延：`债务台账.md` D-44 行＝设计 §5 钦定记债；经验库新文＝收口门令#E1。`ceo_test.go` `SetProject→SetTicketProject` 改名＝票面明列「去重留一」随行改。禁线零触：`META_ADMIN:dispatch` 段格式未动（SplitN 4→5 仅影响第 5 段解释，dispatch/fill_spec 只用前 4 段）、路由表 dispatch 行未配、无 sudoers、未并 master。

## 债项节（暗债对账：diff 零 TODO/FIXME/绕过/半成品）

| 摘要「下一步」申报 | 对账 | 判 |
|---|---|---|
| 存量票回填执行（部署后 set-project 十三票→meta） | 票面范围外明列＋设计 §4 自举序钦定，非本票码面 | 账实符 |
| D-44 按票项目注入配方 | 台账行 diff 实证在账（债务台账.md:+1 行 open） | 账实符 |
| 立票径登记校验为复制件 | 结构必然（票未生无 ticket_event 可挂、不能过闸），经验库文已记坑 | 账实符 |
| `denied:` 拒留痕 | 设计 §5「撞闸留痕」落点，project.go deny 闭包实证 | 账实符 |
| 0013/0014 合并面 rebase 义务 | 票面附注惯例申报，本支未并 master | 账实符 |

零瞒报。

## 质量核三问（举证门槛达则列，不达不驳回）

- **最优解**：attachProject 五判与票面逐字对应；立票三径前置校验复制件有「票未生不能过闸」硬论证且经验库固化——未找到更简等价解（备选「先立后挂两次写」反而多一次写径）。
- **第一性**：立票径 project 与票行同 INSERT 原子落入（非先立后 UPDATE）；挂接点真在取锁闸前（变异实证）；`SetProject`/`SetTicketProject` 同义重复去重留一。
- **洁净**：增行 grep TODO/FIXME/临时/绕过/debug 零命中；`db.SetProject` 尸体删净（全仓 grep 零残调用）；生成件零手改（锚 2）。
- 观察项（不构成驳回）：①立票三径前置校验拒时无 `denied:` 事件行——票未生无 ticket_event 行可挂，结构必然非缺陷；②`set_project` 越票判删后写目标仍绑 `st.run.TicketID`（sp.TicketId 仅作判等输入），残余保护严于票面语义，判等仍必要且已钉锚；③DispatchSeat 中 attach 先于 adjunct/角色占用校验，后续拒则 project 已挂——幂等＋denied 留痕，属设计 §5 可接受区。

## 可沉淀打法候选（意见，非否决依据）

**可沉淀**：「共享机械闸＋新生径复制件＋两处各钉对抗锚」双层模式——已由施工落经验库一文（`docs/经验库/2026-11-02-共享闸前置校验是复制件-变异要打在覆盖它的测试上.md`），「变异前先问哪条断言覆盖这一行」是可复用的变异验证纪律。

## 判决

**通过**。11 锚全绿（其中锚 2 pb 逐字节与锚 11 四变异均由本席独立复算证实，非引施工自述）；范围净；形制全；暗债零瞒报。
