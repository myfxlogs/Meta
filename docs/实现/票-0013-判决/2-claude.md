# 票 0013 判决：通过（审计席·claude 独立实例，2026-11-02）

- 审计 run：`ec61b1580883e722`（shell=claude），触发＝deliver（事件 641，`f0ca226 devin exit=0`）
- 前审计 `ef381e89e0863469`（devin）已写 `1-devin.md` 并 commit（a4dc410）但**无哨兵净退＝半交付不算数**（事件 724 audit_dead）——本席不采其结论，全部证据自行重取；两席独立复算结论一致
- 交付物：支 `t/0013-feishu-chan`，交付 commit `f0ca226`（origin 同 tip 已推、`git log --merges master..支` 零合并＝未并 master ✓）；f0ca226 两笔：`bc4a8e3` 主施工＋`f0ca226` 接替席修出站订户竞态真坑
- 机器记录：施工 `/tmp/t0013.machine-record.txt`（mtime 06:17，晚于 f0ca226＝交付态复跑）；**本席独立复算** `/tmp/t0013-audit2-claude.txt`＋`/tmp/t0013-audit2-selfcheck.txt`（交付 worktree @ a4dc410＝f0ca226 同码树）；对抗锚突变复算于 scratch `/tmp/t0013-mut2`（已拆）

## 逐锚对账（票面七锚）

| 锚 | 机器证据（本席自取） | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 本席复跑：build/vet 链式静默过＋`go test -count=1 ./...` 15 包全 `ok`（包集与施工机录 16 行逐包相同）＋`selfcheck.sh` 退码 0（`SELFCHECK_RC=0` 入本席记录档） | 绿 |
| 2 入站 | `handle`（internal/feishu/feishu.go:90-103）：91 白名单等值→非业主 92 拒答不进泵；业主 99-101 `Feed` 馈泵＋102 机械回执。网关 `FeishuInbound`（gateway/feishu.go:18-24）落 `feishu_inbound` 空串行＋prov=`feishu:<open_id>`＋emit 入泵（ceoPump 消费径＝0011 正口，ceo.go:98-125）。`TestFeishuInbound`（gateway/feishu_test.go:36-60）实证行形＋**真泵馈达**（候到 CEO run `Provenance=="feishu:ou_owner1"`）；`TestHandle_Owner`（feishu_test.go:49-77）三态断言 | 绿 |
| 3 凭证 | `ParseCred`（cred.go:17-53）proto blob 先试→KEY=VALUE 兜底→四键缺一即错；`TestParseCred` 双格式＋3 坏 fixture；`FeishuChannel`（gateway/feishu.go:29-39）坏凭证→`feishu_cred_bad` 行＋emit＋返 nil 不启；`TestFeishuCredBad` 三情形＋`_KeyValue` 网关径侧证。vault.key 自解不走 Secret RPC（58-75） | 绿 |
| 4 出站 | `Observe`（feishu.go:199-206）kind 白名单 `ceo_say`/`ceo_last_resort`→`sendTo` owner chat；败仅 Logf 不炸环（210-219）；`TestObserve`（feishu_test.go:111-128）伪 sender 捕文本＋败发不炸 | 绿 |
| 5 重连有界 | `Run`（feishu.go:160-192）：`fails>=MaxRestart` 返（173-177）＋退避封 `BackoffCap`（187-190）＋ctx 取消即退（166-168/181-186）；`TestRun_BoundedReconnect` calls==3＋<3s；`TestRun_CtxCancel` 退避睡 1h 中取消即退 | 绿 |
| 6 D-35 | `AdminClient.Close()` 进接口（caster.go:28）＋`cycle`/`Once` defer Close（135/169）＋真实现 `adminCaster.Close→conn.Close`（cmd/meta/main.go:832，Dial 每轮新 conn）；`TestConnClose`（caster_test.go:551-575）两 cycle→closes==2、Once→3；台账 D-35 open 行删、已收区落账 | 绿 |
| 7 对抗锚 a/b | 本席 scratch @ f0ca226 突变复算（交付支未动，rtk tee 档 1793572476/1793572482＋stdout）：**a** feishu.go:91 `!=`→`==` → `TestHandle_Owner FAIL: owner 消息未馈泵`；**b** gateway/feishu.go cred_bad 早退块禁用 → `TestFeishuCredBad FAIL: 凭证行缺仍出通道`——两突变皆红得其所 | 绿 |

