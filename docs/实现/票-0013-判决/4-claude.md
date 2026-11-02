# 票 0013 判决：通过（审计席·claude 独立实例·第四审，2026-11-02）

- 审计 run：`e484b6d8855fc7ec`（shell=claude，`/proc/$PPID` 链实证本席进程 3372612），触发＝audit_dead（事件 810——第三审 `b50dae7779272e2c` 已写 3-claude.md 并 commit 6bc33a3 但**无哨兵净退＝半交付不算数**；再前两审同因，见事件 724/781）。**本席不采前三审结论，全部证据自行重取**；四席独立复算结论一致。
- 交付物：支 `t/0013-feishu-chan`，交付 commit `f0ca226`（两笔：`bc4a8e3` 主施工＋`f0ca226` 出站订户同步注册修——修的正是经验库《订阅在goroutine里注册》真坑，返修锚 `TestStartFeishu_SyncSubscribe` 在）。本席亲证三形：origin/t/0013-feishu-chan＝本地 HEAD＝6bc33a3（fetch 后 rev-parse 同值＝已推）；`git diff f0ca226 HEAD -- internal cmd config proto scripts go.mod go.sum` **空**（代码零漂移，HEAD 多出的四件全 docs）；`git log --merges master..支` 零合并（不并 master）。
- 机器记录：施工 `/tmp/t0013.machine-record.txt`（14 ok＋2 no-test-files，零 FAIL）；**本席独立复跑** `/tmp/t0013-audit4-claude.txt`（链式 TEST_RC=0、`grep -c '^ok'`＝14、`grep -ci FAIL`＝0——包集与施工机录逐包同）＋`selfcheck.sh` 直接退码 **SC_RC=0**；对抗锚突变复算于 scratch `/tmp/t0013-mut4` @ f0ca226（已 `git worktree remove --force`，交付树未动）。

## 逐锚对账（票面七锚，全绿——证据皆本席自取）

| 锚 | 机器/行号证据 | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 本席于交付 worktree 复跑票面验证命令：`go build && go vet && go test -count=1 ./...` 链 TEST_RC=0（14 包 ok＋2 no-test，零 FAIL）＋`bash scripts/selfcheck.sh` SC_RC=0 | 绿 |
| 2 入站 | `handle`（internal/feishu/feishu.go:90-103，本席全文亲读）：91 白名单**等值比较** `senderOpenID != Cred.OwnerOpenId`→92 拒答 return **不进泵**；95-98 空正文/非 text 静默；99-101 `Feed` 馈泵＋102 机械回执 `receiptIn`（28 行「收到，这就去办——办完回你」逐字）。网关 `FeishuInbound`（internal/gateway/feishu.go:18-24）：`AppendEvent(ctx, "", "feishu_inbound", …Provenance="feishu:"+openID)` **ticket_id 空串行**＋emit。测试亲 grep 在位：`TestHandle_Owner`（feishu_test.go:49）／`TestFeishuInbound`（gateway/feishu_test.go:36，断言泵馈达 CEO run 带 `feishu:ou_owner1` 戳——前三审引证，行号与本席亲读吻合） | 绿 |
| 3 凭证 | `ParseCred`（cred.go:16-51 亲读）：proto blob 先试（18-20）→KEY=VALUE 兜底（FEISHU_MANAGE_* 全名＋裸名，21-46）→四键缺一即错（47-49）；`FeishuChannel`（gateway/feishu.go:29-39）`!ok`→`feishu_cred_bad` 事件行＋emit＋**返 nil 通道不启**（fail loud）；`feishuCred`（58-75）vaultKey→SecretBlob→DecryptSecret→ParseCred **不走 Secret RPC**。测试在位：`TestParseCred`（81）／`TestFeishuCredBad`（64）／`_KeyValue`（114） | 绿 |
| 4 出站 | `Observe`（feishu.go:199-206）kind 白名单 `ceo_say`/`ceo_last_resort`→`sendTo(Cred.ChatId)`；`sendTo`（210-219）败仅 Logf **不炸环**＋20s 超时＋chatID 空回落 owner chat。`TestObserve`（feishu_test.go:111） | 绿 |
| 5 重连有界 | `Run`（feishu.go:160-192 亲读）：`fails>=MaxRestart`→return（173-177 **次数界**）＋`backoff>BackoffCap` 封顶（187-190 **上界**）＋ctx 取消即退（166-168/181-186）；`time.NewTimer`＋select（180-186）**非 Sleep**。`TestRun_BoundedReconnect`（132）calls==3＋<3s；`TestRun_CtxCancel`（159）退避睡 1h 中取消即退 | 绿 |
| 6 D-35 | diff 亲读：`Close() error` 进 `AdminClient` 接口（caster.go +6 段）＋`Once` defer（135）＋`cycle` defer（169）＋真实现 `adminCaster.Close→conn.Close`（cmd/meta/main.go +11 段）；`TestConnClose` 伪 conn 计数（caster_test.go:52 `closes`、111 `f.closes++`、547/560/569 断言）。台账亲证：D-35 open 行删＋已收区落账「票 0013（AdminClient.Close 进接口＋cycle/Once defer 关＋伪 conn 计数锚）」 | 绿 |
| 7 对抗锚 a/b | 本席 scratch @ f0ca226 独立突变复算（rtk tee 档 1793573058/1793573066）：**a** feishu.go:91 `!=`→`==` → `[FAIL] TestHandle_Owner：owner 消息未馈泵: []`——非业主逻辑翻向，锚咬住；**b** gateway/feishu.go:31 早退禁用（`if !ok`→`if ok && false`，保持编译）→ `[FAIL] TestFeishuCredBad：凭证行缺仍出通道`——坏凭证照启被锚咬住。**两突变皆红得其所** | 绿 |

