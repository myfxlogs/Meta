# 票 0013 判决：通过（审计席·claude 独立实例·第三审，2026-11-02）

- 审计 run：`b50dae7779272e2c`（shell=claude），触发＝audit_dead（事件 781——前审计 `ec61b1580883e722` 已写 `2-claude.md` 并 commit 6c15e20 但**无哨兵净退＝半交付不算数**；再前一审计 `ef381e89e0863469` 同因 1-devin.md a4dc410 无哨兵不算）。**本席不采前两审结论，全部证据自行重取**；三席独立复算结论一致。
- 交付物：支 `t/0013-feishu-chan`，交付 commit `f0ca226`（两笔：`bc4a8e3` 主施工＋`f0ca226` 出站订户同步注册修）。本席亲证：origin/t/0013-feishu-chan＝本地 HEAD（已推）✓；`git log --merges master..支` 零合并（不并 master）✓；交付树与 worktree 代码零漂移（`git diff f0ca226 HEAD -- internal cmd config proto scripts go.mod go.sum` 空）✓。
- 机器记录：施工 `/tmp/t0013.machine-record.txt`（16 行：14 ok＋2 no-test-files，零 FAIL）；**本席独立复跑** `/tmp/t0013-audit3-claude.txt`（16 行包集逐行同施工机录，仅耗时漂移）＋`/tmp/t0013-audit3-selfcheck.txt`＋直接退码 `DIRECT_SC_RC=0`；对抗锚突变复算于 scratch `/tmp/t0013-mut3` @ f0ca226（已 `git worktree remove`，交付树未动）。

## 逐锚对账（票面七锚，全绿——证据皆本席自取）

| 锚 | 机器/行号证据 | 判 |
|---|---|---|
| 1 build/vet/test/selfcheck | 本席于交付 worktree 复跑票面验证命令：build+vet 链式过＋`go test -count=1 ./...` 16 行零 FAIL（14 ok＋2 no-test）＋`bash scripts/selfcheck.sh` 直接退码 0（输出档空） | 绿 |
| 2 入站 | `handle`（internal/feishu/feishu.go:90-103）：91 白名单等值比较→非业主 92 拒答**不进泵**、业主 99-101 `Feed` 馈泵＋102 机械回执；空正文/非 text 静默（95-98）。网关 `FeishuInbound`（internal/gateway/feishu.go:18-24）：`AppendEvent(ctx, "", "feishu_inbound", …Provenance="feishu:"+openID)` 空串行＋emit。`TestFeishuInbound`（gateway/feishu_test.go:36-60）断言行形（ticket_id 空＋prov=feishu:ou_owner1）＋**真泵馈达**（候到 `ActiveRunForRole("CEO")` 且 `Provenance=="feishu:ou_owner1"`——非 system_event，§2 链戳经 ceo.go:169-175 透传坐实）；`TestHandle_Owner`（feishu_test.go:49-77）三态断言 | 绿 |
| 3 凭证 | `ParseCred`（internal/feishu/cred.go:17-53）proto blob 先试→KEY=VALUE 兜底（FEISHU_MANAGE_* 全名＋裸名）→四键缺一即错；`FeishuChannel`（gateway/feishu.go:29-39）坏凭证→`feishu_cred_bad` 行＋emit＋返 nil 不启；`feishuCred`（58-75）vault.key 自解不走 Secret RPC。`TestParseCred`（feishu_test.go:81-107）双格式＋3 坏 fixture；`TestFeishuCredBad`（gateway/feishu_test.go:64-111）三情形＋`_KeyValue`（114-133） | 绿 |
| 4 出站 | `Observe`（feishu.go:199-206）kind 白名单 `ceo_say`/`ceo_last_resort`→`sendTo` owner chat；败仅 Logf 不炸环（210-219）；`TestObserve`（feishu_test.go:111-128）伪 sender 捕文本＋四杂 kind 过手＋败发不炸 | 绿 |
| 5 重连有界 | `Run`（feishu.go:160-192）：`fails>=MaxRestart` 返（173-177）＋退避封 `BackoffCap`（187-190）＋ctx 取消即退（166-168/181-186）——NewTimer＋select 非 Sleep（R2 selfcheck 机证零命中）；`TestRun_BoundedReconnect`（feishu_test.go:132-153）calls==3＋<3s；`TestRun_CtxCancel`（159-174）退避睡 1h 中取消即退 | 绿 |
| 6 D-35 | `Close()` 进 `AdminClient` 接口（internal/caster/caster.go:28）＋`Once` defer（135）＋`cycle` defer（169）；真实现 `adminCaster.Close→conn.Close`（cmd/meta/main.go:832）＋`Dial` 每轮新 conn（795-801）；`TestConnClose`（caster_test.go:546-575）两 cycle→closes==2、Once→3；台账 D-35 open 行删、已收区落账（债务台账.md diff 亲证） | 绿 |
| 7 对抗锚 a/b | 本席 scratch @ f0ca226 突变复算（rtk tee 档 1793572789/1793572803）：**a** feishu.go:91 `!=`→`==` → `TestHandle_Owner FAIL: owner 消息未馈泵: []`；**b** gateway/feishu.go:31 早退禁用（`if !ok`→`if ok && false`）→ `TestFeishuCredBad FAIL: 凭证行缺仍出通道`——两突变皆红得其所 | 绿 |

## 范围核（f0ca226 十七文件逐一对票面范围内）

