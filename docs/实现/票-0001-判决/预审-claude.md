# 票 0001 审计判决（票-0001-网关骨架）

审计席：claude 壳独立实例（施工＝devin，异壳成立）。判决依据＝机械材料原文：
`~/meta-worktrees/t0001.code.diff`（4557 行）、`t0001.selfcheck.diff`、`t0001.machine-record.txt`、
worktree `~/meta-worktrees/t0001` 源码与 git 对象、`/opt/meta/docs/总设计.md` §5.3/§6.2/§12、
`docs/接手者禁区.md`。未运行任何 go 命令；全部验证结论取自 CEO 机械代跑记录。

# 判决：通过

## 逐锚打勾表

| 锚 | 子项 | 判 | 证据 |
|---|---|---|---|
| 1 | go build ./... && go vet ./... 绿 | ✔ | machine-record.txt L1–4：`rc=0`×2 |
| 2 | go test ./... -count=1 绿 | ✔ | machine-record.txt L5–11：clock/db/gateway 三包 `ok`，无 FAIL，`rc=0` |
| 2a | 先连后领：spawn 后不连接＝不产生 施工中 | ✔ | code.diff L1912–1933 `TestAnchorA_ConnectThenClaim`：idle 席位 300ms 窗口零 attach/claim 事件、相位保持 已派、`attached_at` 零值三重断言 |
| 2b | 边表逐边：§6.2＋补边全边，漏一边＝红 | ✔ | code.diff L630–663 `TestAnchorB_EdgeTable`：测试自带 22 边独立副本（L594–619），与实现表 `Edges`（L531–555）逐条对照＋条数断言＋10 相位×15 kind 全组合扫描非法边。副本与总设计 §6.2（主链 8＋回队＋驳回＋补边 4＋非终态×作废 8）逐边一致 |
| 2c | 中途死：SIGCHLD＋EOF 双源→施工中→已派 | ✔ | code.diff L1936–1966 `TestAnchorC_MidDeath`：SIGKILL→`requeue` 事件→相位 已派，且断言 `dead && exited_at≠0 && stream_closed`（SIGCHLD 源＝spawn.go L2898–2911 `cmd.Wait()` goroutine；EOF 源＝attach.go L1440–1454 `Recv` 出错→`onStreamClosed`） |
| 2d | 全相位对账：五相位杀网关→重启逐相收养/回队/补 spawn | ✔ | code.diff L1971–2081 `TestAnchorD_AllPhaseReconcile` 11 例：设计中/施工中/审计中/部署中×(活→`adopted`＋`adopted_at`／死→`requeue` 回 已立/已派/待审/待部署)，已派/待审/待部署无行动者→`spawn_needed` 角色 施工/审计/CTO。与总设计 §5.3 表逐格一致（reconcile.go L2708–2794） |
| 2e | 闹钟重算：deadline 取自 ticket_event；已过期→立即发事件 | ✔ | code.diff L2085–2163 `TestAnchorE_AlarmRecompute`：时钟回拨 −1h→重启 8s 内收 `stall_suspect`（过期立即响）；未过期→`AlarmDeadline＝LastActivityAt+30min`（±2s 断言）＋300ms 窗口无 stall。重算径 reconcile.go L2798–2804＋db.go `LastActivityAt`（code.diff L1350–1366，SQL 取 `kind IN ('progress','claim')` 的 MAX(created_at)） |
| 2f | 伪连接断流：无 token/错 token 的 Attach 被断 | ✔ | code.diff L2166–2206 `TestAnchorF_FakeConnCut` 三例（首帧非 Auth／forged-token／静默超时）均断言 `codes.Unauthenticated`；实现＝attach.go L1414–1454（首帧验签、`RunByToken` 查无/dead 即断） |
| 2g | 收养互证：收养首帧网关出示 run_token；错 token 席位拒续且留痕 | ✔ | 网关出示：reconcile.go L2817 `Adopt(cctx, &pbv1.AdoptAuth{RunToken: run.RunToken})`。测试：code.diff L2210–2246 `TestAnchorG_AdoptMutualAuth`——forged token→席位 `PermissionDenied`＋trace 文件含 `refused`；正确 token 径由锚 d live 例经真实 `tryAdopt` 产出 `adopted` 事件闭环 |
| 2h | 迁移烟测：九表齐、WAL 开 | ✔ | code.diff L707–728 `TestAnchorH_MigrationSmoke`：九表名逐一对照＋`journal_mode＝wal`；schema.go L945–1055 建全九表 |
| 3 | bash scripts/selfcheck.sh 退出码 0 | ✔ | machine-record.txt L12–13：`rc=0`；selfcheck.diff 全文（与 227eaa1 blob `0ddea7c` 逐字一致） |
| 4 | 备份烟测：VACUUM INTO 产物可只读打开且表数一致 | ✔ | code.diff L731–754 `TestAnchor4_BackupSmoke`：`Backup`→重开→表数相等＋票行随行。备注：「只读打开」以普通 `Open`（可写径）验证——可写可开蕴含可读可开，判据不弱于票面 |

## 交付形制

- 支名 `t/0001-gateway-skeleton` ✔（worktree `git branch --show-current`）
- 交付 commit `227eaa1`，提交信息 `t0001: 网关骨架——v1 两死因（心跳假活、文件/DB相位双源）的对症结构。`——符合 `t0001: <为什么>` ✔
- 已推送：`origin/t/0001-gateway-skeleton`＝227eaa1 ✔；未并 master（基＝merge-base 63d692f）✔
- 工作区干净（`git status --porcelain` 空）✔

## CEO 六项特别质询

