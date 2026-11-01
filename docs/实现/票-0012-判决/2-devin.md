# 票 0012 判决：通过（审计席·devin 独立实例，2026-11-01）

- 审计 run：`fe324eed84c7fba8`（shell=devin），触发＝deliver（事件 115，`devin exit=0`）＋前席 audit_dead 重派（事件 185）
- 施工 run：`8b43f7d02ddd8a25`（shell=devin），worktree `~/meta-worktrees/t0012`
- 交付物：支 `t/0012-seat-board` @ `e9641bd`（`git ls-remote origin` 核实已推；master=`af3feba` 未并，`git log --merges` 零合并），单 commit `t0012: 前端席位板——业主眼前的一面立起（§9）`，17 文件 +2935/−217；其后 `becfb8c` 为前审计席判决档 commit（审计簿记非施工面）
- 前席遗产：`1-devin.md`（claude 席，判通过）已 commit 但席死于哨兵前（SIGCHLD+EOF 双源）——本席**独立取证重判**，不引其结论为据；其 `/tmp/t0012-audit-recompute.txt` 仅作旁证

## 逐锚对账（票面六锚，全绿）

| 锚 | 机器证据 | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 施工机录 `/tmp/t0012.machine-record.txt`（go test 原文 tee）＝15 行全 `ok`；**本席在交付 worktree 原命令复算**（记录 `/tmp/t0012-audit2-recompute.txt`）：`go build ./... && go vet ./...` 链式静默过、`go test -count=1 ./...` 15 行全 `ok`（包集与施工机录全同、仅耗时列漂移）、`bash scripts/selfcheck.sh` 退码 `SELFCHECK_RC=0` | 绿 |
| 2 全态帧 | `TestBoard_full帧_组装全`（board_test.go:258–329 行断言区 293–329）：伪席 run 行＋非终态票入 full；断言 run_id/role/ticket_id、shell/model 拆分（devin/swe-2-high→两列）、stage=`写码`/note=`联调中`、预算四数 used=123/cap=400/usedTotal=123/hardCap=4000、attached=true；死席与终态票滤除（seats=1/tickets=1）。机器：双记录均 `ok meta/internal/board` | 绿 |
| 3 变化帧三型 | `TestBoard_变化帧三型`（board_test.go:333–398）：真 `r.g.Spawn`＋`syscall.Kill(SIGKILL)`→`seat_gone`；`Progress{stage:自测}`→seat upsert（stage 断言变）；`g.DispatchSeat`→ticket upsert（T2 相位`已派`） | 绿 |
| 4 token 闸 | `TestBoard_token闸_failclosed`（board_test.go:195–211）：secret 行缺→Connect 与 unix gRPC 两面皆 Unauthenticated；`TestBoard_token闸_错token`（board_test.go:214–241）：错/空/后缀 token 拒、对 token 过且首帧 full（种子带尾换行兼容）。`CheckBoardToken`（gateway/board.go:31–42）四路 fail closed＋`subtle.ConstantTimeCompare` | 绿 |
| 5 端到端 CLI | `TestBoardCLI_端到端`（main_test.go:43–108）：`boardCmd` 走 `--socket`（unix gRPC）与 `--connect`（TCP Connect 二进制）两径各验；读到 `FULL` 行后经真 `Attach+Progress` 触发第 2 帧，`--n 2` 零错退 | 绿 |
| 6 对抗锚 ≥2（本席突变复算，scratch `/tmp/t0012-mut2`，交付支未动） | a. `full()` 组装体删（留空板返回）→`go test -run TestBoard_full` FAIL `board_test.go:304: seats=0 预期 1`——锚红证实；b. `CheckBoardToken` 等值比较 `== 1` 翻 `!= 1`（gateway/board.go:42）→`go test -run TestBoard_token` FAIL `board_test.go:223: 应拒却放行`——锚红证实 | 绿 |

## 交付形制与范围

- 支名 `t/0012-seat-board` ✓；commit `t0012: <为什么>` ✓；已推（origin=e9641bd）✓；**不并 master**（`git log --merges master..HEAD` 空）✓。
- 交付摘要五节＋收口两步门 ✓（`run/logs/8b43f7d02ddd8a25.log` 尾部实录）：新增面表／根因／验证／下一步（四条全申报）／可沉淀候选；两步门经本席复核（worktree `git status` 净、经验库件在、机录在）。
- 范围：17 文件逐一落票面「范围内」——proto＋三生成件（pb/pb_grpc/metav1connect）／`internal/board` 两件／db 三查询件（全态组装输入）／gateway 两件（CheckBoardToken＋事件 emit 源）／cmd/meta 两件／config 种子行／web 页／go.mod,go.sum。`docs/经验库/2026-11-01-connect-es-无构建页骨架.md`＝票面附「浏览器连通性禁区申报点」的令#E1 一坑一文沉淀件，摘要 §5 已申报，非范围蔓延。
- 禁区机械自查：selfcheck R2（Ticker/Sleep）R3（encoding/json）零命中退码 0；Connect 二进制＝禁区#4 钦定豁免径；板只读 DB 组装无写径（禁区#6）；网关新增 if 皆机械判（token 比对/loopback 判，禁区#7 自检口令合法类）；无周期源——`serve()` 纯 `select` 事件循环（board.go:71–91）。
- 票状态无副本（R5）：变化帧内容恒为事件触发后 DB 现态重读（board.go:127–155 `frames()`），先订后装（board.go:80–83）消组装间隙丢失；页端 Map 为显示态非真源。
- 依赖：`connectrpc.com/connect/v2 v2.0.2`——禁区「待业主裁」白名单第 4 类（connect）内；模块缓存 `v2.0.2.info` 亲证 `Time:2026-10-21T22:05:54Z`＝11 天 ≥7 天钉版 ✓。