## 范围核（diff 20 文件，施工面 17 件逐一对票面范围内）

internal/feishu 三件（新包）／internal/gateway/feishu.go+test（「一进程三件」必要接线）／gateway/ceo.go（prov 透传＝§2 链戳）／cmd/meta（serve 挂 StartFeishu＋D-35 真件）／proto+pb.go（`FeishuApp` 钦定消息；pb.go 141 行 diff 抽样＝protoc 再生形）／caster+test（D-35）／caster.textproto 五知行／go.mod+go.sum（lark 钉版票面要求入 diff）／债务台账 D-35 销账／经验库坑文（令#E1）——多出的三件判决档系审计产物，**施工无票外顺手改**。

敏感件亲读：`selfcheck.sh` R3 加 `--exclude-dir=feishu`＝设计-0007 §10 R3「JSON 止于 internal/feishu（lark API）与 internal/adapter」逐字钦定（diff 注释自引条款），不改则锚 1 必红——必要最小改动非运动场自改。产线同生：master config/gateway.textproto 亲证 `ceo_spawn_kinds`/`ceo_events` 含 `feishu_inbound`（0011 预种接线位，不在本 diff、正确地未重复）——泵在产线非哑。caster.textproto 五行全 notify、**零 dispatch CEO 行**（票面明写不写）。

## 禁线/禁区核（diff 新增行 grep 机证）

- #3：`encoding/json` 新增仅 internal/feishu 一处（grep 命中 1 行）；R3 豁免亲证 ✓
- R2：`time.Sleep|Ticker` 新增行 grep **零命中**（Run 用 NewTimer＋select 有界等待）✓
- #19：测试全伪 fixture（cli_a/ou_owner/oc_owner/sk），diff 内零真凭证值、零 env 读 ✓
- #16：全 I/O 归网关一进程三件（serve 挂 StartFeishu 亲读）✓；#7 新增 if 皆机械判（等值/kind switch/计数界）✓；R4 飞书 WS vend 外通道钦定豁免 ✓；feishu-notify.sh 未动 ✓；webhook 未做（范围外申报）✓
- 暗债扫：TODO/FIXME/XXX/HACK/绕过/临时/fmt.Print/println 新增行 grep **零命中** ✓
- lark 钉版：go.mod 主 require `github.com/larksuite/oapi-sdk-go/v3 v3.12.2`；`go list -m -json` 机证 `Time: 2026-10-15T08:18:30Z`＝今 18 天 ≥7 天（总设计 §3.1 钦定件）✓

