# 票 req-9ce5 判决：通过（审计席·devin 独立实例·三判，2026-11-02）

- 审计 run：`68c8e82422bf398e`（shell=devin，独立实例）。触发＝deliver（事件 385，
  `docs/实现/设计-req-9ce5-票挂项目.md devin exit=0`，施工 run `ca93f994fca6da85`）。
- **缘起**：前两任审计皆席死未出哨兵——`47255ee4004b8c7d`（claude）已落
  `1-claude.md`（commit `495b2f6`）但净退漏哨兵（事件 401/402）；
  `fa7aa447152bc961`（claude）落 `2-claude.md` 后撞 API 429 限死（事件
  414–416）。本席**不信前判自述，全部锚亲手重取机器证据**——下表每条为
  本席跑出的命令输出，非转抄。`2-claude.md` 为死席遗档（文完而未 commit），
  本席收编入同 commit，不湮灭审计痕。
- 交付物：master commit `1bd1c79` 三文件 317 行纯新增——设计文档
  `docs/实现/设计-req-9ce5-票挂项目.md`（110 行）＋本票票面
  `docs/实现/票-req-9ce5-设计票.md`（74 行）＋子票票面
  `docs/实现/票-0015-票项目挂接.md`（133 行）。设计票交付形制＝文档 commit
  入仓（票面第 8 栏明定 ref＝设计文档 commit hash，先例 3ccd043/495b2f6
  同款——本票无交付支，`git branch -a` 无 `t/req-9ce5*`）。

## 逐锚对账（每条指向本席命令输出）

| 锚 | 机器证据（本席复算） | 判 |
|---|---|---|
| 1 设计档在＋三节名 | `git show 1bd1c79 --stat` 含该文件 110 行；`grep -c` → attachProject=**2**（L37「共用机械闸 `attachProject(ctx,…)`（五判，全机械）」＋L60）、挂接矩阵=**1**（§2 标题 L23「总体方案：挂接矩阵」）、自举序=**1**（§4 标题 L78「自举序（鸡生蛋的合法解）」） | **绿** |
| 2 子票票面在＋解析过 | `git show 1bd1c79` 含票-0015（133 行）；`grep "^### [0-9]"` → 1–8 八栏全在；sqlite3 `select length(spec)…where id='0015'` → **7046B**（spec 落库必经 `inject.ParseTicketMD`＋CheckSpec——created 事件 rowid 380 在＝闸过） | **绿** |
| 3 DB 0015 存在 spec 非空 | sqlite3 → `0015\|7046\|待审`；`meta phase --ticket 0015` → `0015 待审 frozen=false`（注：子票已推进——施工 run `e0b4355c7faac3a4` 已 deliver，事件 419 ref `a5b3aaa`，相位较前判时「施工中」前移，锚所断「存在且 spec 非空」仍立） | **绿** |
| 4 0015 派发真 | `meta events --ticket 0015` → 380 created／381 design_start／382 `dispatch 设计中→已派`／383 `claim 已派→施工中 e0b4355c7faac3a4`／413 progress／419 deliver——dispatch 事件与派发动作皆在事件史 | **绿** |
| 5 本票 spec 回填 | sqlite3 → `req-9ce5efb2e4ad\|2864\|审计中`；事件 rowid 379 `fill_spec`（施工日志：spec 520→2864B，admin_resp ok=true）；本票文件 `grep "^### [0-9]"` → 1–8 八栏全在 | **绿** |
| 验证命令（第 7 栏） | 本席全跑：`grep -c "attachProject\|挂接矩阵\|自举序"` → 2/1/1 非零；`meta phase --ticket 0015` 通；`meta events --ticket 0015` 通（socket 用绝对径 `/opt/meta/run/meta.sock`，与票载相对径同一 socket）。票载机器记录文件 `/tmp/treq-9ce5efb2e4ad*` 不存在——本票验证命令未载 tee 路径，机器证据＝网关事件史＋DB＋仓内 commit，如上表 | **绿** |
| 交付摘要四节形 | 施工日志 `run/logs/ca93f994fca6da85.log`：交付摘要四节在——新增/修改面（三文件＋commit `1bd1c79`）／根因（列在而无生产写径→全表空→闸拒服）／验证引（事件 379＋0015 立派claim）／下一步（流断复位＋sudoers）〔令#E4：摘要非证据，证据仍是上表〕 | **绿** |

