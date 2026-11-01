# 票 0016 判决（6-claude · 六判 · 通过）

审计 run `ea4f53aca753d3f9`（claude/glm-5.3[1m]，audit_dead 触发第六棒，
事件 785/786）。前五棒 `1-claude.md`（`e3461c6`）/`2-devin.md`（`4939006`）/
`3-claude.md`（`e453124`）/`4-claude.md`（`927d2fb`）/`5-claude.md`（`1b12179`）
均判通过，其中 claude 四班 stdout 哨兵均被壳吞（事件 648/661/735/774/784
audit_dead——D-42 本病在审计席上的持续实演；0017 修复件已 audit_pass
待部署，事件 654/655 实录 B1 部署闸占用中）。本班独立复算，非对前判的
背书复述。交付链：`accab10`（设计档＋票 0017 票面）→ `19e0c90`（经验库
哨兵三壳失明谱）→ `ac2b80b`（决策64 账，CEO 流水，非施工产出）→
`6631ae2`（本票 fill_spec，deliver ref＝事件 550）。**只信 diff 与机器
记录**——以下逐锚对账，每条指到本班亲跑的机器输出。

## 逐锚对账（票面第 6 栏五锚）

**锚 1（设计档在＋根因表/双机制/去重与优先序）——绿**
- 文件在仓：`docs/实现/设计-0016-D42哨兵扫描移层.md`（151 行，accab10
  引入，本班 `git diff effddc7..6631ae2 --stat` 亲见 +151 行整文件）。
- 票面验证命令本班复跑：`grep -c "双机制\|去重\|sentinel.log"` → **11**
  rc=0；`meta phase --ticket 0017` rc=0、`meta events --ticket 0017`
  rc=0——第 7 栏三段命令**连跑总 rc=0**（本班 VERIFY_TOTAL_RC=0 实录）。
- 节名本班 `grep -n "^##"` 亲见：§1「需求原文与现状确诊」（根因确诊表）、
  §2「总体方案：双机制＋双发契约」、§3.4「去重与优先序」、§3.3
  「sentinel.log 文件兜底泵」、§3.1–3.5 落点明细、§5 风险与回退、
  §6 测试面——票面范围内钦定要件全在。

**锚 2（票 0017 在仓＋ParseTicketMD 解析过）——绿**
- 文件在仓：`docs/实现/票-0017-D42哨兵修复.md`（6964B，accab10 引入）。
- 本班一次性 harness（仓内 `tmp-audit-6c/`，用后即删、`git status`
  复核净）直调 `inject.ParseTicketMD`（`internal/inject/inject.go:193`，
  尾行 `return spec, CheckSpec(spec)` 本班亲读）→ 输出
  `0017 PARSE_OK md=6964B marshal=6723B goal=367B why=578B scope=1732B
  oos=402B refs=525B anchors=1891B verify=334B deliver=303B`——八栏
  全非空，marshal 与票面括号「6723B」**逐字节吻合**。

**锚 3（DB 票 0017 存在且 spec 非空）——绿（口径复核吻合）**
- 本班 `sqlite3 run/meta.db` → `0017 | text | 4356 | 39 | 待部署`；
  writefile 落盘 6741B。存在 ✅、非空 ✅。
- 口径复核：harness 对仓内 md parse→marshal 得 **6723B**，与票面括号
  「6723B」**逐字节吻合**——立票时解析的就是仓内这份 md；DB 列现值
  4356 字符/6741B 为全票册性 text 化结果（债项 1，本班 unmarshal 双 ERR
  亲证仍在），非本票交付缺陷。

**锚 4（0017 派发真）——绿**
- 本班 `meta events --ticket 0017`：事件 540 `dispatch 设计中→已派
  gateway`、事件 541 `claim 已派→施工中 d10ca1b4c6565b7b`；其后 547
  `deliver 5e9ce28`、652 `audit_pass`（子票另票另审，不入本判决）。

**锚 5（本票 spec 已回填＋CheckSpec 过）——绿**
- 事件 549 `fill_spec`（actor `acd0c6416bd56b79` 本票二班）在案。
- 机械闸亲证（`internal/gateway/ceo.go:627-639`，本班读码）：
  fill_spec 分支先 `inject.ParseTicketMD`（失败即 `resp(false)` **不落
  事件**）——事件 549 落账＝解析＋八栏零缺闸过。
- 本班 harness 直调仓内 `票-0016-D42哨兵修复设计.md` → `0016 PARSE_OK
  md=4082B marshal=3714B goal=442B why=513B scope=644B oos=374B
  refs=425B anchors=512B verify=212B deliver=194B` 八栏全非空，marshal
  与交付摘要所称「spec 3714B」**逐字节吻合**；DB 0016 行
  `text | 2195 | 26 | 审计中`（落盘 3732B）非空。

## 交付形制核对

