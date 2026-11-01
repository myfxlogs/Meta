# 票 0016 判决（7-claude · 七判 · 通过）

审计 run `035455da3f31532e`（claude/glm-5.3[1m]，audit_dead 触发第七棒，
事件 798/799——第六棒 `ea4f53aca753d3f9` 亦死于壳吞哨兵，事件 797
seat_err「净退无 META_VERDICT 哨兵」实录）。前六档（`e3461c6`/`4939006`/
`e453124`/`927d2fb`/`1b12179`/`7b428ed`）均判通过；本班**不信前判自述，
五锚全部独立重取机器证据**。交付链：`effddc7`（基）→ `accab10`（设计档
151 行＋票 0017 票面 127 行，`git show --stat` 两文件 278+ 实名）→
`19e0c90`（经验库 24 行）→ `ac2b80b`（决策64 账，CEO 流水非施工产出，
四档先例一致注记）→ `6631ae2`（本票八栏 87 行，deliver ref＝事件 550）。

## 逐锚对账（票面第 6 栏五锚，每条指向本班机器输出）

**锚 1（设计档在＋根因表/双机制/sentinel.log/去重与优先序）——绿**
- `git show --stat accab10` 亲见设计档 +151 行整文件引入。
- 票面验证命令第一段复跑：`grep -c "双机制\|去重\|sentinel.log"
  docs/实现/设计-0016-D42哨兵扫描移层.md` → **11** rc=0。
- `grep -n "^##"` 亲见节名：L7「需求原文与现状确诊」、L27「总体方案：
  双机制＋双发契约」、L79「3.3 sentinel.log 文件兜底泵」、L90「3.4
  去重与优先序」、L128「风险与回退」、L137「测试面」——票面钦定要件全在。

**锚 2（票 0017 在仓＋ParseTicketMD 解析过）——绿**
- 本班一次性 harness（仓内 `tmp-audit-7c/main_test.go`，用后即删）直调
  `inject.ParseTicketMD`（`internal/inject/inject.go:193`，尾行
  `return spec, CheckSpec(spec)` 本班亲读——八栏缺一即错）→
  `0017 PARSE_OK md=6964B marshal=6723B goal=367B why=578B scope=1732B
  oos=402B refs=525B anchors=1891B verify=334B deliver=303B`——八栏
  全非空，marshal 与票面括号「6723B」**逐字节吻合**。

**锚 3（DB 票 0017 存在且 spec 非空）——绿（口径复核吻合）**
- `sqlite3 run/meta.db` → `0017 | text | 4356 | 16 | 待部署`（audit_pass
  后相位前移，事件 652）；writefile 落盘 6741B 非空。
- 口径复核：harness 对仓内 md parse→marshal 得 6723B＝票面括号值，
  立票时解析的就是仓内这份 md；DB 列现值 4356 字符/6741B 为全票册性
  text 化结果（债项 1，本班双 ERR 复测亲证），非本票交付缺陷。

**锚 4（0017 派发真）——绿**
- 本班 `meta events --ticket 0017`：事件 540 `dispatch 设计中→已派
  gateway`、541 `claim 已派→施工中 d10ca1b4c6565b7b`、547 `deliver
  5e9ce28`、652 `audit_pass`（子票另票另审，不入本判决）。

**锚 5（本票 spec 已回填＋CheckSpec 过）——绿**
- `sqlite3 ticket_event` 亲见：id **549** `fill_spec | acd0c6416bd56b79 |
  八栏回填`、id **550** `deliver | … 6631ae2 … exit=0`。
- 机械闸亲读（`internal/gateway/ceo.go:627-639`）：fill_spec 分支先
  `ParseTicketMD`（失败即 `resp(false)` **不落事件**）再
  `UpdateSpec`——事件 549 落账＝解析＋八栏零缺闸过。
- harness 直调本票票面 md → `0016 PARSE_OK md=4082B marshal=3714B
  goal=442B why=513B scope=644B oos=374B refs=425B anchors=512B
  verify=212B deliver=194B` 八栏全非空，marshal 与交付摘要「spec
  3714B」逐字节吻合；DB 0016 行 `text | 2195 | 审计中`（落盘 3732B）非空。

**验证命令（第 7 栏三段连跑）——绿**：grep=11 → `meta phase` rc=0
（`0017 待部署 frozen=false`）→ `meta events` rc=0，**总 rc=0**
（本班 VERIFY_TOTAL_RC=0 实录）。