## 范围核对

diff＝3 文件全量新增、零码改（`git show 1bd1c79` 无 `.go`/`.proto`），对第 3
栏范围内四条（设计档 commit／子票票面 commit＋立票／dispatch 派发／
fill_spec＋Deliver）一一对应；第 4 栏禁线「实现码」未碰。子票支
`t/0015-project-link`（worktree 在）现有提交 `a5b3aaa`——系 0015 自身交付，
属子票范围非本票蔓延。工作区除死席遗档 `2-claude.md` 外净（本 commit 收编后
全净）。

施工中两处非常规动作，核为非绕闸（依日志＋事件史）：①devin 壳行合并缺陷
（D-42 类）致哨兵行无法行首起——往进程 fd 1 直写独占行，帧仍经席位扫描→
网关闸→admin_resp，fill_spec/create_ticket/dispatch 三帧 ok=true 且事件
379/380/382 落痕，闸全过；②席位上行流断（sendPump 死）后经官方收养径
（per-seat UDS＋run_token 互证 Adopt）复位，一次性呼叫器仓外临时文件交付前
已删、未入 commit。

## 暗债对账（令#E6）

交付 diff `TODO|FIXME|XXX|绕过|hack` grep **零中**（退出码 1＝无匹配）。摘要
「下一步」申报两债：席位流断/sendPump 死＋devin 行合并（D-42 类缺陷本体未
修）、sudoers `/etc/sudoers.d/meta-runner` 待业主——与日志实况对上，非瞒报。

## 债项（移交，非否决依据）

1. **D-42 类缺陷本体**：devin 壳行合并＋sendPump 死流——施工与前任审计皆
   靠自救径完成，缺陷未修。宜立票或入经验库。
2. **per-project 配方注入**：注入机 Recipes 为配置静态列——设计 §5 记债，
   子票票面明「记债台账」。
3. **`DispatchAction.project` 路由表行为**：本批只留字段口不改表（设计 §5）。
4. **sudoers 行**：业主动作，票面两处已载。
5. 小疵（记录非否决）：第 8 栏「ref＝设计文档 commit hash」，deliver 哨兵
   实载文档**路径**非 hash——hash `1bd1c79` 由摘要点名且为该文件唯一触及
   commit，唯一可解；后续设计票哨兵直载 hash 更合票面（同 2-claude.md 所记）。

## 质量三问（令#E5）

- **最优解**：六写径共用一处机械闸 `attachProject`（设计 L37），非六处各判；
  复用既存 `db.SetTicketProject` 并顺手消 `SetProject`/`SetTicketProject`
  同义重复——复用现有、无新增间接层。
- **第一性**：确诊表逐条指到代码行（schema.go:11/exec.go:146-152/
  dispatch.go:94/113）；自举序 §4 正面解「部署需闸、闸需本功能」循环依赖
  （L78–92 五步钦定序），非 workaround 摞补丁。
- **洁净**：纯文档交付，diff 内无重复实现/死码/TODO/调试残渣。
- **禁线扫描**：attachProject 五判全属机械判（存在性/登记比对/相位集合成员/
  等值比对）——禁区「机械判合法」范畴；存量回填径 CLI→RPC→网关，不破唯一
  写者；无周期任务、无心跳、无 JSON/REST；AdminGateway 增 RPC 有
  RetroVerdict/IssueCeoToken 先例，总设计不动有据。

设计票——审＝核设计档在、子票立、派发真（票面首行语）：三者在，五锚全绿，
判**通过**。

## 可沉淀意见（令#E8，纯意见）

摘要「下一步」所载「席位流断→官方收养径（per-seat UDS＋run_token 互证）
复位」经三席接力复现，已是实证过的完整打法：**可沉淀**——一坑一文入
`docs/经验库/`，后续席位遇同类死流径用。另：审计席哨兵义务宜在轨文/提示
中加重——前两判皆绿却死于哨兵未发，质量闸空转两轮。
