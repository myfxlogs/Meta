# 票 0013 判决：通过（审计席·devin 独立实例，2026-11-02）

- 审计 run：`ef381e89e0863469`（shell=devin），触发＝deliver（事件 641，`f0ca226 devin exit=0`）
- 施工 run：`8f9803f748caca36`（shell=devin，第六任接替席——前五任死序：两席哨兵被壳 JSON 包裹失明、一席 429 限额、两席净退无哨兵；补发接力链见事件 362–641），worktree `~/meta-worktrees/t0013-a`
- 交付物：支 `t/0013-feishu-chan` @ `f0ca226`（`git fetch` 核 origin=f0ca226=本地 HEAD 已推；master=`80b6d7e` 未并，`git log --merges master..HEAD` 零合并），两 commit：`bc4a8e3` 主施工＋`f0ca226` 出站订户同步注册修复（前任末班），17 文件 +996/−39
- 判决史：空——本席首判
- 机器记录：施工 `/tmp/t0013.machine-record.txt`（mtime 06:17，晚于 commit 01:53＝接替席对交付树复跑，与 f0ca226 同树）；**本席在交付 worktree 原命令复算** `/tmp/t0013-audit-recompute.txt` 作旁证

## 逐锚对账（票面七锚，全绿）

| 锚 | 机器证据 | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 施工机录 16 行全 `ok`（build/vet 链式静默过——`go test` 得跑即证前段绿）；本席复算：`go test -count=1 ./...` 15 包全 `ok`（包集与施工机录逐一相同，仅耗时漂移）＋`bash scripts/selfcheck.sh` 退码 `SELFCHECK_RC=0` | 绿 |
| 2 入站：owner→行＋泵馈达；非 owner→拒答 | `handle`（internal/feishu/feishu.go:90-103）：91 行白名单等值比较→Feed 馈泵（99-101）＋机械回执 receiptIn（102）；非 owner→receiptReject（92）不进泵。`TestHandle_Owner`（feishu_test.go:42-79）三态断言全在。网关级 `FeishuInbound`（gateway/feishu.go:19-24）：`AppendEvent("", "feishu_inbound", …Provenance="feishu:"+openID)`＋emit；`TestFeishuInbound`（gateway/feishu_test.go:31-58）实证事件行 ticket_id 空串、戳 `feishu:ou_owner1`＋真泵馈达（候到 `ActiveRunForRole("CEO")` run 且 `Provenance=="feishu:ou_owner1"`） | 绿 |
| 3 凭证：双格式 fixture＋坏→cred_bad＋不启 | `ParseCred`（cred.go:17-53）proto blob 先试、KEY=VALUE 兜底、四键缺一即错；`TestParseCred`（feishu_test.go:82-107）双格式＋3 坏 fixture。`FeishuChannel` 早退（gateway/feishu.go:30-38）＋`feishuCred` vault.key 自解不走 Secret RPC（58-72）；`TestFeishuCredBad`（gateway/feishu_test.go:60-111）三情形（行缺/垃圾密文→nil＋cred_bad 行；好 blob→通道出）＋`TestFeishuCredBad_KeyValue`（113-133） | 绿 |
| 4 出站：say 消费伪 sender 捕文本 | `Observe`（feishu.go:199-206）kind 白名单 `ceo_say`/`ceo_last_resort`→`sendTo` Cred.ChatId（210-219，败仅 Logf 不炸环）；`TestObserve`（feishu_test.go:111-128）：两 kind 各一发至 `oc_owner`、四杂 kind 过手、failSend 不炸 | 绿 |
| 5 重连：有界断言（上界＋次数界） | `Run`（feishu.go:160-192）：`fails>=MaxRestart→return`（173-177）＋退避封 `BackoffCap`（188-190）＋ctx 取消即退（166-168/181-185）——有界重试链，R2 边界条款合法非周期；`TestRun_BoundedReconnect`（feishu_test.go:132-153）：MaxRestart=3→calls==3、cap=4ms→全程<3s；`TestRun_CtxCancel`（159-174）：退避睡 1h 中 ctx 取消即退 | 绿 |
| 6 D-35：Close 调用断言 | `AdminClient` 接口增 `Close()`（caster.go:28）；`cycle` defer Close（169）、`Once` defer Close（135）；`adminCaster.Close→conn.Close`（cmd/meta/main.go:833，Dial 每轮新 conn——795-801）；`TestConnClose`（caster_test.go:546-575）：两 cycle→closes==2、Once→3 | 绿 |
| 7 对抗锚 ≥2（本席 scratch `/tmp/t0013-mut` @ f0ca226 突变复算，交付支未动） | a. feishu.go:91 `!=`→`==` → `go test -run TestHandle_Owner` FAIL：`owner 消息未馈泵`——等值比较翻向红得其所；b. gateway/feishu.go:30-38 删 `cred_bad` 早退 → `go test -run TestFeishuCredBad` FAIL：`凭证行缺仍出通道`——坏凭证通道照启被锚咬住。scratch 已 `git worktree remove` | 绿 |