## 范围核（f0ca226 十七文件逐一对票面范围内）

internal/feishu 三件（新包本体）／gateway/feishu.go+test（「通道归网关一进程三件」必要接线）／gateway/ceo.go 8 行（prov 透传，§2 链戳）／cmd/meta serve 挂接＋D-35 真件／proto+pb.go（`FeishuApp` 钦定消息，测试 proto.Marshal 往返证再生件可用）／caster+test（D-35）／caster.textproto 五知行（无 dispatch CEO 行——票面明写）／go.mod+go.sum（lark 钉版票面要求）／债务台账 D-35 销账／经验库坑文（令#E1）——**无票外顺手改**。敏感件单核：`selfcheck.sh` R3 豁免＝设计-0007 §10 R3「JSON 止于 internal/feishu（lark API）与 internal/adapter」逐字钦定，不改则锚 1 必红——必要最小改动非运动场自改。**产线同生配置**：master gateway.textproto:60/65 已预种 `ceo_spawn_kinds`/`ceo_events` 含 `feishu_inbound`（0011 接线位，不在本 diff、正确地未重复）——泵在产线非哑。

## 禁线/禁区核

#3 JSON 止 internal/feishu（selfcheck R3 零命中他包，RC=0 机证）；#16 全 I/O 归网关进程；#19 diff 全伪 fixture（cli_a/ou_owner 等）零真凭证值；R2 零 Ticker/Sleep（selfcheck 机证；Run 用 NewTimer＋select 有界等待）；#7 新增 if 皆机械判（等值/kind switch/计数界）；R4 飞书 WS vend 外通道 §10 钦定豁免；feishu-notify.sh 未动；webhook 未做（范围外）。模板逐字核：`receiptIn`/`receiptReject` 与 mbusd v1（feishu-ingress/main.go:41-42）逐字同——钦定件落位。lark SDK `v3.12.2` 发布 `2026-10-15T08:18:30Z`（go list -m -json 机证）＝18 天 ≥7 天 ✓。

## 质量三问（令#E5）

最优解：Feed/Send/StartWS 三注入缝最小测试面；复用 0011 泵口/既有 subscribe/SecretBlob/DecryptSecret 零 db 改动；无逆向依赖（feishu 不 import gateway）。第一性：fail loud 落装配口一处；provenance 取事件行戳透传（ceo.go 最小改）非旁路造链；出站订进程内事件口不轮询。洁净：diff 扫 TODO/FIXME/HACK 零命中；gofmt -l 全净；无死码。说不出更优替代形制——无质量驳回项。

## 债项节（暗债对账：diff 扫描 vs 摘要「下一步」）

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| 部署步 | `secret.feishu.app` 重打包（`meta secret put`，票面注明运维不动票） | 已申报 |
| 范围外 | webhook/卡片/心跳未做 | 已申报 |
| 观察1 | `Run` 上轮存活>5min 归零分支无直测（测试注释自承；锚 5 钦定的次数界＋上界已直测坐实） | 非债级 |
| 观察2 | 出站串行消费 20s 队头阻塞＋订户 256 缓冲满丢——与既有订户口同语义 | 非债级 |
| 瑕疵 | feishu_test.go:155 注释头残留旧名 `TestRun_LongLivedResets`（实名 `TestRun_CtxCancel`） | 瑕疵级 |

**瞒报零起**：diff 无未申报 TODO/绕过/半成品；摘要「下一步」与扫描逐条对上。

## 沉淀意见（令#E8，纯意见）

接替席「前任似已交付但席死」打法（亲验机器记录＋锚覆盖，按本壳失明谱选哨兵径补帧）**可沉淀**——与经验库《哨兵三壳失明谱》《订阅在goroutine里注册》互证；/proc fd 直写属 devin 壳特形，沉淀须注适用面。前审计 1-devin.md 之突变复算法（scratch worktree 突变＋锚红断言）本席沿用并独立重跑——判真手段本身值得沿用。

## 判决

**通过**。七锚全绿（锚 1 本席独立复跑、锚 7 两突变本席独立复算红得其所）；范围净无蔓延；形制齐（支名/提交式/已推/不并/摘要五节/收口两步门）；暗债零瞒报；设计-0007 §1.6/§2/§5/§10 与总设计 E12 逐条对上。

META_VERDICT:pass:docs/实现/票-0013-判决/2-claude.md
