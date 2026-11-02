# 票 0022 判决：通过（审计席·claude 独立实例 22aa1980ad234e07，2026-11-02）

- 前席说明：审计 run `ea9e29e517746621`（devin/swe-2-max）已 commit 判决档 `1-devin.md`（17ddec1，判通过）但哨兵漏发＝死席＝等同未判——本席全量独立复算，不复用其结论；其档留支作史。
- 施工 run：`3ffcd4f1c1a06b8e`（devin/swe-2-high，exit=0）；交付＝支 `t/0022-audit-shellbias` @ `18f6e6a` 单 commit，merge-base＝`5f66e21`＝票面钦定 off 点，未并 master（`git branch --contains 18f6e6a` 仅本支），已推 origin（本席 `git ls-remote` 亲核）。
- 证据径：施工官方会话实录 `/opt/meta/run/artifacts/3ffcd4f1c1a06b8e/atif.json`（harness 机录）＋本席在交付 worktree 独立复算（记录 `/tmp/t0022-audit2-recompute.txt`）＋本席独立变异自证（临时 detached worktree，跑毕即删，交付支未触）。

## 逐锚对账（票面 §6 六锚，全绿）

| 锚 | 机器证据（本席复算为主，atif 实录互证） | 判 |
|---|---|---|
| A1 连败跳过 | `TestAuditShellBias_Skip` 本席复算 `--- PASS (0.09s)`：devin/swe-2-max 审计行 ×2（MarkExited 1＋MarkDead）→ prefer=devin 仍被剔，`waitEvent("audit_shell_skip")` 命中且 note 含 devin，末 run＝claude 真实立住（真 spawn 断言，非伪记录），ticket_event 恰 1 行（t0022_test.go:100-118） | 绿 |
| A2 阈值边界 | `TestAuditShellBias_Threshold` PASS (0.12s)：仅 1 败→devin 照试（末 run=devin/swe-2-max）；败-成(ExitCode=0)-败夹序→断计不跳＋skip 事件 0 行（t0022_test.go:122-147）。算法面：`closed[s]=true` 断计后同壳旧败行 `continue` 不再累计（dispatch.go:587-598），他壳行经 `closed[prev]` 闭合上一壳段——「遇他壳或 ExitCode==0 断计」两断条款码面兑现 | 绿 |
| A3 角色隔离 | `TestAuditShellBias_RoleIsolation` PASS (0.09s)：双向——施工连败行（devin/swe-2-high，shellNameOf 同剥为 devin）不进审计账（若误计则 devin 被剔末 run 必为 claude，断言仍 devin/swe-2-max）；审计连败行在而施工派发照原序（末 run=devin/swe-2-high＋零事件）。唯一闸＝`req.role=="审计" && g.cfg.AuditShellSkipStreak>0`（dispatch.go:289） | 绿 |
| A4 剔空归宿 | `TestAuditShellBias_Exhausted` PASS (0.07s)：claude/devin 各 2 连败（分块交叉，两壳各自达阈）→ `shells_exhausted` 事件命中＋skip 恰 2 行＋RunsForTicket 仍 4 行（零新 spawn）——剔空走既有 `len(attempts)==0→shellsExhausted` 径（dispatch.go:293-296） | 绿 |
| 对抗锚（必红证） | 三径互证：①施工实录 atif step35——删偏置接线行→`TestAuditShellBias_Skip` FAIL（audit_shell_skip 超时 8s）；②atif step36-37——翻 `ExitCode!=0→==0`→Skip＋Exhausted 双 FAIL，回退后 4 passed（rc=0）；③**本席独立变异自证**——临时 worktree @18f6e6a 上翻同一判定→A1 FAIL、删同一接线→A1 FAIL（两跑均 `0 passed, 1 failed`，worktree 跑毕即删，交付 diff 亲读无变异残渣） | 绿 |
| A5 全量 | 本席复算（/tmp/t0022-audit2-recompute.txt）：`go build ./...` BUILD_OK；`go vet ./...` VET_OK；`go test -count=1 ./...` 15 包全 ok（gateway 18.76s 含全测试面）；`gofmt -l internal/ cmd/` 零行；`bash scripts/selfcheck.sh` rc=0；`grep -n audit_shell_skip` 命中 config/gateway.textproto:10＋dispatch.go:567/602/604。atif step39/40 互证（154 passed/15 包、selfcheck rc=0） | 绿 |

