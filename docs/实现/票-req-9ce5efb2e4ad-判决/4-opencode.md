# 票 req-9ce5efb2e4ad 判决：通过（审计席·opencode 独立实例，2026-11-02）

- 审计 run：`07a866bb1f4bf8a9`（shell=opencode，独立实例，本席）。触发＝deliver（事件 385，
  `docs/实现/设计-req-9ce5-票挂项目.md devin exit=0`，施工 run `ca93f994fca6da85`＝CTO-A 设计席）。
- 前任审计接力（供对照，不信自述）：`47255ee4004b8c7d`（claude）落 `1-claude.md`（commit
  `495b2f6`）漏发哨兵死席（事件 401/402）；`fa7aa447152bc961`（claude）撞 API 429 限死
  （事件 414–416）；`68c8e82422bf398e`（devin）落 `3-devin.md`＋收编死席遗档 `2-claude.md`
  （commit `eee536a`），后席死 respawn（事件 426/427）。D-42 哨兵 JSON 失明已修（扫描点移入
  适配器解析后文本流，决策 60/决策 46 闭合），本次席位 opencode 哨兵扫描恢复可用，本席
  独立重取机器证据后出本档。
- 交付物：master commit `1bd1c79` 三文件 317 行新增——设计文档
  `docs/实现/设计-req-9ce5-票挂项目.md`（110 行）＋本票票面 `docs/实现/票-req-9ce5-设计票.md`
  （74 行）＋子票票面 `docs/实现/票-0015-票项目挂接.md`（133 行）。设计票交付形制＝文档
  commit 入仓＋摘要四节（令#E4：摘要非证据，证据仍是 diff＋机器记录）。

## 逐锚对账（每条指向机器输出）

| 锚 | 机器证据（本席复算） | 判 |
|---|---|---|
| 1 设计档在＋三节名 | `git show --stat 1bd1c79` → 三文件/317 增；设计档在 commit 内；`grep -c "attachProject\|挂接矩阵\|自举序"` → **attachProject=2**（L37「共用机械闸 `attachProject(ctx,…)`（五判，全机械）」＋L60）、**挂接矩阵=1**（§2 L23）、**自举序=1**（§4 L78）；§7 验证命令原样 grep → **4**（rc=0） | **绿** |
| 2 子票票面在＋解析过 | `git show --name-only --format="" 1bd1c79` 含票-0015（八栏 `grep "^### "` → 1–8 全在）；sqlite3 `select length(spec) … where id='0015'` → **7046**；created 事件 rowid 380 在（`meta events --ticket 0015`） | **绿** |
| 3 DB 0015 存在 spec 非空 | sqlite3 → `0015\|7046\|待审`；`meta phase --ticket 0015` → `0015 待审 frozen=false`；事件 380/382/383（dispatch/claim）在 | **绿** |
| 4 0015 派发真 | `meta events --ticket 0015` → 380 created／381 design_start／382 `dispatch 设计中→已派`／383 `claim 已派→施工中 e0b4355c7faac3a4`；提示文件 `/tmp/prompt-0015-build.md`（1.8K）；施工日志 `seat: admin_resp ok=true dispatched 0015 施工` | **绿** |
| 5 本票 spec 回填 | sqlite3 → `req-9ce5efb2e4ad\|2864\|审计中`；事件 rowid 379 `fill_spec`（spec 520→2864B）；`seat: admin_resp ok=true spec 已回填`；deliver 事件 385 ref=`docs/实现/设计-req-9ce5-票挂项目.md devin exit=0` | **绿** |
| 验证命令（第 7 栏） | `grep -c "attachProject\|挂接矩阵\|自举序"` → 4（rc=0）；`meta phase --ticket 0015` → `0015 待审 frozen=false`；`meta events --ticket 0015` → rc=0 | **绿** |
| 交付摘要四节形 | 施工日志 `run/logs/ca93f994fca6da85.log` L18–21：新增/修改面（三文件＋commit `1bd1c79`）／根因（列在而无生产写径→全表空→闸拒服）／验证引（事件 379＋0015 立派claim）／下一步（流断复位＋sudoers）〔令#E4〕 | **绿** |

