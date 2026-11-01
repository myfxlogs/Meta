# 票 0016 判决（4-claude · 四判 · 通过）

审计 run `91ee100501a4145e`（claude/glm-5.3[1m]，audit_dead 触发第四棒）。
前三棒 `1-claude.md`（`e3461c6`）/`2-devin.md`（`4939006`）/`3-claude.md`
（`e453124`）均判通过但 stdout 哨兵三次未达网关（事件 648/661/735
audit_dead——claude 壳 JSON 包裹吞哨兵，即 D-42 本病在审计席上的第 N 次
实演；0017 修复件已过审待部署，部署闸 B1 占用中）。本班独立复算，
非对前判的背书复述。交付链：`accab10`（设计档＋票 0017 票面）→
`19e0c90`（经验库哨兵三壳失明谱）→ `ac2b80b`（决策64 账，CEO 流水，
非施工产出）→ `6631ae2`（本票 fill_spec，deliver ref＝事件 550）。
**只信 diff 与机器记录**——以下逐锚对账，每条指到本班亲跑的机器输出。

## 逐锚对账（票面第 6 栏五锚）

**锚 1（设计档在＋根因表/双机制/去重与优先序）——绿**
- 文件在仓：`docs/实现/设计-0016-D42哨兵扫描移层.md`（9.8K/151 行，accab10 引入）。
- 票面验证命令本班复跑：`grep -c "双机制\|去重\|sentinel.log"` → **11**，
  exit=0（第 7 栏三段命令连跑总 rc=0）。
- 节名与根因表机器证据（`grep -n "^#\{1,3\} "`＋`sed -n 7,26p`）：§1「需求
  原文与现状确诊」含 **8 行确诊事实表**，逐行指到 `internal/seat/seat.go:264`
  （扫描点在 `ad.Parse` 之前）、`internal/adapter/claude.go:19`
  （stream-json）、`run/logs/fa7aa447*`/`ca93f994*` 实证；§2「总体方案：
  双机制＋双发契约」（扫描移层＋sentinel.log 文件兜底）；§3.4「去重与
  优先序」；另 §3.1–3.5 落点明细、§4 边界外、§5 风险与回退、§6 测试面
  ——范围内钦定要件全在。

**锚 2（票 0017 在仓＋ParseTicketMD 解析过）——绿**
- 文件在仓：`docs/实现/票-0017-D42哨兵修复.md`（6.8K/6964B，accab10 引入）。
- 本班一次性 harness（仓内 `tmp-audit-4c/`，直调 `inject.ParseTicketMD`，
  用后即删、`git status` 复核净）→ 输出
  `0017 PARSE_OK md=6964B marshal=6723B goal=367B why=578B scope=1732B
  oos=402B refs=525B anchors=1891B verify=334B deliver=303B`——八栏全非空；
  `ParseTicketMD` 尾行 `return spec, CheckSpec(spec)`（inject.go:238）——
  解析过＝八栏零缺闸过。

**锚 3（DB 票 0017 存在且 spec 非空）——绿（口径本班三判复定）**
- `sqlite3 run/meta.db "SELECT id,typeof(spec),length(spec),octet_length(spec),
  phase …"` → `0017 | text | 4356 | 6741 | 待部署`。存在 ✅、非空 ✅。
- 口径复定（三判结论本班复算吻合）：harness 对仓内 md 直调 parse→marshal
  得 **6723B**，与票面括号「6723B」**逐字节吻合**——立票时解析的就是仓内
  这份 md，无版本漂移；DB 列现值 4356/6741 是全票册性 text 化结果（债项 1），
  非本票交付缺陷。

**锚 4（0017 派发真）——绿**
- `meta events --ticket 0017`：事件 540 `dispatch 设计中→已派 gateway`、
  事件 541 `claim 已派→施工中 d10ca1b4c6565b7b`。派与接均在案；其后 547
  `deliver 5e9ce28`、652 `audit_pass`（子票另票另审，不入本判决）。

**锚 5（本票 spec 已回填＋CheckSpec 过）——绿**
- 事件 549 `fill_spec`（actor `acd0c6416bd56b79` 本票二班）在案。
- 机械闸亲证（`internal/gateway/ceo.go` fill_spec 分支）：越票拒/相位限→
  `ParseTicketMD` 失败即 `resp(false)` **先于**事件落账返回；事件行
  （ceo.go:638）在解析＋UpdateSpec 成功后才写——事件 549 落账＝八栏闸过。
- harness 直调仓内 `票-0016-…md` → `PARSE_OK md=4082B marshal=3714B
  goal=442B … deliver=194B` 八栏全非空，marshal 与交付摘要所称「spec
  3714B」**逐字节吻合**；DB 0016 行 `text | 2195 | 3732` 非空。

## 交付形制核对

- **交付 ref**：事件 550 `deliver 6631ae2 exit=0`——票面第 8 栏「ref＝设计档
  链顶」：6631ae2 在 accab10→19e0c90→6631ae2 链顶且在 master
  （`git merge-base --is-ancestor` 过）✅。
