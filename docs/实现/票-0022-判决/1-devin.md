# 票 0022 判决：通过（审计席·devin 独立实例，2026-11-02）

- 审计 run：`ea9e29e517746621`（shell=devin/swe-2-max），触发＝deliver（事件 1431，`18f6e6a devin exit=0`）→ audit_start（事件 1436）
- 施工 run：`3ffcd4f1c1a06b8e`（shell=devin/swe-2-high），worktree `~/meta-worktrees/t0022-audit-shellbias`
- 交付物：支 `t/0022-audit-shellbias` @ `18f6e6a`（`git branch -r` 核实 `origin/t/0022-audit-shellbias` 已推；merge-base `master...HEAD`＝`5f66e21`＝票面钦定预制品；`git log --merges` 零合并未并 master），单 commit `t0022: 审计席死→重派同壳连败照试，单壳循环烧穿 opencode 日限（D-46）`，7 文件 +315/−38
- 机器记录：无 `/tmp/t0022*.machine-record.txt`；以施工 run 官方会话实录 `/opt/meta/run/artifacts/3ffcd4f1c1a06b8e/atif.json` 为准（step31–39 的 exec 观察值＝harness 机录退码＋输出原文，非自述），另本席在交付 worktree 原命令独立复算，记录 `/tmp/t0022-audit-recompute.txt`——两径互证

## 逐锚对账（票面 §6 六锚，全绿）

| 锚 | 机器证据 | 判 |
|---|---|---|
| A1 连败跳过 | `TestAuditShellBias_Skip`：devin/swe-2-max 审计行 ×2（Dead+ExitCode=1，最新两枚）→ prefer=devin 仍被剔，`waitEvent("audit_shell_skip")` 命中且 note 含 "devin"，seat_up 后末 run＝claude 顶上，ticket_event 恰 1 行。atif step33 `Go test: 4 passed`；本席复算 `--- PASS (0.09s)` | 绿 |
| A2 阈值边界 | `TestAuditShellBias_Threshold`：仅 1 败→devin 照试立住（末 run=devin/swe-2-max）；败-成（ExitCode=0)-败夹序→断计不跳且 `audit_shell_skip` 零行。同 step33/复算 `--- PASS (0.13s)` | 绿 |
| A3 角色隔离 | `TestAuditShellBias_RoleIsolation`：role=施工 连败行不进审计偏置账（审计派发照立 devin/swe-2-max）；反向，审计连败行在而 role=施工 派发→偏置不生效（末 run=devin/swe-2-high，零 skip 事件）——`req.role=="审计"` 单闸（dispatch.go 新增段首行）。同 `--- PASS (0.13s)` | 绿 |
| A4 剔空归宿 | `TestAuditShellBias_Exhausted`：claude/devin 各 2 连败（分块交叉）→ `audit_shell_skip` 恰 2 行＋`shells_exhausted` 事件命中＋runs 总数仍 4（零新 spawn）。同 `--- PASS (0.09s)` | 绿 |
| 对抗锚（必红证） | 施工实跑留变异记录（atif step34–36）：①删偏置接线行→`TestAuditShellBias_Skip` FAIL（audit_shell_skip 等事件超时 8s）；②`ExitCode !=0→==0` 翻判→`TestAuditShellBias_Skip`+`TestAuditShellBias_Exhausted` 双 FAIL；回退复跑 `Go test: 4 passed`。交付 diff 中无 MUTANT 残渣（dispatch.go 新增段亲读） | 绿 |
| A5 全量 | atif step37/38：`gofmt -l` 零行 rc=0、`go build && go vet && go test -count=1 ./...` → `Go test: 154 passed in 15 packages` exit 0；step39 `selfcheck rc=0`、`grep audit_shell_skip` 命中 textproto:10＋dispatch.go:602/604。本席复算 BUILD_OK/VET_OK/锚测 4 PASS/GOFMT_CLEAN/SELFCHECK_RC=0 | 绿 |

## 交付形制与范围