## 范围核对

diff＝commit `1bd1c79` 三文件全量新增、零码改（`.go`/`.proto` 无任何改动），对设计票第 3
栏范围内（设计档 commit／子票票面 commit＋立票／dispatch 派发／fill_spec＋Deliver）一一对应；
第 4 栏禁线「实现码」未碰；子票交付 commit `a5b3aaa`（0015 自身交付）属子票范围非蔓延。
工作区净（`git status --short` 空）。

施工中两处非常规动作，核为非绕闸（依日志＋事件史＋DB，非自述）：①devin 壳行合并缺陷
致哨兵行无法行首起——经 fd 直写独占行 + 官方收养径（per-seat UDS＋run_token 互证 Adopt）
自救，三帧 fill_spec/create_ticket/dispatch 闸全过（admin_resp ok=true 且事件 379/380/382
落痕）；②一次性 Adopt 呼叫器为仓外临时文件，交付前已删、未入 commit、树净。闸未旁路。

## 暗债对账（令#E6）

交付 diff 内 `TODO|FIXME|XXX|绕过|临时|hack` grep **零中**（rc=1）。摘要「下一步」申报
两债——席位流断/sendPump 死＋devin 行合并（D-42 类缺陷本体未修）与 sudoers
`/etc/sudoers.d/meta-runner` 待业主——与台账/日志实况对上，非瞒报。

## 债项（移交，非否决依据）

1. **D-42 类缺陷本体**：devin 壳行合并＋sendPump 死流——施工与前任审计皆靠自救径完成，
   缺陷未修。宜立票或入经验库。
2. **per-project 配方注入**：注入机 Recipes 为配置静态列——设计 §5 记债，子票票面明「记债」。
3. **`DispatchAction.project` 路由表行为**：本批只留字段口不改表（设计 §5）。
4. **sudoers `/etc/sudoers.d/meta-runner`**：业主动作，票面两处已载。

## 质量三问（令#E5）

- **最优解**：六写径共用一处机械闸 `attachProject`（设计 L37），非六处各判；复用既存
  `db.SetTicketProject` 且确诊顺手清同义重复（`db.SetProject` ticket.go:235／`SetTicketProject`
  project.go:84 同义，去重留一）。复用现有、无新增间接层。
- **第一性**：确诊表逐条指到代码行（`internal/db/schema.go:11` 列在而无生产写径／
  `internal/gateway/exec.go:146-152` 拒点／`dispatch.go:94/113` 锁退化/跳过）——直击
  「列在而无生产写径」根因；自举序 §4 正面解「部署需闸、闸需本功能」循环依赖，非绕过。
- **洁净**：纯文档交付，diff 无重复实现/死码/调试残渣（grep 零中）。

设计票——审＝核设计档在、子票立、派发真（票面首行语）：三者在，五锚全绿，**判通过**。

## 可沉淀意见（令#E8，纯意见）

摘要「下一步」所载「席位流断→官方收养径（per-seat UDS＋run_token 互证）复位」是完整可
复用打法：**可沉淀**——一坑一文入 `docs/经验库/`。

## 判决

**通过**——交付（设计文档 commit `1bd1c79`＋摘要四节）与票面锚（设计档在/三节名、子票立/
派发真/本票回填、验证命令、摘要四节）闭合；范围、形制、暗债、质量三问均对账无误；交付
零码改、无 TODO/绕过/半成品；闸全过非旁路。

**下一步（系统）**：本席哨兵 `META_VERDICT:pass:docs/实现/票-req-9ce5efb2e4ad-判决/4-opencode.md`
回流 → gateway `handleDeliver` audit_pass 边 → 相位 `审计中→待部署`（设计票交付闭环）。
债务随后续子票链（0015 部署闸端到端首验）处置。
