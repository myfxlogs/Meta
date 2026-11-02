# 票 0037-p2 判决：通过（审计·devin，2026-11-02，run b3bdabb2d729e2a6）

交付：支 `t/0037-p2-precedent-inject` @ `61bf9de`（`origin/` 同指 61bf9de；
merge-base＝master 顶 `f119f11`，基于 master 顶成立）。
机器记录：`/opt/meta/run/artifacts/10b9590e9699e798/atif.json`（施工 run 逐
步含退码）；`/tmp/t0037-p2*.machine-record.txt` 不存在，以 atif 为准。

## 逐锚对账

**锚1**（`go test` 绿＋三分支）：atif step25 `go test ./internal/inject/ -v`
→ `Go test: 13 passed in 1 packages` exit=0；step26 `go build ./... && go vet
./internal/inject/ ./internal/gateway/ && go test -run TestAssemble` →
`4 passed` exit=0（链式退码 0＝build/vet 同绿）。三分支由新增四用例覆盖：
`TestPrecedent_命中插段`（产物含 `## 判例前置`＋命中路径行＋段位置
i5<ip<i6 断言，precedent_test.go L57-82）、`TestPrecedent_无命中无段`
（L85-96）、`TestPrecedent_超三截断`（段内恰 3 条 `- docs/` 行＋top3 精确
断言＋指针行，L99-139）。

**锚2**（对抗锚，删/翻必红）：fixture 票 Goal「修复哨兵在 JSON 包裹下丢
行」→产物含 `## 判例前置` 且含 `- docs/经验库/2026-11-02-哨兵三壳失明
谱-JSON包裹与行合并.md — `（precedent_test.go L68-72）。变异实证：atif
step27 把 `len(hits) > 0` 翻成 `== 0` → `命中插段`/`无命中无段`/`超三截
断` 三用例如期转红，还原后 `13 passed` exit=0。

**锚3**（非施工 Role 不含段）：`TestPrecedent_非施工不插`
（precedent_test.go L142-155）＋inject.go 插段整块关在
`if in.Role == "施工"` 内（inject.go L144-159）——其他 Role 产物字节不
变，禁线成立。

**锚4**（存量不破）：包内 13 用例全绿＝8 既有（inject_test.go 8 个
Test 函数）＋1（t0030_test.go）＋4 新增；inject.go diff 为纯增 19 行
（`+19 -0`），不改任何既有行；`TestAssemble_DeliverSentinelLastLine` 仍
在（inject_test.go L170）；实证档末行 `META_DELIVER:<ref>` 原样（with-
hits 末两行实证）。

**锚5**（两份实证档在案）：`run/artifacts/10b9590e9699e798/p0037-
inject-with-hits.md` L205-210——段内恰 3 行 `- docs/经验库|技能库/<名>.
md — <摘要>`（命中真库档）＋指针行含钦定文本「两库全量索引」「四/五」；
`p0037-inject-no-hits.md` grep「判例前置」计数＝0。atif step30-32 机器复
核输出同结论。

## 范围核对

diff 七件全在票面范围内：`internal/inject/{inject.go,precedent.go,
precedent_test.go}`；testdata 三个 fixture 正文「fixture→样例」清撞词
（internal/inject 包内测试面，非真库 .md 本体——不禁）；`docs/经验库/`
新增坑档一件（令#E1 遇坑即写义务，非两库格式改造）。真库 .md 零改动；
无模型/embedding；无定时器/缓存/轮询（同步随 Assemble 完成）；七段编号
模板与 META_DELIVER 末行未动；无非施工 Role 产物变化。
（网关前置闸范围核对报告本票未见落档——范围核对由本席直接 diff 复核。）

## 债项（暗债对账）

diff 内零 TODO/绕过/半成品（临时 dump 程序交付前已 `rm -rf tmpdump`，不
在 diff——atif step33）。摘要「下一步」申报三点与 diff 全对上：

- 摘要口径「首个非空非标题行」会把「沉淀日：…」行带进产物——
  with-hits L207 摘要行实证（钦定口径，非缺陷）；
- 评分＝去重交集计数、不计词频——precedent.go `overlap()` 与票面「关键
  词重叠计数」口径一致；
- 支基于 master 顶 f119f11（含他席新坑档）——merge-base 实证一致。

无瞒报。

**观察项**（非否决，归 CTO 裁量）：分词对 CJK/ASCII 交界不产跨界二元
（`哨兵JSON`→{哨兵, json}，非示例的 兵J/JSO 式）——票附括号示例
「JSO/SON」为三字符与正文「2 字滑窗」自相矛盾，按正文钦定口径本实现
成立；若示例属意跨界切窗，是票面措辞待裁而非施工偏差。

## 质量三问

最优：`precedents` stdlib 纯函数＋组装链内联插段（inject.go L144-159），
无更简等价形制可指；第一性：检索只发生在派单组装时刻、同步完成，无旁
路间接层；洁净：无死码/TODO/重复实现/调试残渣，注释点名对抗锚变异位
可溯。

## 可沉淀打法候选：可沉淀

「接入既有 fixture 前先证 票关键词×扫描面 零重叠」「对抗锚写进注释提
名变异位、变异实测一次再走」「无命中用例选词先 grep 扫描面」三点有跨
票复用价值，且坑档已随票入经验库
（`docs/经验库/2026-11-02-判例检索fixture撞词-选词先证零重叠.md`）。

## 结论

五锚全绿、范围净、禁线未碰、暗债零瞒报——**通过**。
