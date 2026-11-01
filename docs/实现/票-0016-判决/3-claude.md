# 票 0016 判决（3-claude · 三判 · 通过）

审计 run `de055af74e326c17`（claude/glm-5.3[1m]）。**第三棒独立复算**：首判
`1-claude.md`（`e3461c6`）与二判 `2-devin.md`（`4939006`）均判通过但 stdout
哨兵未达网关（事件 648/661 audit_dead——JSON 包裹/席位死，即 D-42 本病实演），
本班重派再判。**只信 diff 与机器记录**，本档非对前判的背书复述。交付链：
`accab10`（设计档＋票 0017 票面）→ `19e0c90`（经验库哨兵三壳失明谱）→
`ac2b80b`（决策64 账，CEO 流水，非施工产出）→ `6631ae2`（本票 fill_spec，
deliver ref＝事件 550）。

## 逐锚对账（票面第 6 栏五锚）

**锚 1（设计档在＋根因表/双机制/去重与优先序）——绿**
- 文件在仓：`docs/实现/设计-0016-D42哨兵扫描移层.md`（accab10 引入，151 行）。
- 票面验证命令本班复跑：`grep -c "双机制\|去重\|sentinel.log" …` → **11**，
  exit=0。
- 节名机器证据（`grep -n "^#\{1,3\} "`）：§1「需求原文与现状确诊」含 8 行
  确诊表（逐条指 `seat.go:264`/`claude.go:19`/`run/logs/*` 实证）；§2「总体
  方案：双机制＋双发契约」（扫描移层＋sentinel.log 文件兜底）；§3.4「去重
  与优先序」；另 §3.1–3.5 落点明细、§4 边界外、§5 风险与回退、§6 测试面
  ——范围内钦定要件全在。

**锚 2（票 0017 在仓＋ParseTicketMD 解析过）——绿**
- 文件在仓：`docs/实现/票-0017-D42哨兵修复.md`（accab10 引入，6964B）。
- 本班仓内一次性 harness 直调 `inject.ParseTicketMD`
  （`internal/inject/inject.go:193`，尾行 `return spec, CheckSpec(spec)`）
  → 输出 `PARSE_OK … goal=367B why=578B scope=1732B oos=402B refs=525B
  anchors=1891B verify=334B deliver=303B`，八栏全非空。harness 用后即删。

**锚 3（DB 票 0017 存在且 spec 非空）——绿（口径本班定分，见债项1）**
- `sqlite3 run/meta.db "SELECT id,typeof(spec),length(spec),octet_length(spec)
  …"` → `0017 | text | 4356 | 6741`。存在 ✅、非空 ✅。
- 口径定分（修正前两判「版本不可考」猜测）：harness 对仓内 md 直调
  parse→marshal 得 **6723B**，与票面括号「6723B」**逐字节吻合**——立票时
  解析的就是仓内这份 md，无版本漂移；DB 列现值 4356/6741 是全票册性
  text 化结果（债项 1），非本票交付缺陷。

**锚 4（0017 派发真）——绿**
- `meta events --ticket 0017`：事件 540 `dispatch 设计中→已派 gateway`、
  事件 541 `claim 已派→施工中 d10ca1b4c6565b7b`。其后 547 `deliver 5e9ce28`、
  652 `audit_pass`（子票另票另审，不入本判决）。

**锚 5（本票 spec 已回填＋CheckSpec 过）——绿**
- 事件 549 `fill_spec`（actor `acd0c6416bd56b79` 本票二班）在案。
- 机械闸亲证：`internal/gateway/ceo.go` fill_spec 分支——越票拒/相位限
  →`ParseTicketMD` 失败即 `resp(false)` **先于**事件落账返回；事件行
  （ceo.go:638）在解析＋UpdateSpec 成功后才写。事件 549 落账＝八栏闸过。
- harness 直调仓内 `票-0016-…md`：`PARSE_OK … goal=442B why=513B scope=644B
  oos=374B refs=425B anchors=512B verify=212B deliver=194B`，八栏全非空；
  DB 0016 行 `text | 2195 | 3732` 非空。

## 交付形制核对

- **交付 ref**：事件 550 `deliver 6631ae2 exit=0`——票面第 8 栏「ref＝设计档
  链顶」：6631ae2 在 accab10→19e0c90→6631ae2 链顶 ✅。
- **摘要四节形在**：`run/logs/acd0c6416bd56b79.log` 尾部实录——新增/修改面、
  根因、验证引、下一步四节全，末行 `META_DELIVER:6631ae2`。