## 范围与禁线复核

- 7 文件全落票面 §3 名单：meta.proto（#16 字段）→meta.pb.go（regen 纯结构重排＋新字段＋getter，protoc 标准产物，本席逐行亲核 diff 无夹带）、gateway.textproto（落 2）、gateway.go（Config 字段＋注释）、dispatch.go（偏置段＋skipLosingAuditShells）、t0022_test.go（新增）、main.go（接线）。**config.go 零改动合法**：`config.Load` 直返 `*pbv1.GatewayConfig`（config.go:28-34），textproto unmarshal 自动吃字段 16，无解析码可改——票面「解析落 GatewayConfig 新字段」经 regen 自然满足，非漏做。
- 码位合规：偏置插在 preferFirst 后、空序判前（票面许可两码位之一），注释自标码位（dispatch.go:283-288）。
- 禁线全绿：偏置唯一闸 role=="审计"（施工/CTO 径不碰，A3b 反向断言）；diff 对 failSet/NextHealth/health 列/reportShell 零改动；caster/路由表/席位零触；不部署不并 master 不碰生产 DB；N 值仅 textproto:10 一处落 2，码内读 `g.cfg.AuditShellSkipStreak`（测试 fixture 自赋值属测试面惯例，非第二处写死）。
- 交付形制：commit `t0022: <为什么>` ✓；施工哨兵 `META_DELIVER:18f6e6a` 落 atif step47＋`artifacts/3ffcd4f1c1a06b8e/sentinel.log`（本席 cat 亲核，21B）✓；摘要五节形齐（atif step48 亲读）✓。

## 设计对账（设计-0020 §3）

RunsForTicket 倒扫（`ORDER BY started_at` 升序→倒读即新→旧）滤 role=审计 非 adjunct；`Dead && ExitCode!=0` 计连败；他壳/成功行/未判死行断计；≥N 剔除＋AppendEvent+emit 双发（note=壳名+连败数）；RunsForTicket 取错 fail-open 照原序（注释明言「偏置是优化不是闸」）——与设计逐条对上。

## 债项节（暗债对账：diff 扫描 vs 摘要「下一步」申报）

| 债 | 内容 | 对账 |
|---|---|---|
| D-0022a | `audit_shell_skip` 未入 default_notify 种子——属 0021 面，0021 未并前仅落 ticket_event/事件侧流 | 摘要已申报；跨票依赖票面附已注 |
| D-0022b | N=0 关断径无单列测试（门槛 `>0` 机械生效） | 摘要已申报；票面未列此锚 |
| 非-债 | `_ = g.database.AppendEvent(...)` 忽错 | attach.go:147/204/226 等 ≥7 处既有同款惯例，非本票新债 |

**瞒报零起**：diff 增面 grep `TODO|FIXME|HACK|XXX|fmt\.Print|debug` 零命中（本席跑 CLEAN）；摘要「范围内项全做、无降级实现、无绕过路径」与扫描逐条对上。

## 沉淀意见（令#E8，纯意见不构成否决依据）

可沉淀。「新→旧逐壳块闭合」（`closed`/`prev` 两器）一行同时兑现「遇他壳断计」与「各壳独立达阈」两条钦定语义，比全序列计数或滑窗皆简；测试侧 `SetClock` 注入钟保 started_at 严格序（连败账赖扫序）免真 sleep，值得入经验库。