## 设计对账（设计-0007 现经 `c/0007-design` 支可达）

票面设计引用栏 `docs/实现/设计-0007-阶段3.md` 在交付支/master 均不存，但 `c/0007-design` 支可达（0008 判决立账 D-0008a 已记此漂移）。本席取该支档核 §1.10/§3/§7：`SeatBoard(BoardReq)→stream BoardFrame` ✓、`--board-listen`/`board_listen` 配置 ✓、token 对 `secret.board.token` 行缺 fail closed（"ACL 空＝网关自校"＝`SecretBlob` 免 ACL 网关内径的设计出处）✓、三型变化帧幂等重放 ✓、`meta board` CLI 验收径 ✓、`web/board.html` connect-es CDN 无构建 ✓——全对上；总设计 §9「接入全态→只推变化帧→不存历史不查过去」对上。总设计 §12 接口草图 `SeatBoard(Empty)` 被票面/设计-0007 的 `BoardReq{token}` 取代——草图注明「字段可在实现中补」，以票面为准 ✓。

## 质量三问（令#E5）

- **最优解**：单 `board.Service` 两出——`GRPC()` 适配器入 unix socket、`connecthttp.Mount` 走 TCP，零双实现（board.go:38–66）；复用网关既有 `subscribe` 事件总线与 `GetBudget`/`NonTerminalTickets` 查询件，新增三查询皆最小列；依赖方向净——`internal/gateway` 零 import board（board 经 `Deps` 函数槽反向注入），接线只在 `cmd/meta` `wireBoard`（main.go:563–577）。
- **第一性**：幂等由「恒重读 DB 现态」构造保证而非去重缓存——R5 的直接落形；token 闸一处 `serve()` 入口判、两适配器各翻本侧错误码，无 workaround 摞补。
- **洁净**：diff 增行扫 TODO/FIXME/XXX/HACK/绕过/半成品/临时 零命中；pb 删除行抽样全为 rawDesc/goTypes 索引漂移（再生噪声），无手改痕；`run == nil` 分支可达（`scanRun` ErrNoRows→nil,nil）。

## 债项节（暗债对账：diff 扫描 vs 摘要「下一步」）

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| D-0012a | proto 合并面：与 0011 同文件 meta.proto（`GatewayConfig` 字段 11 对 0011 ceo_* 键），并入序晚者 rebase＋pb 重生成 | 已申报（票面附亦预告） |
| D-0012b | 部署备忘：`meta secret put secret.board.token` 落种＝业主凭证动作；`board_listen: "127.0.0.1:9712"` 已种。行缺即 fail closed | 已申报 |
| D-0012c | 设计引用漂移：票面引 `设计-0007-阶段3.md` 不在交付支（存 `c/0007-design`）——对账已本席代做，全对上 | 已申报（摘要 §4 第三条） |
| D-0012d | 已知限制：订户口 256 缓冲满丢帧（与 Observe 面同语义）；`Listener.Close` 不断存量连接（进程退出即全断） | 已申报 |
| 观察 | go.mod connect v2 标 `// indirect` 实为直接 import（`go mod tidy` 会归位）；`stageOf` 复用 LatestStages 全量图换单票查询（以复用换小过度取）；CORS `*`（关防在 token 非 origin，board.go:212 注释已载明推理）；终态相位 upsert 上行由前端滤（proto 注释钦定「板只 upsert 不删行」）——皆非债级 | — |

**瞒报零起**：diff 内未申报 TODO/绕过/半成品零命中；摘要「下一步」四条与本席 diff 扫描逐条对上。

## 沉淀意见（令#E8，纯意见不构成否决依据）

「Connect 双面服务」打法（一份 service＋`GRPC()` 适配器＋`connecthttp.Mount`，CLI 验收与浏览器径零双写）配 connect-es 无构建页骨架（fileDesc 内嵌描述符＋esm.sh `?deps=` 钉共享 protobuf 版本）——**可沉淀**：施工已自沉 `docs/经验库/2026-11-01-connect-es-无构建页骨架.md`，本席亲验其解法与交付码一致（codegenv2 子路径/deps 钉版/CORS 预检头列），后续浏览器面可直接引。

## 判决

**通过**。六锚全绿，对抗锚经本席独立 scratch 突变复算证真（红得其所）；形制齐、范围净、暗债零瞒报；成品与设计-0007 §1.10/§3/§7 及总设计 §9 双向对上。

META_VERDICT:pass:docs/实现/票-0012-判决/2-devin.md