1. **selfcheck 语义改动**——现存三条检查（selfcheck.diff L28–32）：
   - `time\.(NewTicker|Ticker|Tick\()`：`NewTicker` 调用、`Ticker` 类型引用、`Tick(` 调用三种直写形式全命中；
   - `time\.Sleep` 全禁（含测试）：票面禁式「无界 for { sleep }」的构成要件即 sleep，全禁是覆盖该禁式的更强检查；
   - `"encoding/json"` 导入路径精确匹配。
   三禁式均抓得住，无洞。被删的「无界 for」形检查针对无 sleep 的循环，而票面禁线文字（范围内第 4 栏）＝「任何 `time.Ticker`、无界 `for { sleep }`、`encoding/json`」——无 sleep 的 `for { stream.Recv() }` 本就不在禁线内。机器记录 selfcheck `rc=0`。备注（不构成驳回）：grep 模式可被 `import . "time"` 类 dot-import 绕过——grep 式机械闸的固有边界，票面钦定的检查方式即 grep，非本次改动引入的洞。
2. **锚 b 全边**——覆盖。测试自带 22 边独立副本与实现表 `Edges` 双向对照（条数 22＝22、集合互含），另有 10 相位×15 kind 全组合负扫；副本内容与总设计 §6.2（主链 8＋回队＋驳回＋补边 4：`设计中→已立`/`部署中→待部署`/`审计中→待审`/`施工中→已派(Yield)`＋非终态×作废 8）逐边一致。实现表或测试副本任一方缺边/多边均红。
3. **锚 d/e 全相位**——覆盖。锚 d：11 测试例＝四有行动者相位×(活收养/死回队)＋已派/待审/待部署补 spawn，回队目标相位逐格符合 §5.3 表（设计中→已立、施工中→已派、审计中→待审【不计驳回：走 `audit_dead` 边，非 `reject`】、部署中→待部署）；锚 e：过期立即发事件（时钟回拨布置）＋未过期 deadline 取自 `ticket_event`（±2s 数值断言）两子测俱在。
4. **锚 f/g**——各有具名测试：`TestAnchorF_FakeConnCut`（无 token/错 token/静默三例→Unauthenticated）、`TestAnchorG_AdoptMutualAuth`（错 token→席位 PermissionDenied＋trace 留痕 `refused`）。
5. **范围**——code.diff 内全部文件属票面范围内（go.mod/cmd/meta/internal/db/internal/gateway/internal/clock/scripts/selfcheck.sh＋proto 源与 protoc 生成码，系席位面 gRPC 的必要成分，§12 钦定接口）。未碰 docs/总设计、roles/、CEO 面/前端面/外部面、命令代跑、部署闸、自动票径、预算闸启用、角色卡注入、飞书。交付 commit 除代码外另含三件，逐一裁量如下，均判非范围蔓延：①`go.sum`——go.mod 锁文件，验证命令链机械必需；②`.gitignore`（9 行：`.gomodcache/ .gocache/ .gopath/ run/ *.db* *.sock /meta`）——全部为构建/运行产物，GOMODCACHE/GOCACHE 经环境注入 worktree 内（机器记录可 rc=0 的既定施工配方），不忽略则工作区永久脏；③`docs/经验库/` 施工署名三篇（gitignore-裸名吃目录、grpc-unix-dial-ENOENT-fastfail、selfcheck-grep-扫到-gomodcache）——业主令#E1「遇坑即写，写完即 commit；不写＝违规」（经验库 README 钦定，先例 63d692f）的强制义务履行，形制合规，且三篇内容分别与 `.gitignore` 的 `/meta` 锚根、helper_test.go 的有界重试链（`adoptOnce`/`waitSeatUp`）、selfcheck.sh 的 `GREPEXCL` 在 diff 内互证。另机械复核：`git diff 63d692f...227eaa1` 与 code.diff 之差异恰为上述五文件的纯排除块（153 行全为全量侧多出、反向零行）——网关所给 code.diff 无删改美化；关键 blob hash 与 diff 头 index 一致（selfcheck.sh＝`0ddea7c`、phase.go＝`f803dd9`、attach.go＝`80d1bae`）。
6. **R2/R3 纪律**——审计席亲自 grep worktree 全部自有 .go（cmd/internal/proto）：`time.(NewTicker|Ticker|Tick\(` 零命中、`time.Sleep` 零命中、`"encoding/json"` 零命中（rc=1 三连）；仓内 `.gomodcache/` 的数百条命中全为第三方依赖源码，属禁区#1 边界条款「第三方库内部不受限」，selfcheck 的 `--exclude-dir` 排除表与此一致。时钟径全走 `time.AfterFunc`/`time.After`（clock/alarm.go、attach.go `recvFirst`、helper_test 重试链 deadline 封顶——有界重试链，边界条款明文合法）。网关 if 逐处核过：token 比对、相位比对、退码/`errors.As`、布尔双源组合、`kill(pid,0)` 一次性核实、role_acl 清单匹配、超时/EOF——全为机械判（禁区「阈值/退码/键比对/清单比对/计数/验签」类），无语义裁决；卡死（`onStall`）只发事件不杀、非施工中相位死亡只发 `seat_dead` 事件，裁决权留智能体，符合 §3。

## 结论

四锚全绿、子项 a–h 逐一有具名机器测试、六质询全数核过、范围与交付形制合规、禁线零触碰。**判决：通过**。本票进入业主会话验收（阶段 0 手工径）。

备注两条供后续票参考（不影响本判决）：
1. selfcheck 的 grep 禁式存在 dot-import（`import . "time"` 后裸调 `Sleep`）理论绕过面——grep 闸固有局限；如需收紧，属后续票（改 selfcheck 语义须经审计）。
2. 票面锚 4 的「只读打开」措辞与实现测试的验证径（普通 Open）存在精确度差——建议下票锚文写「可打开且表数一致」或指定 `mode=ro` 打开。