- **摘要四节形在**：`run/logs/acd0c6416bd56b79.log` 尾部实录——新增/修改面、
  根因、验证引、下一步四节全，末行 `META_DELIVER:6631ae2`。
- **范围**：`git diff effddc7..6631ae2 --stat`＝6 文件全 `docs/`（398+/1-），
  零实现码——设计票不施工 ✅。范围外禁线逐条：0017 支交付 `5e9ce28` NOT
  in master（`git merge-base --is-ancestor` exit=1）✅；历史丢判未回捞 ✅；
  prefer_shell 路由/配置未动（链零配置零码）✅；devin CLI/ACP 未碰 ✅。
- **暗债对账**：链 diff grep `TODO|FIXME|XXX|HACK|待补|半成品` 零命中；
  摘要「根因」节如实申报本班插曲（Attach 流断→收养重挂→帧泵空转→
  grpcurl 绕行）并写经验库 `42a646e`（本班核实：单文件 47+ 行纯 docs，
  令#E1 义务产出）＋「建议立票修…归 CEO 裁」；「下一步」节申报项与
  diff 对得上。**无瞒报**。
- **首班 exit=2 处置**：票面附言与事件史（543 死/544 重派/545 二班 claim）
  一致；accab10 为首班产出，二班摘要如实标注。
- **ac2b80b 归属注记**：链中决策64 账 commit（CEO 账面流水，docs-only）
  非施工产出，不构成本票范围问题（二/三判已注记，本班复核 `--stat`
  为 ceo_memory＋债务台账 2 文件纯账面）。

## 质量三问（令#E5）

- 最优解：扫描移层＝壳无关根治；sentinel.log 兜底复用既有 `META_ART_DIR`
  机制（先例 `exec-resp.log` 同构），无新凭证面、无逆向依赖。
- 第一性：直击「接收器聋」病根——确诊表对 8 项事实逐条给机器证据
  （含 1MB 缓冲上限、preview 截断两条暗径），substring 扫描不可行有
  机理论证；文件源值优先有据。
- 洁净：纯文档交付，无重复实现/死码/调试残渣；§4 边界外与 §5 风险回退
  自洽闭合。
- 无可指认的更优替代形制，不触发质量驳回；病根在设计→票面待裁：不适用。

## 债项（HEAD e453124 现状核对）

1. **DB spec 列全票册不可解析（系统级非本票病，三判新证本班复核仍在）**：
   11 张非空 spec 全 `typeof=text`（本班查 0016=2195/3732、0017=4356/6741
   均 text）——决策 67 事故链所致；活径无碍（dispatch 用内存 spec）、
   冷径受累（网关重启后 specComplete 坏）。恢复源＝仓内票面 md（本班
   harness 实证 parse→marshal 与票面/摘要所称逐字节吻合）。归 CEO 裁
   （存量重解析回填专项；0018/D-45 是否涵盖待确认）。
2. **帧泵空转缺陷**（摘要申报）：sendPump 拉走即丢＋stream_closed 闩锁
   不可分辨＋全程零日志——经验库在档（`42a646e`）但**债务台账仍无行**
   （本班 grep「帧泵/收养重挂」零命中）、无票。建议挂账立票，归 CEO 裁。
3. **create_ticket 静默洞**（首判发现，三判复核成立，本班未再验）：gateway
   create 分支无 spec 闸——空 spec 票可立库（DB 0018 行 spec null 活证），
   dispatch 才被 CheckSpec 拦。建议 fail loud。
4. **spec 度量口径**（本班定分复算）：三口径并存——md 原文字节（0016=
   4082B/0017=6964B）／proto marshal 字节（票面「6723B」、摘要「3714B」
   属此，均为仓内 md 现值）／DB text 列字符·字节。今后票面/摘要宜标明
   所量为何物。
5. **D-44 四合修去向**（台账在账行，非本票交付缺陷）：台账记「并入 0016
   或紧随小票（最高优先）」，而 0016 为设计票未含 D-44 实修——归 CEO 裁
   落票去向。

## 可沉淀意见（令#E8，纯意见不构成否决）

摘要根因节「grpcurl Attach 同 token 直推帧、网关机械闸逐条适用」绕行
打法——**可沉淀**，已随 `42a646e` 经验文在档，无需另沉淀。

## 附注（本班过程留痕）

一次性审计 harness（`tmp-audit-4c/main.go`，用后即删，`git status`
复核净后才提交本档）——沿用三判「防误扫入仓」教训（e453124 曾清
`42eda13` 误扫件）。

## 判决

**通过**。五锚机器证据闭合（本班 harness 直调解析复算＋事件/DB/代码径
亲证），形制全合，暗债无瞒报。前三棒结论经本班独立复算一致成立。
子票 0017 交付（`5e9ce28`，已 audit_pass 事件 652）另票另审，不入本
判决。**附系统观察**：本票四棒审计三死皆因 claude 壳吞哨兵（D-42 未
部署期），判决档落仓而哨兵不达——0017 部署后此循环应绝迹。