## 交付形制与范围

- 支名 `t/0013-feishu-chan` ✓；两 commit 均 `t0013: <为什么>` 形 ✓；已推 origin ✓；不并 master ✓。
- 交付摘要五节齐（`run/logs/8f9803f748caca36.log` 尾部实录）：新增/修改面／根因（补发票身世）／验证（复跑全绿）／下一步（两条全申报）／可沉淀候选；收口两步门复核过——worktree `git status` 净、经验库坑文 `2026-11-02-死ctx测装配-凭证DB读同死.md` 在支、判决史空无需入。
- 依赖 0011 已并：泵馈径所需的 `ceo_spawn_kinds: "feishu_inbound"`/`ceo_events` 为 master 预种（gateway.textproto:60,65，不在本票 diff——0011 接线位，非漏账非蔓延）；`spawnCEO` provenance 透传（ceo.go:158,169-175）落设计 §2「run 盖同戳」。
- 范围逐文件核：internal/feishu 三件、gateway/feishu.go+test（「通道归网关一进程三件」必要接线）、gateway/ceo.go（prov 透传）、cmd/meta（serve 挂接＋D-35 真实实现 Close）、proto+pb.go（FeishuApp 钦定）、caster+test（D-35）、caster.textproto（5 notify 行，无 dispatch CEO 行——票面明写）、selfcheck.sh（R3 豁免，设计-0007 §10 R3「JSON 止于 internal/feishu」钦定——不改则锚 1 必红，必要最小改动）、go.mod/go.sum（lark 钉版票面要求 diff 可见）、债务台账 D-35 销账＋经验库坑文（簿记/令#E1）。无票外顺手改。
- 禁线：encoding/json 仅 internal/feishu（selfcheck R3 零命中其他包）✓#3；无 Ticker/Sleep/心跳，退避有界 ✓#1#2；凭证只走 secret 表 vault.key 自解、diff 内零真实凭证值（测试全伪 fixture）✓#19；通道全 I/O 在网关进程 ✓#16；新增 if 皆机械判（等值比较/kind switch/计数上界）✓#7；飞书 WS 属 vend 外通道（设计 §10 R4 钦定豁免）✓#4。
- lark SDK 钉版：`oapi-sdk-go/v3 v3.12.2` 入 go.mod 主 require 块；模块缓存 `v3.12.2.info` 亲证 `Time:2026-10-15T08:18:30Z`＝18 天 ≥7 天 ✓（总设计 §3.1 钦定件）。
- pb.go 为再生件非手改：`msgTypes` 66→67、类型索引顺移（SandboxSpec 61→62）、rawDesc 字节噪声——抽样无手改痕。

## 设计对账（设计-0007 §1.6/§2/§3/§5 ＋ 总设计 §8/E12 ＋ mbusd 借形件）

