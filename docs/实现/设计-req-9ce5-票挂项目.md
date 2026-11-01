# 设计 req-9ce5：票→项目挂接线（NewTicket.project 落地＋挂接闸＋存量回填径）

> 需求票 `req-9ce5efb2e4ad`（provenance=terminal，决策 63 立）。
> 本设计一票到底：proto→db→网关→席→CLI→caster 字段接线＋回填 RPC＋文档同步。

## 1. 需求原文与现状确诊

**需求**：`NewTicket.project` 字段落地＋立票/派发径 `SetTicketProject` 接线；
存量票回填项目列的合法径（不破网关唯一写者）。

**确诊**（逐条指到代码行）：

| 事实 | 落点 |
|---|---|
| `ticket.project` 列已在 | `internal/db/schema.go:11`（票0010 批） |
| `NewTicket.project` 列明未落 | 设计-0007 §193 字段清单有；`proto/meta/v1/meta.proto` `NewTicket` 无此字段 |
| 写径存在但无生产调用 | `db.SetTicketProject`（project.go:84）、`db.SetProject`（ticket.go:235）——**两函数同义重复**，仅 `deploy_test.go:147` 调 |
| exec 闸拒点 | `internal/gateway/exec.go:146-152`：`GetProject(t.Project)`→nil→「项目 X 无配方登记」；`t.Project=""` 时永不命中 |
| 锁键退化 | `dispatch.go:94` `acquireProjectLocks(t.Project,…)`：空 project 票全聚 `""` 键桶——全表待部署票串行于一把锁 |
| 设计锁被跳过 | `dispatch.go:113`：`t.Project != ""` 才查 `designLocks`——空 project 票设计链不串行 |
| 存量 | `ticket` 表 0003–0012 十票相位 `待部署`、`project=''`；0013 施工中、0014 已立、本票施工中，同空 |

## 2. 总体方案：挂接矩阵

票挂项目＝**只能加不能换**：`"" → P` 或 `P → P`（幂等）；`X → Y`（X 非空）
不是挂接是换挂——本设计一律拒（换挂语义＝作废重立或专门裁决，非机械闸副业）。

| 径 | 载体 | 何时写 | 谁用 |
|---|---|---|---|
| 立票·管理面 | `NewTicket.project` | `CreateTicket` 校验登记→写入 | `meta ticket new --project` |
| 立票·CEO 面 | `Requirement.project` | `CeoSubmitRequirement` 落票时写 | `meta require --project`；需求票落地即挂 |
| 立票·席内 | `SeatAdmin.create_ticket`（同 `NewTicket.project`） | 子票创建；**空则继承本票 project** | `META_ADMIN:create_ticket\|id\|title\|spec.md[\|project]`（新增第 5 段可选） |
| 本票·席内 | `SeatAdmin.set_project` | 限本票＋施工中（同 fill_spec 闸） | `META_ADMIN:set_project\|project`——需求票漏挂时设计席自补 |
| 派发径 | `DispatchReq.project` | `DispatchSeat` 内、取锁闸**之前**写 | `meta dispatch --project`；caster `DispatchAction.project`（字段留口，本批不改表） |
| 回填径 | `AdminGateway.SetTicketProject` RPC | 任意时刻显式挂 | `meta ticket set-project --ticket --project`——存量票唯一合法写径（CLI→RPC→网关，不破唯一写者） |

**共用机械闸 `attachProject(ctx, tid, project, actor, prov)`**（五判，全机械）：
1. 票不存在 → 拒；
2. `project` 未登记（`GetProject` nil）→ 拒「项目 X 无配方登记」；
3. 相位 ∈ {部署中, 完成, 作废} → 拒——部署中票的项目锁已按旧键取，
   换键＝锁孤儿；终态票无挂义；
4. `t.Project == project` → 幂等 ok，不写无事件；
5. `t.Project != ""` 且 ≠ → 拒「票已挂 Y——换挂非本径」。
过闸：`db.SetTicketProject`＋`project_set` 事件（actor/prov 随行）。

**空项目挂接无锁孤儿**（论证）：设计锁只在派发时 `t.Project!=""` 才取
（空票从不持设计锁）；部署锁只在 `待部署→CTO` 派发取。故 `""→P` 于任意
非终态非部署中相位安全——持锁票必非空，非空即被第 5 判挡。