## 交付形制核对

- **交付 ref**：事件 550 `deliver 6631ae2 exit=0`；6631ae2 在 master
  （`git branch --contains` 亲证，master/t/0019/c/0018-design 含之）。
- **摘要四节形在**：`run/logs/acd0c6416bd56b79.log` 尾部本班复读——
  新增/修改面、根因、验证引、下一步四节全，末行 `META_DELIVER:6631ae2`。
- **范围**：`git diff effddc7..6631ae2 --stat`＝6 文件全 `docs/`
  （398+/1-），零实现码——设计票不施工 ✅。禁线逐条：0017 支交付
  `5e9ce28` NOT in master（`git merge-base --is-ancestor` rc=1 亲证）✅；
  历史丢判未回捞、prefer_shell 配置未动（链零配置零码）、devin CLI/ACP
  未碰 ✅。
- **暗债对账（令#E6）**：链 diff grep
  `TODO|FIXME|XXX|HACK|待补|半成品|绕过|workaround` **零命中**（rc=1
  本班亲跑）；摘要「根因」节如实申报本席插曲（Attach 流断→收养重挂→
  帧泵空转→grpcurl 绕行）并写经验库＋「建议立票修归 CEO 裁」；「下一步」
  申报项与实况对上。**无瞒报**。
- **首班 exit=2 处置**：票面附言与事件史（543 死/544 重派/545 二班
  claim）一致；accab10 为首班产出，二班摘要如实标注。

## 质量三问（令#E5）

- 最优解：扫描移层＝壳无关根治；sentinel.log 兜底复用既有
  `META_ART_DIR` 机制，无新凭证面。
- 第一性：直击「接收器聋」病根；substring 扫描不可行有机理论论证；
  文件源值优先有据。
- 洁净：纯文档交付，无重复实现/死码/调试残渣。
- 无可指认的更优替代形制，不触发质量驳回；病根在设计→票面待裁：不适用。

## 债项（本班现状复核，移交非否决）

1. **DB spec 全票册不可解析仍在**（系统级非本票病）：本班 harness 对
   DB 现值 Unmarshal——0016（3732B）/0017（6741B）双双
   `proto: cannot parse invalid wire-format data`（决策 67 text 化所致）。
   活径无碍、冷径受累；恢复源＝仓内票面 md（本班 parse→marshal 与
   票面/摘要所称逐字节吻合）。归 CEO 裁存量回填专项。
2. **帧泵空转缺陷**（摘要申报）：经验文 `42a646e` 在档但债务台账
   grep「帧泵/收养重挂」**零命中**（本班 rc=1 亲证）、无票。建议挂账
   立票，归 CEO 裁。
3. **create_ticket 静默洞**：本班 sqlite3 亲见 `0018 | null | | 17 |
   待审`——空 spec 票活证仍在。建议 fail loud。
4. **spec 度量三口径并存**（md 原文/proto marshal/DB text 列）：本班
   三数齐测（4082/3714/3732 与 6964/6723/6741），票面/摘要宜标所量。
5. **D-44 四合修去向**：台账行 82 在账记「并入 0016 或紧随小票」而
   0016 为设计票未含实修——归 CEO 裁落票去向。

## 可沉淀意见（令#E8，纯意见不构成否决）

摘要「grpcurl Attach 同 token 直推帧、机械闸逐条适用」绕行打法——
**可沉淀**，已随 `42a646e` 经验文在档。本班循 req-9ce5 八档先例将
「fd 直写扫描管」自救动作适用于本票第七棒，同荐入库。

## 附注（本班过程留痕）

一次性审计 harness（`tmp-audit-7c/main_test.go`）与 DB spec 取证 dump
（`/tmp/t0016-audit7-*.spec`）用后即删，`git status --short` 复核净后
才提交本档。六棒五死皆 claude 壳吞哨兵（D-42 未部署期，B1 部署闸占用
中）——本班判决工作与哨兵回流双轨送达（档内结论与哨兵一致）。

## 判决

**通过**。五锚机器证据闭合（本班 harness 直调解析复算＋事件/DB/diff
亲证），形制全合，暗债无瞒报。前六档结论经本班独立复算一致成立。
子票 0017 交付（`5e9ce28`，audit_pass 事件 652）另票另审，不入本判决。