设计-0007 §1.6 逐项对上：一进程三件（WS 入站＋出站＋凭证同驻网关共 lark client）；`secret.feishu.app` ACL 空＋vault.key 自解不走 Secret RPC（feishuCred 与 board.go token 闸同形）；`im.message.receive_v1`→open_id==owner→`feishu_inbound` 空串行→CEO 泵；非业主礼貌回绝（模板逐字钦定件落 receiptReject）；机械回执「收到，这就去办」不等 CEO（receiptIn 逐字）；`ceo_say`→owner chat；WS 重连＝SDK 内建＋外层指数退避。§2 时序链 prov=feishu:openid 盖 run 戳对上（TestFeishuInbound 实证）。§5 票面锚与本票七锚一一对应。mbusd feishu-ingress 借形（白名单/退避/静默忽略非 text）确认只借形——env 读文件段换 secret 自解，码不相似。

## 质量三问（令#E5）

- **最优解**：Feed/Send/StartWS 三注入缝＝最小测试面；馈口复用 0011 既有泵接口（`g.FeishuInbound` 直落事件行＋emit，不另造通道）；出站复用 `subscribe()` 事件总线（同步注册在 New/起动前——与 ceoPump「订阅先行」同形，f0ca226 修的正是此竞）；`feishuCred` 复用既有 `SecretBlob`/`DecryptSecret`/`LoadOrCreateVaultKey` 零 db 改动；无逆向依赖（feishu 不识 gateway，gateway 单向注入）。
- **第一性**：fail loud＝装配口一处早退非满处补丁；provenance 取触发事件行戳透传（ceo.go 8+/−3 最小改）而非旁路造链；出站订进程内事件口不轮询 DB；lark JSON 在 dispatcher 回调即抽字段转原生串，止于包界。
- **洁净**：diff 扫 TODO/FIXME/XXX/HACK/绕过/临时/fmt.Print/println 零命中；gofmt -l 全净；无死码（handle/dispatcher/Run/Observe 全经 StartFeishu 或测试触达）；测试名与注释仅一处瑕疵见债项。

## 债项节（暗债对账：diff 扫描 vs 摘要「下一步」）

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| 部署步 | `secret.feishu.app` 重打包（`meta secret put`，票面注明运维不动票） | 已申报 |
| 观察1 | `Run` 的「上轮存活>5min 归零连败」分支无直测（TestRun_CtxCancel 注释自承无法真睡 5min）——锚 5 钦定的次数界＋退避上界已坐实，reset 属设计钦定退避语义 | 非债级 |
| 观察2 | `feishuOutbound` 串行消费：`sendTo` 20s 超时内队头阻塞＋订户口 256 缓冲满丢 `ceo_say`/`ceo_last_resort`（`emit` 非阻塞 default 丢）——与既有订户口同语义（0012 判决 D-0012d 同类已立） | 非债级 |
| 观察3 | feishu_test.go:155 注释头残留旧名 `TestRun_LongLivedResets`（函数实名 `TestRun_CtxCancel`） | 瑕疵级 |

**瞒报零起**：diff 内未申报 TODO/绕过/半成品零命中；摘要「下一步」两条（部署步重打包＋webhook/卡片/心跳范围外未做）与本席 diff 扫描逐条对上。

## 沉淀意见（令#E8，纯意见不构成否决依据）

接替席「前任似已交付但席死」打法——亲跑票面验证命令复验机器记录＋锚覆盖度，再按本壳失明谱选哨兵径补交付帧（本席 devin 壳用 `/proc/<pid>/fd/1` 直写独占行）——**可沉淀**：不重写不猜做，与经验库《哨兵三壳失明谱》《订阅在goroutine里注册》互证；但 fd 直写属壳环境特形，沉淀文若录须注明适用面。

## 判决

**通过**。七锚全绿，两对抗锚经本席独立 scratch 突变复算证真（红得其所）；形制齐、范围净、暗债零瞒报；成品与设计-0007 §1.6/§2/§5 及禁区逐条对上。

META_VERDICT:pass:docs/实现/票-0013-判决/1-devin.md