- **范围**：`git diff accab10~1(effddc7)..6631ae2 --stat`＝6 文件全 `docs/`
  （398+/1-），零实现码——设计票不施工 ✅。范围外禁线逐条：0017 支
  `t/0017-d42-sentinel` 与交付 ref `5e9ce28` 均 NOT in master
  （`git merge-base --is-ancestor` 双 exit=1）✅；历史丢判未回捞 ✅；
  prefer_shell 路由/配置未动（链零配置零码）✅；devin CLI/ACP 未碰 ✅。
- **暗债对账**：链 diff grep `TODO|FIXME|XXX|HACK|待补|半成品` 零命中；
  摘要「根因」节如实申报本班插曲（Attach 流断→收养重挂→帧泵空转→
  grpcurl 绕行）并写经验库 `42a646e`（档在：`docs/经验库/收养重挂后帧泵
  空转.md`）＋「建议立票修…归 CEO 裁」。**无瞒报**。
- **首班 exit=2 处置**：票面附言与事件史（543 死/544 重派/545 二班 claim）
  一致；accab10 为首班产出，二班摘要如实标注。
- **ac2b80b 归属注记**：链中决策64 账 commit（CEO 账面流水，docs-only，
  2 判已注记）非施工产出，不构成本票范围问题。

## 质量三问（令#E5）

- 最优解：扫描移层＝壳无关根治；sentinel.log 兜底复用既有 `META_ART_DIR`
  机制（先例 `exec-resp.log` 同构），结构体增字段零破坏、无新凭证面、
  无逆向依赖。
- 第一性：直击「接收器聋」病根——substring 扫描不可行有机理论证（载荷
  无自闭界、粘文不可机判边界）；文件源值优先有据；事件驱动零定时器
  （禁区#1/R2 合规）；扫描域界定防自吞（tool_use 不进 Text）。
- 洁净：纯文档交付，无重复实现/死码/调试残渣；§4 边界外与 §5 风险回退
  自洽闭合。
- 无可指认的更优替代形制，不触发质量驳回；病根在设计→票面待裁：不适用。

## 债项

1. **DB spec 列全票册不可解析（本班新证，系统级非本票病）**：11 张非空
   spec 全 `typeof=text` 且首 `0AEFBFBD…`（varint 字节被 U+FFFD 替换）；
   本班 unmarshal 实测 0016/0017 双双 `invalid wire-format data`。**活径
   无碍**——dispatch 用内存 spec（`dispatch.go:29-31`「驳回重派…仍在内存」）；
   **冷径受累**——`specComplete`（ceo.go:509-519）unmarshal 失败＝false，
   网关重启后再派/CEO 面读票即坏。根因＝决策 67 事故链（坏字节「进票面
   10 张 spec」＋清毒 replace 语义，ceo_memory.md:508-509）。恢复源＝仓内
   票面 md（本班 harness 实证 parse→marshal 与票面所称吻合）。**建议 CEO
   裁：存量重解析回填专项，并确认 0018/D-45 是否涵盖（据决策 68 描述为
   入账消毒预防件，未见存量修复）**。
2. **帧泵空转缺陷**（摘要申报）：sendPump 拉走即丢＋stream_closed 闩锁
   不可分辨＋全程零日志——经验库在档（42a646e）但债务台账无行（本班
   HEAD 2ff4538 grep 零命中）、无票。建议挂账立票，归 CEO 裁。
3. **create_ticket 静默洞**（首判发现，本班复核成立）：gateway create 分支
   无 spec 闸——空 spec 票可立库（DB 0018 行 spec null 即该形态活证），
   dispatch 才被 CheckSpec 拦。建议 fail loud。
4. **spec 度量口径**（前两判已记，本班定分）：三口径并存——md 原文字节／
   proto marshal 字节（票面「6723B」属此）／DB text 列字符·字节
   （2195/3732）。今后票面/摘要宜标明所量为何物。

## 可沉淀意见（令#E8，纯意见不构成否决）

摘要根因节「grpcurl Attach 同 token 直推帧、网关机械闸逐条适用」绕行
打法——**可沉淀**，已随 `42a646e` 经验文在档，无需另沉淀。

## 附注（本班过程留痕）

一次性审计 harness（`tmp-audit-3c/`，用后即删）被并发 CEO 会话的收口
commit `42eda13` 以 add -A 误扫入仓——本判决 commit 随档移除该文件并于此
留痕。

## 判决

**通过**。五锚机器证据闭合（含 harness 直调解析复算），形制全合，暗债
无瞒报。首判、二判结论经本班独立复算一致成立。子票 0017 交付（5e9ce28）
另票另审（已 audit_pass，事件 652），不入本判决。