- **交付 ref**：事件 550 `deliver 6631ae2 exit=0`；6631ae2 在
  accab10→19e0c90→6631ae2 链顶且在 master（`git branch --contains`
  本班亲证，master 与 t/0019 两支含之）✅。
- **摘要四节形在**：`run/logs/acd0c6416bd56b79.log` 尾部本班复读——
  新增/修改面、根因、验证引、下一步四节全，末行 `META_DELIVER:6631ae2`。
- **范围**：`git diff effddc7..6631ae2 --stat`＝6 文件全 `docs/`
  （398+/1-），零实现码——设计票不施工 ✅。范围外禁线逐条：0017 支
  交付 `5e9ce28` NOT in master（`git merge-base --is-ancestor` 本班亲证
  NOT-IN-MASTER）✅；历史丢判未回捞 ✅；prefer_shell 路由/配置未动
  （链零配置零码）✅；devin CLI/ACP 未碰 ✅。
- **暗债对账**：链 diff grep
  `TODO|FIXME|XXX|HACK|待补|半成品|绕过|workaround` 零命中（本班亲跑
  rc=1）；摘要「根因」节如实申报本班插曲（Attach 流断→收养重挂→
  帧泵空转→grpcurl 绕行）并写经验库 `42a646e`（本班核实文件在：
  `docs/经验库/收养重挂后帧泵空转.md`，commit 在 master 亲证）＋
  「建议立票修…归 CEO 裁」；「下一步」节申报项与 diff 对得上。
  **无瞒报**。
- **首班 exit=2 处置**：票面附言与事件史（543 死/544 重派/545 二班
  claim）一致；accab10 为首班产出，二班摘要如实标注。
- **ac2b80b 归属注记**：链中决策64 账 commit（CEO 流水，docs-only）
  非施工产出，四档先例一致注记。

## 质量三问（令#E5）

- 最优解：扫描移层＝壳无关根治；sentinel.log 兜底复用既有
  `META_ART_DIR` 机制（先例 `exec-resp.log` 同构），无新凭证面。
- 第一性：直击「接收器聋」病根；substring 扫描不可行有机理论论证；
  文件源值优先有据。
- 洁净：纯文档交付，无重复实现/死码/调试残渣。
- 无可指认的更优替代形制，不触发质量驳回；病根在设计→票面待裁：
  不适用。

## 债项（本班 HEAD e10cd27 现状复核）

1. **DB spec 列全票册不可解析（系统级非本票病，本班亲证仍在）**：
   本班 harness 对 DB 现值试 Unmarshal——0016（3732B）/0017（6741B）
   双双 `proto: cannot parse invalid wire-format data`（决策 67 事故链
   text 化所致）；活径无碍（dispatch 用内存 spec）、冷径受累（网关
   重启后 specComplete 坏）。恢复源＝仓内票面 md（本班 parse→marshal
   与票面/摘要所称逐字节吻合）。归 CEO 裁存量回填专项。
2. **帧泵空转缺陷**（摘要申报）：经验文 `42a646e` 在档但债务台账仍
   无行（本班 grep「帧泵/收养重挂」rc=1 零命中）、无票。建议挂账立票，
   归 CEO 裁。
3. **create_ticket 静默洞**（首判发现，六班复核成立）：DB 0018 行
   spec null 活证仍在（本班 sqlite3 亲见 `0018|null||23|施工中`）——
   空 spec 票可立库，dispatch 才被拦。建议 fail loud。
4. **spec 度量口径三口径并存**（md 原文／proto marshal／DB text 列）：
   本班三数齐测（4082/3714/3732 与 6964/6723/6741），今后票面/摘要
   宜标明所量为何物。
5. **D-44 四合修去向**（台账在账行，非本票交付缺陷）：台账记「并入
   0016 或紧随小票」而 0016 为设计票未含实修——归 CEO 裁落票去向。

## 可沉淀意见（令#E8，纯意见不构成否决）

摘要根因节「grpcurl Attach 同 token 直推帧、网关机械闸逐条适用」绕行
打法——**可沉淀**，已随 `42a646e` 经验文在档，无需另沉淀。

## 附注（本班过程留痕）

一次性审计 harness（仓内 `tmp-audit-6c/main_test.go`，`go test` 直调）
与 DB spec 取证 dump（/tmp/t0016-audit6-spec-*.bin）用后即删（`git
status --short` 复核净后才提交本档）。

## 判决

**通过**。五锚机器证据闭合（本班 harness 直调解析复算＋事件/DB/diff
亲证），形制全合，暗债无瞒报。前五棒结论经本班独立复算一致成立。
子票 0017 交付（`5e9ce28`，已 audit_pass 事件 652）另票另审，不入本
判决。**附系统观察**：本票六棒审计五死（claude×4＋devin×1）皆缘
claude 壳吞哨兵（D-42 未部署期，事件 654/655 B1 部署闸占用中）——
判决档落仓而哨兵不达；0017 部署后此循环应绝迹。
