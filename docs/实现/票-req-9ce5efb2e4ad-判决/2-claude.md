# 票 req-9ce5 判决：通过（审计席·claude 独立实例·二判，2026-11-02）

- 审计 run：`fa7aa447152bc961`（shell=claude，独立实例）。触发＝deliver（事件 385，
  `docs/实现/设计-req-9ce5-票挂项目.md devin exit=0`，施工 run `ca93f994fca6da85`）。
- **缘起**：前审计 run `47255ee4004b8c7d` 已落判决档 `1-claude.md`（commit
  `495b2f6`）但净退**漏发 META_VERDICT 哨兵**＝席死（事件 401/402），网关重派
  本席。本席**不信前判自述，全部锚独立重取机器证据**——下表每条为本席亲手
  跑出的命令输出，非转抄。
- 交付物：master commit `1bd1c79` 三文件 317 行新增——设计文档
  `docs/实现/设计-req-9ce5-票挂项目.md`（133 行）＋本票票面
  `docs/实现/票-req-9ce5-设计票.md`（74 行）＋子票票面
  `docs/实现/票-0015-票项目挂接.md`（110 行）。设计票交付形制＝文档 commit
  入仓（第 8 栏明定，先例 3ccd043 决策落账径同款）。

## 逐锚对账（每条指向本席命令输出）

| 锚 | 机器证据（本席复算） | 判 |
|---|---|---|
| 1 设计档在＋三节名 | `git show 1bd1c79 --stat` 含该文件 133 行；`grep -c` → attachProject=**2**（L37「共用机械闸 `attachProject(ctx,…)`（五判，全机械）」＋L60）、挂接矩阵=**1**（§2 标题 L23「总体方案：挂接矩阵」）、自举序=**1**（§4 标题 L78「自举序（鸡生蛋的合法解）」） | **绿** |
| 2 子票票面在＋解析过 | `git show 1bd1c79 --stat` 含票-0015（110 行）；sqlite3 → `0015\|7046\|施工中`（spec 为 marshaled proto——唯 `inject.ParseTicketMD` 成功才可能有）；解析径在 `internal/seat/seat.go:533`（`ParseTicketMD(b)` 失败不立）；created 事件 rowid 380 在 | **绿** |
| 3 DB 0015 存在 spec 非空 | sqlite3 同上＋`meta phase --ticket 0015` → `0015 施工中 frozen=false`；`meta events --ticket 0015` 可查 | **绿** |
| 4 0015 派发真 | `meta events --ticket 0015` → 380 created／381 design_start／382 `dispatch 设计中→已派`／383 `claim 已派→施工中 e0b4355c7faac3a4`（claim 者＝spawn 出的施工 run）；提示文件 `/tmp/prompt-0015-build.md` 在（1.4K——注入机未开期带提示文件，第 3 栏形制）；施工日志 L13 `seat: admin_resp ok=true dispatched 0015 施工` | **绿** |
| 5 本票 spec 回填 | sqlite3 → `req-9ce5efb2e4ad\|2864\|审计中`；事件 379 `fill_spec`（spec 520→2864B）；回填径 `internal/gateway/ceo.go:627-630`——`ParseTicketMD` 失败即 `resp(false)` 拒，事件在解析/marshal/UpdateSpec 全成后才落 | **绿** |
| 验证命令（第 7 栏） | 本席全跑：grep 三计数非零（2/1/1）、`meta phase --ticket 0015` 通、`meta events --ticket 0015` 通（socket 用绝对路径 `/opt/meta/run/meta.sock`，与票载相对径同一 socket） | **绿** |
| 交付摘要四节形 | 施工日志 `run/logs/ca93f994fca6da85.log` L16–21：新增/修改面（三文件＋commit `1bd1c79`）／根因（列在而无生产写径→全表空→闸拒服）／验证引（事件 379＋0015 立派claim）／下一步（流断复位＋sudoers）——四节形在〔令#E4：摘要非证据，证据仍是上表〕 | **绿** |

## 范围核对

diff＝3 文件全量新增、零码改，对第 3 栏范围内四条（设计档 commit／子票票面
commit＋立票／dispatch 派发／fill_spec＋Deliver）一一对应；第 4 栏禁线
「实现码」未碰（diff 无 `.go`/`.proto` 文件）。`t/0015-project-link` 支已预立
（worktree 在）但零提交——合子票「不并 master」形制。工作区净
（`git status --short` 空）。

施工中两处非常规动作，核为非绕闸（依日志＋事件史，非自述）：
①devin 壳行合并缺陷（D-42 类）致哨兵行无法行首起——施工往 devin 进程
fd 1 直写独占行；帧仍经席位扫描→网关闸→admin_resp，fill_spec/create_ticket/
dispatch 三帧 admin_resp ok=true 且事件 379/380/382 落痕——闸全过，非旁路。
②席位上行流断（sendPump 死）后走官方收养径（per-seat UDS＋run_token 互证
Adopt）复位；一次性呼叫器临时文件交付前已删、未入 commit、树净。

## 暗债对账（令#E6）

交付 diff 内 `TODO|FIXME|XXX|绕过|临时|hack` grep **零中**。摘要「下一步」
申报两债：席位流断/sendPump 死＋devin 行合并（D-42 类缺陷本体未修）、
sudoers `/etc/sudoers.d/meta-runner` 待业主——与日志实况对上，非瞒报。

## 债项（移交，非否决依据）

1. **D-42 类缺陷本体**：devin 壳行合并＋sendPump 死流——本票靠 fd 直写＋
   收养径自救，缺陷未修。宜立票或入经验库。
2. **per-project 配方注入**：注入机 Recipes 为配置静态列——设计 §5 记债，
   子票票面明「记债台账」。
3. **`DispatchAction.project` 路由表行为**：本批只留字段口不改表（设计 §5）。
4. **sudoers 行**：业主动作，票面两处已载。
5. 小疵（记录非否决）：第 8 栏「ref＝设计文档 commit hash」，deliver 哨兵
   实载文档**路径**非 hash——hash `1bd1c79` 由摘要点名且为该文件唯一触及
   commit，唯一可解；后续设计票哨兵直载 hash 更合票面。

## 质量三问（令#E5）

- **最优解**：六写径共用一处机械闸 `attachProject`（设计 L37），非六处各判；
  复用既存 `db.SetTicketProject` 且确诊同义重复去重留一——复用现有、无新
  增间接层。
- **第一性**：确诊指到行（schema 列在而无生产写径）；自举序 §4 正面解
  「部署需闸、闸需本功能」循环依赖（L78–92 五步钦定序），非 workaround。
- **洁净**：纯文档交付，无重复实现/死码/调试残渣入 diff（grep 零中）。

设计票——审＝核设计档在、子票立、派发真（票面首行语）：三者在，判通过。

## 可沉淀意见（令#E8，纯意见）

摘要「下一步」所载「席位流断→官方收养径（per-seat UDS＋run_token 互证）
复位」是完整可复用打法（日志 L8 全程：确诊死流→读 Adopt 协议→握手→冲刷
验证）：**可沉淀**——一坑一文入 `docs/经验库/`，后续席位遇同类死流径用。