## 3. 改件清单

- **proto**：`NewTicket.project=6`；`DispatchReq.project=8`；
  `Requirement.project=6`；`DispatchAction.project=6`；
  `SeatAdmin` oneof 增 `set_project`（新 `SetProject{ticket_id,project}`）；
  `AdminGateway` 增 `SetTicketProject(TicketProjectReq) returns (Ack)`
  ＋`TicketProjectReq{ticket_id,project}` 消息。pb 三件重生成。
- **db**：`SetProject`/`SetTicketProject` 去重留一（留 `SetTicketProject`）；
  `CreateTicketProv` 增 project 形参（INSERT 原子落入——签名改一处，
  各调用方随行）。
- **gateway**：`attachProject` 共享闸；`CreateTicket`/`CeoSubmitRequirement`
  /`DispatchSeat`/`handleAdmin`（create_ticket 继承＋set_project 新动作）
  /新 RPC 六处接线。
- **seat**：`adminSentinel` `create_ticket` 第 5 段可选 project；
  `set_project` 新支。`META_ADMIN:dispatch` 格式不动（子票挂接已在
  create_ticket 落，派发径挂接归管理面）。
- **cmd/meta**：`ticket new --project`；新子命令 `ticket set-project`；
  `dispatch --project`；`require --project`；用法行同步。
- **caster**：`DispatchAction.project` → `DispatchReq.Project` 字段映射
  （一行；不改路由表）。
- **配置**：`config/caster.textproto` 增 `project_set`→notify 行
  （管理动作透明——格式同既存通知行）。
- **文档**：`config/caster/cto-design.md`、`renderRedirectPrompt`（ceo.go
  内嵌轨）补 `create_ticket` 第 5 段与 `set_project` 哨兵说明；
  `docs/事件模型.md` 补 `project_set` 行。**总设计不动**（AdminGateway
  RPC 集本是实现层扩展面，RetroVerdict/IssueCeoToken 同例；八栏契约
  不变——project 是票行字段，不是第九栏）。

## 4. 自举序（鸡生蛋的合法解）

本功能的部署需要闸通，而闸通需要本功能——**老二进制无任何写 project
的径**。钦定序：

1. 子票 0015 施工→审→`待部署`（全程空 project 不碍——闸只在 exec 步拒）。
2. **换装仍走手工径一次**（同决策 63：并入 master→build/vet/test/selfcheck
   绿→`scripts/meta-deploy.sh`→重启）——新二进制起服。
3. `meta ticket set-project` 回填（幂等，顺序无关）：
   `0003,0004,0005,0006,0007,0008,0009,0010,0011,0012`（存量十票）、
   `0013`、`0014`、`req-9ce5efb2e4ad`（本票）、`0015`（自挂票据）→ 全 `meta`。
4. 此后 `audit_pass→CTO-B→exec` 闭环复通；存量十票经 `spawn_needed:CTO`
   （重启对账/锁释补发/路由派发）逐张过闸→`完成`＝部署闸端到端首验。
5. 前置不变：check 步 `sudo -n -u metaseat` 待业主
   `/etc/sudoers.d/meta-runner` 行——sudo 失＝退码 fail loud，不静默降级。

## 5. 边界与风险

- **派发径挂接的并发**：两张派单同时对同票挂不同 project——先写者赢，
  后写者撞第 5 判被拒并出 `project_set` 失败事件，票停发原位。可接受
  （挂接是一次性动作，非热径）。
- **非目标**：积票部署（E17 批闸）本体；按票项目注入配方（注入机 Recipes
  现取配置静态列——per-project 化是另一设计，记债）；O 项目登记。
- **`DispatchAction.project` 配 route**：字段留口但本批不改表——隐式挂接
  行为显式化之前不进路由表。
- **事件模型**：`project_set` 非相位边、非 CEO 事件——只进 ticket_event
  持久日志＋emit（通知行如上配）。

## 6. 拆票（一子票）

- 票 0015「票→项目挂接线全链」：本设计 §3 全部改件＋锚（见子票票面）。
- 依赖：无硬依赖；与在途 0013/0014 有 proto/dispatch.go/ceo.go/main.go
  合并面——并入序晚者 rebase＋pb 重生成（惯例）。
