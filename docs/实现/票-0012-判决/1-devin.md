# 票 0012 判决：通过（审计席·claude 独立实例，2026-11-01）

- 审计 run：`4569826df45538c5`（shell=claude），触发＝deliver（事件 115，`devin exit=0`）
- 施工 run：`8b43f7d02ddd8a25`（shell=devin），worktree `~/meta-worktrees/t0012`
- 交付物：支 `t/0012-seat-board` @ `e9641bd`（ls-remote origin 核实已推；master=`1c74160` 未并），单 commit `t0012: 前端席位板——业主眼前的一面立起（§9）`，18 文件 +2935/−217

## 逐锚对账（票面六锚，全绿）

| 锚 | 机器证据 | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 施工记录 `/tmp/t0012.machine-record.txt`（go test 原文 tee）＝15 行全 `ok`；本席在交付 worktree 原命令复算（记录 `/tmp/t0012-audit-recompute.txt`）：`go build ./... && go vet ./...` 链式静默过、`go test -count=1 ./...` 15 行全 `ok`（与施工记录 diff 仅耗时列差异、包集相同）、`bash scripts/selfcheck.sh` 退码 0 | 绿 |
| 2 全态帧 | `TestBoard_full帧_组装全`（board_test.go:258–329）：伪席 run 行＋非终态票入 full；断言 run_id/role/ticket_id、shell/model 拆分、stage=`写码`/note=`联调中`、预算四数 used=123/cap=400/usedTotal=123/hardCap=4000、attached=true；死席与终态票滤除（seats=1/tickets=1）。机器：`ok meta/internal/board`（双记录） | 绿 |
| 3 变化帧三型 | `TestBoard_变化帧三型`（board_test.go:333–398）：真 `syscall.Kill(SIGKILL)` 网关 spawn 席→`seat_gone` 帧；`Progress{stage:自测}`→seat upsert（stage 变）；`DispatchSeat` 相位迁移→ticket upsert（T2 已派） | 绿 |
| 4 token 闸 | `TestBoard_token闸_failclosed`（board_test.go:195）：secret 行缺→Connect 与 unix gRPC 两面皆 Unauthenticated；`TestBoard_token闸_错token`（board_test.go:214）：错/空/后缀 token 拒、对 token 过且首帧 full（种子带尾换行亦兼容） | 绿 |
| 5 端到端 CLI | `TestBoardCLI_端到端`（main_test.go:43–125）：`boardCmd` 走 `--socket`（unix gRPC）与 `--connect`（TCP Connect 二进制）两径；读到 `FULL` 打印行后经真 `Attach+Progress` 触发第 2 帧，`--n 2` 零错退 | 绿 |
| 6 对抗锚 ≥2（本席突变复算） | scratch 副本 `/tmp/t0012-mut` 施突（交付支未动）：a. `full()` 组装体删→返空板，`TestBoard_full帧_组装全` FAIL `seats=0 预期 1`（board_test.go:304）——锚红证实；b. `CheckBoardToken` 等值比较 `== 1` 翻 `!= 1`（gateway/board.go:40），`TestBoard_token闸_错token` FAIL `应拒却放行`（board_test.go:223）——锚红证实 | 绿 |

## 交付形制与范围

- 支名 `t/0012-seat-board` ✓；commit `t0012: <为什么>` ✓；已推 ✓；**不并 master** ✓。
- 交付摘要五节＋收口两步门 ✓（`run/logs/8b43f7d02ddd8a25.log` 尾部）；两步门声明经本席独立复核（经验库件在、worktree `git status` 净、支已推、机器记录在）。
- 范围：18 文件逐一落票面「范围内」映射（proto＋三生成件／internal/board／db 三查询件＝全态组装输入／gateway 两件＝接线与事件源／cmd/meta 两件／config 种子／web 页／go.mod,go.sum）。`docs/经验库/2026-11-01-connect-es-无构建页骨架.md`＝票面附「浏览器连通性禁区申报点」的沉淀件（令#E1 一坑一文档形），非范围蔓延。
- 禁区自查（机器级）：selfcheck R2（Ticker/Sleep）与 R3（encoding/json）零命中＝退码 0；Connect 二进制＝禁区#4 豁免钦定径；板只读 DB、无第二写者（禁区#6）；网关内 if 皆机械判（token 对验/loopback 判，禁区#7 口令合规）；依赖 `connectrpc.com/connect/v2 v2.0.2` 属 connect 白名单（禁区「待业主裁」清单第 4 类内），模块缓存 `@v/v2.0.2.info` 证实发布 `2026-10-21T22:05:54Z`＝11 天 ≥7 天钉版。
- 票状态无副本（R5）：变化帧内容恒为事件触发后 DB 现态重读投影（board.go:134–155），浏览器端为显示态非真源。

