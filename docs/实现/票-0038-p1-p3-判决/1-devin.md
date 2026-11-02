# 票-0038-p1-p3 判决（rollback_cmd 双源漂移·设计票）——审计·devin

交付：deliver_ref=`917bbda`（直落 master，父 `61f8a47`＝当时 master 顶；
设计票无独立交付支——docs commit 直并主线与先例同款）。基 `61f8a47`。
机器记录：`/tmp/t0038-p1-p3*.machine-record.txt` 不存在；施工 atif 档
`run/artifacts/65c12b70c2e57141/{atif.json,sentinel.log}` 在案；票面 §7
验证命令全为只读取证（grep/meta/sqlite/git），本席逐条亲跑取证，输出
原文引于下。

## 判决：通过

## 逐锚对账

- **锚①**（设计档在仓＋六要素）：`git show 917bbda --name-only` 含
  `docs/实现/设计-0038-p1-p3-配方行真源归一.md`（143 行）。亲读：
  §0 实证链复核（逐条亲核＋现状变更注明）、§1 真源归一裁断（第三解
  ＋三层理由）、§2/§3 机制边界（造一/不造一）、§4 时序与 0041 衔接、
  §5 残险与判词、§7 子票锚预览——六要素齐。票面 grep
  `真源|回种|漂移` 计数＝27。**PASS**
- **锚②**（子票票面八栏零缺）：`docs/实现/票-0046-配方行真源归一.md`
  同 commit 入仓（137 行），§1 目标→§8 交付形制＋附＋判决史全在；
  旁证＝0046 派发链无 `inject_failed`（事件 16238 dispatch→16239
  claim 直达成，ParseTicketMD 已机械过）。**PASS**
- **锚③**（DB 票 0046 存在·spec 非空·project=meta）：
  `meta phase --ticket 0046`→`施工中 frozen=false`；sqlite 亲验
  `0046|meta|length(spec)=7572`；`project_set` 事件 16236 记
  `project=meta`。**PASS**
- **锚④**（0046 事件史含 dispatch/claim）：事件 16238
  `dispatch 设计中→已派`、16239 `claim 已派→施工中 f4e709cd2774ca3c`
  ——派发真发生且施工席已上身。**PASS**
- **锚⑤**（本票 spec 回填）：事件 16234 `fill_spec 八栏回填`
  （65c12b70c2e57141）；sqlite 亲验 `0038-p1-p3|meta|length(spec)=4305`；
  `票-0038-p1-p3-设计票.md` §1-§8＋附＋判决史零缺。**PASS**

范围内其余申报项复核：需求原文档 `票-0038-p1-p3-rollback_cmd双源漂移.md`
（121 行）同 commit 入仓——立案时暂挂项已按票补落；sentinel.log 记
`META_ADMIN:create_ticket|0046|…`＋`META_ADMIN:dispatch|0046|施工|…`
＋`META_DELIVER:917bbda`，与 DB/事件史逐一对上。**PASS**

## 偏差/债项

- diff 四件全在 `docs/实现/`（需求原文档/设计票回填档/设计档/子票票面），
  `+495 -0` 纯增；零码面——禁线「设计票不施工（不动 project.go/
  gateway.go）」未碰；范围外零蔓延。
- diff 内 grep `TODO|FIXME|XXX|HACK|绕过|半成品` 零命中（exit=1）。
- 摘要「下一步」申报两条对账：①「0046 施工链」——0046 现 `施工中`
  （claim f4e709cd）属实；②「脏树处置已由 0038-p1 判决流收口」——
  事件 15838 `deploy_failed`（stub 未 abort 如实记）→ master `e21d5e7`
  「解冲突并完成合并：0038-p1 支并入」在史；本席亲验
  `git rev-parse -q --verify MERGE_HEAD` 解析失败、`git status` 无 UU。
  **无瞒报。**
- **观察项**（非否决，记档供 CEO 裁）：真源归一取第三解「config 文面
  钦定源＋DB 启动投影」，非需求票 §3 所给二选字面值——然票面同节授
  「方向裁量归施工/CTO-A」且终极目标只钦「消灭歧源」；设计档 §1 对三
  解取舍给三层具体理由（DB 为源＝配置成死文案歧义更深；逐步回读＝热
  径解析＋丢审计语义），设计票附节已预先披露此裁断——属透明申报的
  裁量行使，歧源消灭成立，不构成锚缺口。

## 独立复算

- 票面 §7 命令本席亲跑（只读取证）：
  `grep -c "真源\|回种\|漂移" 设计档`→27；
  `meta phase 0046`→施工中；`meta events 0046`→created/project_set/
  design_start/dispatch/claim/adopted×2 全链；
  `meta events 0038-p1-p3 | tail`→fill_spec 16234→deliver 16244
  （917bbda exit=0）→audit_start 16250。
- sqlite 亲验：`0038-p1-p3` spec=4305B、`0046` spec=7572B，project
  均 meta——与 atif step47 施工自验输出（同两数）互证一致。
- `git show 917bbda`：四文件恰为票面申报四件，commit 前缀
  `[角色:CTO][0038-p1-p3]` 合 `[角色:X][票]` 史例；交付摘要五节
  （新增/修改面、根因、裁断、验证引、下一步）覆盖票面四节制。
- 质量三问：最优——复用既有 `UpsertProject` ON CONFLICT 全列覆写
  ＋漂移比对＋既有事件径，零新 RPC/CLI（机制二钦定不造，理由在 §3），
  无更简等价形制可指；第一性——直击「改文件≠改运行时」结构根因，
  漂移消灭锚定在启动边界（部署必 restart→回种闭环）非补丁摞间接层；
  洁净——纯文档交付，无死码/残渣可生。

## 可沉淀打法候选：可沉淀

「裁量项取票给选项外第三解时：设计档立节给取舍理由＋票附预先披露」
一节为本次交付自带示范——把裁断做成可审的明账而非埋进结论，跨票
（尤其拆票/设计票）可复用。

## 结论

五锚全绿、范围内申报项逐项对上机器记录、范围净、禁线未碰、暗债零
瞒报——**通过**。
