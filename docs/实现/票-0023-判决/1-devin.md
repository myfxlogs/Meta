# 票 0023 判决：通过（审计席·devin 独立实例，2026-11-02）

- 审计 run：`027a78ce807b7f17`（shell=devin/swe-2-max），触发＝deliver（事件 1463，`55739a3487517a5d… devin exit=0`）→ audit_start（事件 1464）
- 施工 run：`3d57454a2e0756b2`（shell=devin/swe-2-high；前任 `109bd23f1f4b7a54` 死于 progress 中途，事件 1458 requeue 后由本任接替收口），worktree `~/meta-worktrees/t0023-board-verify`
- 交付物：支 `t/0023-board-verify` @ `55739a3`（atif step26 实录 `git rev-parse HEAD origin/t/0023-board-verify` 两值相等＝已推；主检 `git branch -r` 见 remote-only `t/0023-board-verify`），单 commit `t0023: 前端板半活→已验——…`，4 文件 +267/−0：`scripts/board-verify.sh`（新建，mode 100755，168 行）＋`docs/经验库/` 三坑文
- 机器记录：施工未落 `/tmp/t0023*.machine-record.txt`；席位实录 `run/artifacts/3d57454a2e0756b2/atif.json` 各令输出原文在。本席照票-0017 先例在交付 worktree **全锚原命令复算**，记录 `/tmp/t0023-audit-recompute.txt`

## 逐锚对账（票面 §6 五锚，全绿）

| 锚 | 机器证据（本席复算，atif 实录同果） | 判 |
|---|---|---|
| A1 正锚 | `bash scripts/board-verify.sh` → `scratch: tmp=/tmp/board-verify.pozr5Z board=127.0.0.1:39467`，stdout 末行 `BOARD_VERIFY OK full>=1 upsert>=2`，`A1_RC=0`。verdict() 内 `grep -c '^FULL '`≥1 且 `grep -c '^TICKET_UPSERT'`≥2 才放此判行（scripts/board-verify.sh verdict 函数），票面「FULL 行≥1、TICKET_UPSERT 行≥2」口径照字面满足（行计数非帧计数，头注帧账已申明） | 绿 |
| A2 反锚 | `BOARD_CONNECT=http://127.0.0.1:1 bash scripts/board-verify.sh` → `BOARD_VERIFY FAIL full=0 need=1 upsert=0 need=2 rc=1`，`A2_RC=1`；脚本不误放绿 | 绿 |
| A3 无泄漏 | 复算三跑后 `pgrep -af 'meta-bin|board-verify\.'` 仅剩探针自身行；`ls -d /tmp/board-verify.*` 零命中；trap cleanup 三子进程（serve/client/REPL）kill＋kill -9 兜底＋`rm -rf $TMP` 皆 `|| true`（set -e 下半路死实证教训已入经验库文二）。另有外来残程 `/tmp/meta serve`×2（PID 2465259/2465532，11-01 18:39 起，二进制名 `/tmp/meta` 非 `meta-bin`，TMP 径非 `board-verify.*`）——票前旧物与本交付无关，施工已在「下一步」申报 | 绿 |
| 对抗锚 | `BOARD_NEED_UPSERT=99 bash scripts/board-verify.sh` → `BOARD_VERIFY FAIL full=1 need=1 upsert=3 need=99 rc=0`，`ADV_RC=1`——阈值真卡帧数（实测行数 3 被如数计入仍红），非恒绿摆设 | 绿 |
| A4 全量 | `bash -n` → SYNTAX_OK；`go build ./... && go vet ./...` → BUILD_VET_OK；`go test -count=1 ./...` → 150 用例 15 包全过（尾部 `ok meta/internal/…` 全绿）；`bash scripts/selfcheck.sh` → SELFCHECK_RC=0 | 绿 |

## 交付形制与范围

- 支名 `t/0023-board-verify` ✓；commit `t0023: <为什么>` ✓；已推未并 master ✓；交付摘要五节齐（atif step28 实录：新增/修改面、根因/偏差、验证引机器口径、下一步＝全量遗留申报、可沉淀打法候选）；`META_DELIVER:55739a3…` sentinel.log 双发在案（artifacts/3d57454a…/sentinel.log）。
- **范围复核**（diff 文件清单 vs 票面 §3）：新增仅 `scripts/board-verify.sh` 一件钦定面＋经验库三文（施工工作法#6「遇坑即写」义务内件，非码面蔓延）。`internal/board`、`cmd/meta`、`proto`、`web/board.html` 零触；生产网关/DB/在飞席位零触（scratch 全程 `/tmp/board-verify.*` 内，`--board-listen` 随机空闲口不碰 9712）；live 径 token 只读 `META_BOARD_TOKEN` env，仓内零 token 字面值（`grep token` 唯一现形 `secret.board.token` 行名＋scratch 自种的 `tok` 测试值）。go.mod/go.sum 零改动；无周期任务（等待全有界：WAIT_FULL 10s/EXIT 15s/REPL 10s——禁令#1 边界条款有界重试链合法，票面 §4 亦钦定「脚本有界等待是测试语义」）。