- 支名 `t/0022-audit-shellbias` ✓；commit `t0022: <为什么>` ✓；已推 origin ✓；未并 master ✓；`META_DELIVER:18f6e6a` 落 `$META_ART_DIR/sentinel.log`（本席亲读文件），deliver 事件 ref 与 HEAD 短哈希对上 ✓。
- 交付摘要五节（atif step47 实录）：新增/修改面、根因、验证引、下一步＝全量遗留申报、可沉淀打法候选——形齐。
- **范围复核**（网关前置闸报告未见，本席机械比对 diff 文件清单 vs 票面 §3）：7 文件全部落名——`proto/meta/v1/meta.proto`（钦列）→ `internal/pb/metav1/meta.pb.go`（钦定 regen 产物）、`config/gateway.textproto`、`internal/gateway/gateway.go`（Config 字段）、`internal/gateway/dispatch.go`、`internal/gateway/t0022_test.go`（测试面新增）、`cmd/meta/main.go`（钦列接线）。`internal/config/config.go` 票面虽列名但**零改动合法**：`config.Load` 直返 `*pbv1.GatewayConfig`（config.go:28），textproto unmarshal 自动吃新字段 16，无解析码可改——「解析落 GatewayConfig 新字段」由 regen 自然满足，非漏做。范围外零蔓延。
- 禁线核查：偏置段唯一闸 `req.role=="审计" && cfg.AuditShellSkipStreak>0`（施工/CTO 径不碰）；diff 对 failSet/NextHealth/health 列/reportShell 零改动；caster/路由表/席位面零触；不部署不并 master 不碰生产 DB；阈值仅 `textproto:10` 一处落值，码内读 `g.cfg.AuditShellSkipStreak`（测试 fixture 自赋值属测试面惯例，非第二处写死）。

## 设计对账（设计-0020 §3，支 c/0019-design 亲读）

- §3 修法①：`RunsForTicket`（shell.go:76，`ORDER BY started_at` 升序→倒扫即新→旧）滤 role=审计 非 adjunct 行，`Dead && ExitCode!=0` 计连败、遇他壳（`closed[prev]`）或 `ExitCode==0`/未判死行断计——与设计逐字对 ✓
- §3 修法②：连败 ≥N 自 attempts 剔除＋AppendEvent+emit `audit_shell_skip`，note＝`"%s 连败 %d 本轮跳过"`＝壳名+连败数 ✓；N 缺省 2 落 `audit_shell_skip_streak: 2`，proto #16 →regen ✓
- §3 修法③：剔空落既有 `len(attempts)==0→shellsExhausted` 径 ✓（偏置插段在 preferFirst 后、空序判前——票面许可两码位之一，注释已标）
- §3 边界：只偏置审计重派、健康迁移不碰、不标壳坏——实现无任何 health/failSet 写面 ✓

## 质量三问（令#E5）

- **最优解**：复用既有面——`RunsForTicket`/`shellNameOf`/`AppendEvent`/`emit`/`route.Attempt` 零新增抽象；单次过滤一遍成 `kept` 序，无逆向依赖。无更简等价解。
- **第一性**：在派壳决策点（dispatchOnce attempts 成序后）做序内剔除——正中「同壳连败照试」根因；`RunsForTicket` 取错 fail-open 照原序试（注释明言「偏置是优化不是闸」），非 workaround 摞补。
- **洁净**：diff 增面扫 `TODO/FIXME/HACK/XXX/绕过/临时` 零命中；变异行全回退无残渣；`closed`/`prev` 两器实现「他壳断计+块闭合」，与同族连败语义对应无重复实现；测试注入钟（`SetClock` 步进）保证 started_at 严格序，免真 sleep。

## 债项节（暗债对账：diff 扫描 vs 摘要「下一步」）

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| D-0022a | `audit_shell_skip` kind 未入 `default_notify` 种子——那是 0021 的表；0021 未并前事件仅落 ticket_event/事件侧流，不进外部通知（本席核：config/ 内 default_notify 尚不存在，0021 支尚无 commit 落其面） | 已申报＋设计 §3 自载此依赖 |
| D-0022b | N=0 关断径无单列测试——门槛 `>0` 机械生效，票面未列此锚 | 已申报 |
| 观察 | 连败键＝壳名（`shellNameOf` 剥 model 段）——钦定粒度「逐壳」非「壳+型」；started_at 纳秒序，同刻在产线可忽略；在飞未判死审计行断计＝与票面「Dead 且 ExitCode≠0」连败定义逐字一致 | — |

**瞒报零起**：diff 内 TODO/绕过/半成品零命中；摘要「范围内项全做、无降级实现、无绕过路径」与本席扫描逐条对上。

## 沉淀意见（令#E8，纯意见不构成否决依据）

可沉淀。「新→旧逐壳块闭合」连败账（`closed` 标块尾，`streak` 只认最新同壳块）一行两器同时兑现「遇他壳断计」与「各壳独立达阈」两条钦定语义，比全序列计数或 per-shell 滑窗都简；测试侧 `SetClock` 注入钟保 started_at 序免真 sleep 免同刻乱序，值得写进经验库供 seat_run 序依赖类测试复用。