internal/feishu 三件（新包）／gateway/feishu.go+test（「一进程三件」必要接线）／gateway/ceo.go 8+/−3（prov 透传＝§2 链戳钦定）／cmd/meta（serve 挂 StartFeishu＋D-35 真件）／proto+pb.go（`FeishuApp` 钦定消息；pb.go diff 抽样＝纯 protoc-gen 形——struct/标准方法/rawDesc 噪声，无手改痕）／caster+test（D-35）／caster.textproto 五知行（feishu_cred_bad/feishu_inbound/ceo_say/ceo_gap/ceo_last_resort 全 notify，**无 dispatch CEO 行**——票面明写不写）／go.mod+go.sum（lark 钉版票面要求入 diff）／债务台账 D-35 销账／经验库坑文（令#E1）——**无票外顺手改**。

敏感件单核：`selfcheck.sh` R3 加 `--exclude-dir=feishu`＝设计-0007 §10 R3 逐字钦定（「JSON 止于 `internal/feishu`（lark API）与 `internal/adapter`」——本席重读设计原文亲证），不改则锚 1 必红：必要最小改动非运动场自改。产线同生：master config/gateway.textproto 已预种 `ceo_spawn_kinds`/`ceo_events` 含 `feishu_inbound`（本席于 master 亲证 60/65 行）——不在本 diff、正确地未重复，泵在产线非哑。

## 禁线/禁区核（机证）

#3 JSON 止 internal/feishu（selfcheck RC=0；diff 全仓 grep `"encoding/json"` 新增仅 feishu.go/feishu_test.go）✓；R2 零 Ticker/Sleep（diff 新增行 grep 零命中；Run 用 NewTimer＋select）✓；#19 diff 全伪 fixture（cli_a/ou_owner/oc_owner/"s"/"sk"，零真凭证值；env 读零）✓；#16 全 I/O 归网关进程 ✓；#7 新增 if 皆机械判（等值/kind switch/计数界/超时）✓；R4 飞书 WS vend 外通道（§10 钦定豁免）✓；feishu-notify.sh 未动 ✓；webhook 未做（范围外申报）✓。回执模板逐字＝mbusd v1（feishu-ingress/main.go:41-42 亲证同文）。lark SDK `v3.12.2` 发布 `2026-10-15T08:18:30Z`（`go list -m -json` 机证）＝今 17.6 天 ≥7 天 ✓（设计 §10 依赖申报＋§3.1 钦定件；mbusd 现参同版）。

## 质量三问（令#E5）

最优解：Feed/Send/StartWS 三注入缝最小测试面；复用 0011 泵口（AppendEvent+emit）、既有 `subscribe()` 事件总线（f0ca226 修的正是同步注册——`TestStartFeishu_SyncSubscribe` 返修锚在）、`SecretBlob`/`DecryptSecret`/`LoadOrCreateVaultKey` 零 db 面改动；无逆向依赖（feishu 不 import gateway，gateway 单向注入）。第一性：fail loud 落装配口一处早退（gateway/feishu.go:29-39）非满处补丁；provenance 取事件行戳透传非旁路造链；出站订进程内事件口不轮询 DB。洁净：diff 新增行 grep TODO/FIXME/XXX/HACK/fmt.Print/println 零命中；无死码（handle/dispatcher/Run/Observe 皆经 StartFeishu 或测试触达）。说不出更优替代形制——无质量驳回项。

## 债项节（暗债对账：diff 扫描 vs 摘要「下一步」——瞒报零起）

| 债 | 内容 | 摘要申报？ |
|---|---|---|
| 部署步 | `secret.feishu.app` 重打包（`meta secret put`，票面注明运维不动票；KEY=VALUE 旧文直存亦可解） | 已申报 |
| 范围外 | webhook/卡片/心跳未做（票面范围外） | 已申报 |
| 观察1 | `Run` 上轮存活>5min 归零分支无直测（测试注释自承无法真睡 5min；锚 5 钦定的次数界＋退避上界已直测坐实） | 非债级 |
| 观察2 | `feishuOutbound` 串行消费 `sendTo` 20s 队头阻塞＋订户缓冲满丢——与既有订户口同语义 | 非债级 |
| 瑕疵 | feishu_test.go:155 注释头残留旧名 `TestRun_LongLivedResets`（函数实名 `TestRun_CtxCancel`）——建议随下票顺手清 | 瑕疵级 |

## 沉淀意见（令#E8，纯意见不构成否决依据）

接替席「前任似已交付但席死」打法（亲验机器记录＋锚覆盖，按本壳失明谱选哨兵径补帧）**可沉淀**——与经验库《哨兵三壳失明谱》《订阅在goroutine里注册》互证；`/proc/<pid>/fd/1` 直写属 devin 壳特形，沉淀须注适用面。前两审的 scratch 突变复算法本席沿用并独立重跑——判真手段值得沿用。

## 判决

**通过**。七锚全绿（锚 1 本席独立复跑、锚 7 两突变本席独立复算皆红得其所）；形制齐（支名/`t0013:` 提交式/已推 origin/不并 master/摘要五节实录于 run/logs/8f9803f748caca36.log）；范围净无蔓延；暗债零瞒报；设计-0007 §1.6/§2/§5/§10 与总设计 E12/§8、禁区#3/#7/#16/#19、R2/R4 逐条对上。

META_VERDICT:pass:docs/实现/票-0013-判决/3-claude.md