## 票面字面三处偏差（施工申报＋本席码证，锚口径不动）

| 票面字面 | 实做 | 码证 | 判 |
|---|---|---|---|
| `meta rule --action void --ticket X --confirm`（§3.5） | `meta ceo` REPL 经 FIFO 喂 `rule void <票> confirm` | `ruleCmd`→`ceoClient`＋`RunToken: ceoToken()`（cmd/meta/main.go:1024；ceoToken 读 `META_RUN_TOKEN`，main.go:875）；`ceoByToken` 要求 role=CEO（internal/gateway/ceo.go:30-31）——施工席 token 必被拒；且 scratch 网关独立 DB，生产 token 在其中根本无档。REPL 径 `IssueCeoToken` 对 scratch 库自发 CEO token（main.go:674），同一 `CeoRule` RPC——是可行径中的唯一直径 | 合理，不冤 |
| `--n 3`（§3.4，FULL＋2 upsert＝3 帧账） | `--n` 缺省 4（`BOARD_N` 可覆） | `frames()` 事件映射：ruling 事件带 CEO RunID→RunID 分支产 SEAT_UPSERT＋TicketID 分支产 TICKET_UPSERT（internal/board/board.go:136-151），一次 void 实产 3 帧非 2——票面账漏计 ruling 夹的 SEAT 帧。锚口径（FULL≥1＋UPSERT≥2）不变，n=4 是达标所需最小值 | 合理，不冤 |
| `secret put --key $TMP/vault.key`（§3.1） | `KEY=$RUND/vault.key`（落 rundir 内） | 网关只载 `<RunDir>/vault.key`（internal/gateway/gateway.go:236），钥指别处＝vaultKey nil→CheckBoardToken fail closed「token 无效」——票面位置照字面实现＝恒红。票面 §3.1 自带「实现以现码行为为准」让位句 | 合理，不冤 |

## 设计对账（设计-0020 §4）

钦定面全落：mktemp 自含 scratch＋secret 落种＋serve 独立 db/sock/rundir/board_listen＋先立票后接入＋Connect 客户端拉帧＋void 泵变化帧＋断言 FULL≥1/UPSERT≥2＋有界等待≤15s＋trap 清场＋`--live` env-token 生产复验径。设计速写伪码中的 rule/钥位/n 三处与现码冲突，施工按「现码行为为准」实现并把三坑入档——偏差皆有码证非施工臆改。

## 质量三问（令#E5）

- **最优解**：复用现成 CLI 面（secret put/serve/ticket/ceo/board）零新依赖、零码改；CEO 打点选 REPL＋FIFO 持写端（`exec 3<>` O_RDWR 防 open 阻塞）而非绕道改 auth——无更简等价解（meta rule 无凭、生产径禁令）。
- **第一性**：真 Connect 二进制径过线——与浏览器同一协议面，token 闸/挂板/全态组装/帧泵任一断即红，直击「板面全链是否真活」本质；非纸面断言。
- **洁净**：diff 扫 TODO/FIXME/XXX/HACK/绕过/调试残渣零命中；诊断全走 stderr（say()）、stdout 只留机器判行；trap 每行 `|| true` 防半路死；fd3/ FIFO/三 PID 收口有始有终。168 行对端到端自举体量相称。

## 债项节（暗债对账：diff 扫描 vs 摘要「下一步」）

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| D-0023a | `--live` 生产复验未跑（需业主供 `META_BOARD_TOKEN`，票面列为可选） | 已申报 |
| D-0023b | 票面字面三处偏差（REPL 代 rule CLI、n=4 代 n=3、钥落 rundir） | 已申报（commit 正文＋摘要根因节＋经验库三文） |
| D-0023c | 外来残程 `/tmp/meta serve`×2（PID 2465259/2465532，11-01 18:39 起）非本票产物，建议另票清理 | 已申报（「旁见·范围外未动」） |
| 观察 | A2 反锚径仍先自举整套 scratch 再打死地址（~秒级浪费，换形制统一）；端口探测 bind→close→listen 有理论竞态（无碍判性）；`upsert` 计行非计帧（FULL 展行亦计——与票面「行」口径字面一致，头注已申明） | — |

**瞒报零起**：diff 内 TODO/绕过/半成品零命中；施工申报三偏差＋一外来残程，与本席 diff/机器复核逐条对上——报重于缺，申报足额。

## 沉淀意见（令#E8，纯意见不构成否决依据）

「scratch 自举验收径」（独立 DB/socket/rundir＋env 阈值对抗钮 `BOARD_NEED_*`＋FIFO-REPL 打点 CEO）——**可沉淀**：凡需端到端验「服务真活」的票，此三件套（自举、对抗钮、REPL 桥无凭打点）可直接复用；三坑文已就各坑单独成档。建议 CEO 收编打法库。

## 判决

**通过**。五锚全绿（本席独立复算＋atif 实录双源一致）；票面字面三偏差皆有现码硬约束佐证、锚口径未动、已全量申报；范围净、形制齐、暗债零瞒报。

META_VERDICT:pass:docs/实现/票-0023-判决/1-devin.md
