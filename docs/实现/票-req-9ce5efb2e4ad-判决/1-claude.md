# 票 req-9ce5 判决：通过（审计席·claude 独立实例，2026-11-02）

- 审计 run：`47255ee4004b8c7d`（shell=claude，独立实例），触发＝deliver（事件 385，
  `docs/实现/设计-req-9ce5-票挂项目.md devin exit=0`，施工 run `ca93f994fca6da85`）
  → 待审→审计中（事件 386，本席起）。
- 施工 run：`ca93f994fca6da85`（CTO-A 设计席，devin 壳）。
- 交付物：master commit `1bd1c79` 三文件 317 行新增——设计文档
  `docs/实现/设计-req-9ce5-票挂项目.md`（133 行）＋本票票面八栏
  `docs/实现/票-req-9ce5-设计票.md`（74 行）＋子票票面
  `docs/实现/票-0015-票项目挂接.md`（110 行）。设计票交付形制＝文档 commit
  入仓（票面第 8 栏：ref＝设计文档 commit hash——与施工票「不并 master」
  形制不同，票面明定，先例同 3ccd043 决策落账径）。

## 逐锚对账（每条指向机器输出）

| 锚 | 机器证据 | 判 |
|---|---|---|
| 1 设计档在＋三节名 | `git show 1bd1c79 --stat` 含该文件＋133 行；`grep -c` → attachProject=2、挂接矩阵=1（§2 标题）、自举序=1（§4 标题），§2-L37「共用机械闸 attachProject…（五判，全机械）」 | **绿** |
| 2 子票票面在＋解析过 | `git show 1bd1c79 --name-only` 含票-0015；sqlite3 `select length(spec) from ticket where id='0015'` → **7046B**；立票径机器链：`cmd/meta/main.go:448` ParseTicketMD → `internal/gateway/inject.go:30` CheckSpec 不过即拒 → created 事件 rowid 380 在＝闸过；票面 MD ### 1–8 八栏全在 | **绿** |
| 3 DB 0015 存在 spec 非空 | sqlite3 → `0015\|7046\|施工中`；`meta phase --ticket 0015` → `0015 施工中 frozen=false` | **绿** |
| 4 0015 派发真 | `meta events --ticket 0015` → rowid 382 `dispatch 设计中→已派`＋rowid 383 `claim 已派→施工中 e0b4355c7faac3a4`；派发提示文件 `/tmp/prompt-0015-build.md` 在（1447B）；施工日志 `seat: admin_resp ok=true dispatched 0015 施工` | **绿** |
| 5 本票 spec 回填 | sqlite3 → `req-9ce5efb2e4ad\|2864\|审计中`；事件 rowid 379 `fill_spec`（施工日志：spec 520→2864B，`seat: admin_resp ok=true spec 已回填`）；fill_spec 径 `ceo.go:517` CheckSpec==nil 闸过才有事件 | **绿** |
| 验证命令（第 7 栏） | 本席全跑：grep 三节名计数非零、`meta phase --ticket 0015` 通、`meta events --ticket 0015` 通 | **绿** |
| 交付摘要四节形 | 施工日志 `/opt/meta/run/logs/ca93f994fca6da85.log` 尾部：新增/修改面（三文件＋commit 1bd1c79）／根因／验证引／下一步——四节形在（令#E4：摘要非证据，证据仍是上表机器输出） | **绿** |

## 范围核对

diff＝3 文件全量对票面第 3 栏范围内三条一一对应（设计文档 commit／子票票面
commit＋立票／本票八栏回填）。零码改——范围外第 1 条「实现码」未碰；`t/0015-
project-link` 支已预立（worktree 在）但零提交，与子票「不并 master」形制一致。
工作区净（git status clean）。

施工中两处非常规动作，核为非绕闸：①席位上行流断（sendPump 死）后走官方
收养径（per-seat UDS＋run_token 互证 Adopt）复位——日志明「走官方径，不绕
任何闸」，复位后 fill_spec/create_ticket/dispatch 三帧全经网关闸回 admin_resp
ok=true，事件 rowid 379/380/382 落痕；②一次性 Adopt 呼叫器为仓外临时文件，
交付前已删、工作区净，未入 commit。

## 暗债对账（令#E6）

diff 内无 TODO/绕过标记。明债申报与摘要「下一步」对账：摘要申报流断缺陷
（D-42 类：devin 行合并＋sendPump 死）与 sudoers 待业主——属实对上，非瞒报。
设计 §5 记的非目标项见下节债项。

## 债项（移交，非否决依据）

1. **D-42 类缺陷本体**：devin 壳行合并（哨兵行无法行首起）＋席位流断后
   sendPump 死——本票靠 fd 直写＋收养径自救，缺陷未修。宜立票或入经验库。
2. **per-project 配方注入**：注入机 Recipes 为配置静态列——设计 §5 记债，
   子票票面明「记债台账」，台账条待子票链落。
3. **`DispatchAction.project` 路由表行为**：字段留口本批不改表（设计 §5）。
4. **sudoers `/etc/sudoers.d/meta-runner`**：业主前置动作，票面已载两处。

## 质量三问（令#E5）

- **最优解**：六写径共用一处机械闸 attachProject（非六处各判）；复用既存
  `db.SetTicketProject` 且确诊顺手清同义重复（SetProject/SetTicketProject
  去重留一）——复用现有、无新增间接层。
- **第一性**：确诊表逐条指到代码行（schema.go:11/exec.go:146-152/dispatch.go:94/
  113），直击「列在而无生产写径」根因；自举序 §4 正面解「部署需闸、闸需本
  功能」的循环依赖而非绕过。
- **洁净**：无重复实现新增（反消一处既存重复）、无死码、无调试残渣入 diff。

设计票——审＝核设计档在、子票立、派发真（票面首行语）：三者在，判通过。

## 可沉淀意见（令#E8，纯意见）

摘要「下一步」所载「席位流断→官方收养径（UDS＋run_token）复位」是完整可
复用打法：**可沉淀**——一坑一文入 `docs/经验库/`（流断确诊→收养握手→冲刷
验证），后续席位遇同类死流可径用。