## 设计对账（补强）

设计-0007 现已在 `c/0007-design` 支（9321304）可达——0008 判决返修 push 生效。本席取该档读票面引用节 §1.10/§3/§7 逐点对：`SeatBoard(BoardReq)→stream BoardFrame`、`--board-listen`/`board_listen` 配置、token 对 `secret.board.token` 行缺 fail closed（「ACL 空＝网关自校」正是 `SecretBlob` 网关内径的设计出处）、三型变化帧幂等重放、`meta board` CLI 验收径、`web/board.html` connect-es CDN 无构建——全对上。施工自述「该档在全支均无」与其施工时点观察相符与否不可考，但漂移已双申报（progress 事件＋摘要§4）且成品与设计一致，无伤。

## 质量三问（令#E5）

- **最优解**：单 `board.Service` 双面出（`GRPC()` 适配器＋Connect handler 同一 `serve` 核心），零双实现；复用网关既有 `subscribe` 事件总线与 `LatestStages`/`GetBudget` 查询件；依赖方向净——`internal/gateway` 零 import board（grep 0 命中），接线只在 `cmd/meta`。
- **第一性**：先订后装（board.go:83–91）消组装间隙事件丢失，变化帧幂等由「恒重读 DB 现态」构造保证——R5「板是投影非状态」的直接落形，非 workaround 摞补丁。
- **洁净**：diff 增行扫 TODO/FIXME/临时/绕过零命中（生成件内 `not implemented` 属 protoc 样板）；pb 生成件删除行全为消息索引漂移（再生噪声），无手改痕。

## 债项节（暗债对账结果——diff 扫描 vs 摘要「下一步」）

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| D-0012a | proto 合并面：与 0011（CEO 面）同文件 meta.proto（`GatewayConfig` 字段位等），并入序晚者 rebase＋pb 重生成 | 已申报（票面亦预告） |
| D-0012b | 部署备忘：`secret.board.token` 未落种＝业主动作（`meta secret put`，值读 stdin）；`board_listen` 已种 `127.0.0.1:9712`。行缺即 fail closed，无token不通，安全 | 已申报 |
| D-0012c | 已知限制：订户口 256 缓冲满丢帧（与 Observe 面同语义）；`Listener.Close` 为 http.Server.Close 语义不断存量连接（进程退出即全断） | 已申报 |
| 观察 | go.mod 将 connect v2 标 `// indirect` 实为直接依赖（`go mod tidy` 会归位）；`stageOf` 单票径复用 LatestStages 全量图（以复用换一次小过度取）；CORS `*` 放行（关防本体在 token 字段不在 origin，board.go 注释已载明推理）——皆非债级 | — |

**瞒报零起**：diff 内未申报 TODO/绕过/半成品零命中；摘要「下一步」四条与本席 diff 扫描逐条对上。

## 票面备注

设计引用栏指向的 `docs/实现/设计-0007-阶段3.md` 在 master 不可达（存于 `c/0007-design` 支）——票面八栏自含、锚全机器可判，本席已判，不构成「票面待裁」；引用可达性债已在 0008 判决立账（D-0008a），此处仅记指针状态，提请 CTO 并线时顺流。

## 沉淀意见（令#E8，纯意见不构成否决依据）

「Connect 双面服务」打法（一份 service 实现＋`GRPC()` 适配器走 unix socket＋`connecthttp.Mount` 走 TCP，CLI 验收与浏览器径零双写）配 connect-es 无构建页骨架（fileDesc 内嵌描述符＋esm.sh `?deps=` 钉共享 protobuf 版本）——**可沉淀**：施工已自沉 `docs/经验库/2026-11-01-connect-es-无构建页骨架.md`，它把「浏览器连通性先险后试」从禁区申报点变成了有解模板，后续项目配方的浏览器面可直接引。

## 判决

**通过**。六锚全绿，对抗锚经本席隔离突变复算证真（红得其所）；形制齐、范围净、暗债零瞒报；成品与设计-0007 及总设计 §9 双向对上。

META_VERDICT:pass:docs/实现/票-0012-判决/1-devin.md