## 质量三问（令#E5）

最优解：Feed/Send/StartWS 三注入缝＝最小测试面；复用 0011 泵口（AppendEvent+emit）、既有 `subscribe()`（f0ca226 修的正是同步注册防首拍丢窗）、`SecretBlob`/`DecryptSecret`/`LoadOrCreateVaultKey` 零 db 面改动；无逆向依赖（feishu 不 import gateway）。第一性：fail loud 落装配口一处早退非满处补丁；provenance 取事件行戳透传非旁路造链；出站订进程内事件口不轮询 DB。洁净：暗债 grep 零命中（上节机证）；无死码（handle/dispatcher/Run/Observe 皆经 StartFeishu 或测试触达）。说不出更优替代形制——无质量驳回项。

## 债项节（暗债对账：diff 扫描 vs 摘要「下一步」——瞒报零起）

摘要五节实录于 `run/logs/8f9803f748caca36.log` 尾部（本席亲读）；「下一步」两条与本席 diff 扫描逐条对上。

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| 部署步 | `secret.feishu.app` 重打包（`meta secret put`，票面注明运维不动票；KEY=VALUE 旧文直存亦可解——双格式兼容） | 已申报 |
| 范围外 | webhook/卡片/心跳未做（票面范围外） | 已申报 |
| 观察1 | `Run` 上轮存活>5min 归零分支（feishu.go:169-171）无直测——测试注释自承无法真睡 5min；锚 5 钦定的次数界＋退避上界已直测坐实 | 非债级 |
| 观察2 | `feishuOutbound` 串行消费 `sendTo` 20s 队头阻塞＋订户缓冲满丢——与既有订户口同语义 | 非债级 |
| 瑕疵 | feishu_test.go:155 注释头残留旧名 `TestRun_LongLivedResets`（本席 sed 亲证；函数实名 `TestRun_CtxCancel`）——建议随下票顺手清 | 瑕疵级 |

## 四审身世与哨兵径（本席与前三审之别）

前三审（1-devin/2-claude/3-claude）判决实质均已完成且结论一致（通过），死因全在**claude 壳哨兵被 stream-json JSON 包裹**（经验库《哨兵三壳失明谱》），非判决质量问题。本席按同坑文与 cd8fcb9 先例走双保险：正文末行哨兵＋`/proc/3372612/fd/1`（本席席位 stdout 扫描管，PPID 链实证）直写独占行。

## 沉淀意见（令#E8，纯意见不构成否决依据）

接替审计席「前三审半交付」打法——**不重算＝渎职，全重算＝浪费**的折中形＝锚 1/7 全量自跑＋锚 2-6 行号亲读抽验＋禁线/暗债 grep 自跑（本席形）——**可沉淀**：与《哨兵三壳失明谱》互补，多审接力时后审对前审已证锚做「抽查级亲证＋全量机证」即可守「只信 diff 与机器记录」而不空转。

## 判决

**通过**。七锚全绿（锚 1 本席独立复跑 TEST_RC=0/SC_RC=0、锚 7 两突变独立复算皆红得其所）；形制齐（支名/`t0013:` 提交式/已推 origin/不并 master/摘要五节实录）；范围净无蔓延；暗债零瞒报；设计-0007 §1.6/§2/§5/§10 与总设计 §8/E12、禁区#3/#7/#16/#19、R2/R4 逐条对上。

META_VERDICT:pass:docs/实现/票-0013-判决/4-claude.md
