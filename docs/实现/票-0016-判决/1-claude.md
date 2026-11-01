# 票 0016 判决（1-claude · 首判 · 通过）

审计 run `cd4fe8c323a2ff94`（claude/glm-5.3）。交付链：`accab10`（设计档＋票 0017 票面）→ `19e0c90`（经验库哨兵三壳失明谱）→ `6631ae2`（本票 fill_spec，deliver ref＝事件 550）。**只信 diff 与机器记录**——以下逐锚对账，每条指到机器输出。

## 逐锚对账（票面第 6 栏五锚）

**锚 1（设计档在＋三节名）——绿**
- 文件在仓：`docs/实现/设计-0016-D42哨兵扫描移层.md`（accab10 引入，151 行）。
- 票面验证命令复跑：`grep -c "双机制\|去重\|sentinel.log" …` → **11**，exit=0。
- 节名机器证据（`grep -n "^##"`）：§2「总体方案：双机制＋双发契约」、§3.4「去重与优先序」、§1 含根因确诊表（8 行事实表，逐条指到 `seat.go:264`/`claude.go:19`/`run/logs/*` 实证）；另含落点明细（§3.1–3.5）/风险回退（§5）/测试母本（§6）——票面范围内要求的六件全在。

**锚 2（票 0017 在仓＋ParseTicketMD 解析过）——绿**
- 文件在仓：`docs/实现/票-0017-D42哨兵修复.md`（accab10 引入，127 行/6964B，八栏＋附全量，含对抗锚五条）。
- 解析机器证据链：①发送侧 `internal/seat/seat.go:533-534`——create_ticket 前 `inject.ParseTicketMD(b)`→`proto.Marshal(ts)`；②DB `ticket` 表 0017 行 `length(spec)=4356` 非空且为 proto blob（hex 首 `0A` field-1 结构、中文 goal 内容可辨）——空解析不可能产 4356B blob；③dispatch（事件 540）成功＝过 `injectPrompt` 的 `inject.CheckSpec` 闸（`internal/gateway/inject.go:30`，缺栏则派发失败）——八栏零缺。

**锚 3（DB 票 0017 存在且 spec 非空）——绿（度量口径记档）**
- `sqlite3 run/meta.db "SELECT id,length(spec),phase …"`→ `0017 | 4356 | 审计中`。存在 ✅、非空 ✅。
- 口径注记：票面括号「6723B」与 DB blob（4356B，proto 度量）及仓内 md（6964B，accab10 版）均不吻合——立票（事件 538）时读的工作区文件版本已不可考；锚的可判主句「存在且非空」机器实证绿，括号数字不构成可判断言，不据此驳。

**锚 4（0017 派发真）——绿**
- `meta events --ticket 0017`：事件 540 `dispatch 设计中→已派 gateway`、事件 541 `claim 已派→施工中 d10ca1b4c6565b7b`。派与接均在案；其后 547 `deliver 5e9ce28`（子票已走完施工，另票另审）。

**锚 5（本票 spec 已回填＋CheckSpec 过）——绿（度量口径记档）**
- 事件 549 `fill_spec`（actor `acd0c6416bd56b79` 本票二班）在案。机械闸：`internal/gateway/ceo.go:627-639`——fill_spec 分支先 `inject.ParseTicketMD`（失败即 `resp(false)` **不落事件**）；`ParseTicketMD` 尾行 `return spec, CheckSpec(spec)`（`inject/inject.go:238`）。事件 549 落账＝解析＋八栏零缺闸过。
- DB `ticket` 0016 行 `length(spec)=2195` 非空。grpcurl 补径走的同为 Attach 流 admin 帧→`handleAdmin` 同一闸。
- 口径注记：摘要称「spec 3714B」vs DB 2195B（proto blob）vs 仓内 md 4082B——fill_spec 时读的 md 版本与 commit 版本（6631ae2）间的差异不可考；主断言绿，不据此驳。

## 交付形制核对

- **交付 ref**：事件 550 `deliver 6631ae2 exit=0`——票面第 8 栏「ref＝设计档链顶 commit」：6631ae2 在 accab10→19e0c90 链顶 ✅。
- **摘要四节形在**：run log `run/logs/acd0c6416bd56b79.log` 尾部实录——新增/修改面、根因、验证引、下一步四节全。
- **范围**：三 commit 纯 `docs/` 文件（2+1+1），零实现码——设计票不施工 ✅。范围外禁线逐条：0017 支 `5e9ce28` **未并 master**（`git merge-base --is-ancestor` 判 NOT in master）✅；历史丢判未回捞 ✅；审计 prefer 路由未动 ✅；devin CLI/ACP 未碰 ✅。
- **暗债对账**：三 commit diff grep `TODO|FIXME|XXX|待补|半成品|hack` 零命中；摘要「根因」节如实申报本班插曲（Attach 流断→收养重挂→帧泵空转→grpcurl 绕行）并写经验库 `42a646e`（另篇坑文，令#E1 义务产出，票外 docs commit，非码非票面改篡——记债项不记蔓延）＋「建议立票修…归 CEO 裁」。**无瞒报**。
- **首班 exit=2 处置**：票面附言与事件史（543 死/544 重派/545 二班 claim）一致；accab10（05:58，devin-CTO）为首班产出，二班摘要如实标注「前任班产出」。

## 质量三问（令#E5）

- 最优解：扫描移层＝壳无关根治（判决回流不再依赖壳行为）；文件兜底复用既有 `META_ART_DIR` 机制（先例 `exec-resp.log` 同构），无新凭证面、无逆向依赖。
- 第一性：直击「接收器聋」病根而非摞补丁；明确论证了 substring 扫描不可行（载荷无自闭界）、文件源值优先的机理——非 workaround。
- 洁净：纯文档交付，无重复实现/死码/调试残渣；边界外（§4）与风险回退（§5）自洽闭合。
- 无可指认的更优替代形制，不触发质量驳回；病根在设计→票面待裁：不适用（设计文档本身即交付物且自洽）。

## 债项

1. **帧泵空转缺陷**（摘要「根因」节申报）：sendPump 拉走即丢＋stream_closed 闩锁不可分辨＋全程零日志——建议立票修，归 CEO 裁。经验文已入 `42a646e`。
2. **`seat.go:533` create_ticket 静默洞**（审计中发现，非本票 diff）：`ParseTicketMD` 失败时 `nt.Spec` 留空**仍发 create_ticket**——空 spec 票可立库（后续 dispatch 才被 CheckSpec 拦）。本票 0017 未中招（blob 4356B 证解析成功），但闸有洞，建议随 0017 审或新票补 fail loud。
3. **spec 度量口径混乱**：票面「6723B」/摘要「3714B」vs DB proto blob（0016=2195B、0017=4356B）vs md 原文（4082B/6964B）——md 原文与 proto blob 两口径混用，今后票面/摘要宜标明所量为何物。

## 可沉淀意见（令#E8，纯意见不构成否决）

摘要根因节「grpcurl Attach 同 token 直推帧、网关机械闸逐条适用」打法——**可沉淀**，且已随 `42a646e` 经验文入库，无需另沉淀。

## 判决

**通过**。五锚机器证据闭合，形制全合，暗债无瞒报。子票 0017 交付（5e9ce28）另票另审，不入本判决。
