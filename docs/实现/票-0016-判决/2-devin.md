# 票 0016 判决（2-devin · 二判 · 通过）

审计 run `569a5c144ac593e4`（devin/SWE-2 Max）。**对账补发班**：首判 run
`cd4fe8c323a2ff94`（claude）判决档 `1-claude.md` 已落账（`e3461c6`）但其
stdout 哨兵被 JSON 包裹未捕获（事件 648 seat_err 实录 `type:"result"` 帧尾），
audit_dead 重派本班——恰为 D-42 本病的又一实证。本档独立复验，非对首判的
背书复述。交付链：`accab10`（设计档＋票 0017 票面）→ `19e0c90`（经验库）
→ `ac2b80b`（决策64 账，docs）→ `6631ae2`（本票 fill_spec，deliver ref＝
事件 550）。**只信 diff 与机器记录**——以下逐锚对账，每条指到机器输出。

## 逐锚对账（票面第 6 栏五锚）

**锚 1（设计档在＋根因表/双机制/去重与优先序）——绿**
- 文件在仓：`docs/实现/设计-0016-D42哨兵扫描移层.md`（accab10 引入，151 行）。
- 票面验证命令复跑：`grep -c "双机制\|去重\|sentinel.log" …` → **11**，exit=0。
- 节名机器证据（`grep -n "^#\{1,3\} "`）：§1「需求原文与现状确诊」含 8 行
  根因事实表（逐条指到 `seat.go:264`/`claude.go:19`/`run/logs/*` 实证）；
  §2「总体方案：双机制＋双发契约」（扫描移层＋sentinel.log 文件兜底）；
  §3.4「去重与优先序」；另 §3.1–3.5 落点明细、§5 风险回退、§6 测试母本——
  范围内钦定要件全在。

**锚 2（票 0017 在仓＋ParseTicketMD 解析过）——绿**
- 文件在仓：`docs/实现/票-0017-D42哨兵修复.md`（accab10 引入，127 行/6964B）。
- 解析机器实证：本班在仓内建一次性 harness 直调
  `inject.ParseTicketMD`（`inject/inject.go:193`，尾行 `return spec,
  CheckSpec(spec)` ——解析过＝八栏零缺闸过）→ 输出
  `PARSE_OK goal=367B why=578B scope=1732B oos=402B refs=525B
  anchors=1891B verify=334B deliver=303B`，八栏全非空。
  harness 用后已删（`rm -rf tmp-audit-check`），零仓面改动。

**锚 3（DB 票 0017 存在且 spec 非空）——绿（口径记档）**
- `sqlite3 run/meta.db "SELECT id,length(spec),phase …"` →
  `0017 | 4356 | 审计中`。存在 ✅、非空 ✅。
- 口径注记：票面括号「6723B」与 DB proto blob（4356B）及仓内 md（6964B）
  均不吻合——立票时刻所读工作区文件版本已不可考；锚的可判主句
  「存在且非空」机器实证绿，括号数字非任何口径下可判断言，不据此驳。

**锚 4（0017 派发真）——绿**
- `meta events --ticket 0017`：事件 540 `dispatch 设计中→已派`、
  事件 541 `claim 已派→施工中 d10ca1b4c6565b7b`。派与接均在案；
  其后 547 `deliver 5e9ce28`（子票施工已交付，另票另审）。

**锚 5（本票 spec 已回填＋CheckSpec 过）——绿**
- 事件 549 `fill_spec`（actor `acd0c6416bd56b79` 本票二班）落账——
  fill_spec 经网关机械闸：先 `ParseTicketMD`（失败不落事件），事件落账
  ＝解析＋八栏零缺闸过。
- DB `ticket` 0016 行 `length(spec)=2195` 非空。
- 本班直跑 parser 对仓内 `票-0016-…md`：`PARSE_OK`（goal=442B 等八栏全
  非空），与 DB 非空互证。

## 交付形制核对

- **交付 ref**：事件 550 `deliver 6631ae2 exit=0`——票面第 8 栏
  「ref＝设计档链顶 commit」：6631ae2 为交付时刻链顶 ✅。
- **摘要四节形在**：`run/logs/acd0c6416bd56b79.log` 尾部实录——
  新增/修改面、根因、验证引、下一步四节全。
- **范围**：交付链四 commit 全纯 `docs/`（`git show --stat`：accab10
  2 文件+278、19e0c90 1 文件+24、ac2b80b 2 文件+9、6631ae2 1 文件+87），
  零实现码——设计票不施工 ✅。范围外禁线逐条：0017 支 `5e9ce28` 未并
  master（`git merge-base --is-ancestor` exit=1）✅；历史丢判未回捞 ✅；
  prefer_shell 路由未动 ✅；devin CLI/ACP 未碰 ✅。
- **暗债对账**：交付链 diff grep `TODO|FIXME|XXX|HACK|待补|半成品|绕过`
  零命中。摘要「根因」节如实申报本班插曲（Attach 流断→收养重挂→帧泵空转
  →grpcurl 同 token 补径）并写经验库 `42a646e`＋「建议立票修…归 CEO 裁」；
  「下一步」节申报项与 diff 对得上。**无瞒报**。
- **首班 exit=2 处置**：票面附言与事件史（543 seat_err/544 requeue/545
  二班 claim）一致；accab10 为首班产出，二班摘要如实标注「前任班产出」。
- **ac2b80b 归属注记**：交付链中间的决策64 账 commit（ceo_memory＋债务
  台账，docs-only）属 CEO 账面流水非施工产出，不构成本票范围问题。

## 质量三问（令#E5）

- 最优解：扫描移层＝壳无关根治（判决回流不再依赖壳行为）；sentinel.log
  文件兜底复用既有 `META_ART_DIR` 机制（先例 `exec-resp.log` 同构），
  无新凭证面、无逆向依赖。
- 第一性：直击「接收器聋」病根而非摞补丁——substring 扫描不可行性有
  机理级论证（哨兵载荷无自闭界、粘文污染不可机判边界）；文件源值优先
  有据（文件行＝席位直写原文，免疫一切渲染层变形）。
- 洁净：纯文档交付，无重复实现/死码/调试残渣；§4 边界外与 §5 风险回退
  自洽闭合。
- 无可指认的更优替代形制；病根在设计→票面待裁：不适用。

## 债项

1. **帧泵空转缺陷**（摘要申报）：sendPump 拉走即丢＋stream_closed 闩锁
   不可分辨＋全程零日志——经验文 `42a646e` 已入，建议立票修归 CEO 裁。
2. **spec 度量口径混乱**（沿首判债项再记）：票面「6723B」/摘要「3714B」
   vs DB proto blob（0016=2195B、0017=4356B）vs md 原文（4082B/6964B）
   ——今后票面/摘要宜标明所量为何物。
3. **对账补发机制实证**：首判判决档已 commit 而哨兵被 JSON 吞（事件
   648），audit_dead 重派产生本档——同票双判档并存属机制预期（D-42
   未部署期），0017 部署后此类重派应绝迹。

## 可沉淀意见（令#E8，纯意见不构成否决）

摘要根因节「grpcurl Attach 同 token 直推帧过同一机械闸」绕行打法——
**可沉淀**，已随 `42a646e` 入库，无需另沉淀。

## 判决

**通过**。五锚机器证据闭合，形制全合，暗债无瞒报。首判 `1-claude.md`
结论经独立复验成立。子票 0017 交付（`5e9ce28`）另票另审，不入本判决。
