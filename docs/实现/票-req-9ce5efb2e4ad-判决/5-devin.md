# 票 req-9ce5efb2e4ad 判决：通过（审计席·devin 独立实例·五判档，2026-11-02）

- 审计 run：`541027ffd9fcfb0b`（shell=devin，独立实例，本席）。身份实证：事件 594
  `audit_start 待审→审计中` actor=541027ffd9fcfb0b，且 `run/logs/541027ffd9fcfb0b.log`
  文本与本席输出逐字一致。触发＝audit_dead（前任席死）。
- **缘起**：本席是第六任审计。前五任接力——①`47255ee4`（claude）落 `1-claude.md`
  （commit `495b2f6`）漏哨兵死（事件 401/402）；②`fa7aa447`（claude）落 `2-claude.md`
  撞 API 429 死（414–416）；③`68c8e824`（devin）落 `3-devin.md`＋收编 2 档（`eee536a`）
  席死（426/427）；④`07a866bb`（opencode）落 `4-opencode.md`（`aa47322`）漏哨兵死
  （491/492）；⑤`dd1fbec4`（opencode）未落档即死（518–520）。事件 521
  `shells_exhausted 角色 审计 全部候选用尽` 后，574/590 `spawn_needed 对账补发:审计`
  → 594 本席起。本席**不信前判自述，全部锚独立重取机器证据**。
- 交付物：master commit `1bd1c79` 三文件 317 行纯新增——子票票面
  `docs/实现/票-0015-票项目挂接.md`（133 行）＋本票票面
  `docs/实现/票-req-9ce5-设计票.md`（74 行）＋设计档
  `docs/实现/设计-req-9ce5-票挂项目.md`（110 行）。设计票交付形制＝文档 commit 入仓
  （票面第 8 栏明定 ref＝设计文档 commit hash；先例 3ccd043/495b2f6/eee536a/aa47322
  同款——本票无交付支，`git branch -a` 无 `t/req-9ce5*`）。
- 附记：`1/2-claude.md` 把设计档与票-0015 行数记反（133↔110）——以
  `git show --stat` 实名为准：0015=133 行、设计档=110 行。微疵不撼两档结论。

## 逐锚对账（每条指向本席机器输出）

| 锚 | 机器证据（本席复算） | 判 |
|---|---|---|
| 1 设计档在＋三节名 | `git show 1bd1c79 --stat` 含设计档（110 行）；`grep -c "attachProject\|挂接矩阵\|自举序"` → **4**（rc=0）；`grep -n` → L23「## 2. 总体方案：挂接矩阵」、L37「共用机械闸 `attachProject(ctx,…)`（五判，全机械）」＋L60、L78「## 4. 自举序（鸡生蛋的合法解）」 | **绿** |
| 2 子票票面在＋解析过 | `ls` → `docs/实现/票-0015-票项目挂接.md` 在（7.3K）；`grep "^### [0-9]"` → ### 1–8 八栏全在；sqlite3 `length(spec) where id='0015'` → **4712 非空**；落库必经 `inject.ParseTicketMD`（`internal/seat/seat.go:533`）＋`CheckSpec`（`internal/gateway/inject.go:30`），created 事件 rowid 380 在 | **绿** |
| 3 DB 0015 存在 spec 非空 | sqlite3 → `0015\|text\|4712\|审计中`；`meta phase --socket /opt/meta/run/meta.sock --ticket 0015` → `0015 审计中 frozen=false`（子票已推进：自身交付 `a5b3aaa`、audit_start 事件 595——相位较交付时前移，锚所断「存在且 spec 非空」仍立） | **绿** |
| 4 0015 派发真 | `meta events --ticket 0015` → 380 created／381 design_start／382 `dispatch 设计中→已派`／383 `claim 已派→施工中 e0b4355c7faac3a4`；派发提示文件 `/tmp/prompt-0015-build.md` 在（1447B，ls 实证）；施工日志 `seat: admin_resp ok=true dispatched 0015 施工` | **绿** |
| 5 本票 spec 回填 | sqlite3 ticket_event rowid **379** → `fill_spec` actor `ca93f994fca6da85` note「八栏回填」；本票文件 `grep "^### [0-9]"` → ### 1–8 全在；sqlite3 → `req-9ce5efb2e4ad\|text\|1618` 非空（2882B）；回填径 `internal/gateway/ceo.go:517` `CheckSpec==nil` 闸＋627-635 `ParseTicketMD→UpdateSpec`——事件在闸过才落 | **绿** |
| 验证命令（第 7 栏） | 本席全跑：`grep -c` →4 rc=0；`meta phase --ticket 0015` rc=0；`meta events --ticket 0015` rc=0（socket 用绝对径 `/opt/meta/run/meta.sock`，与票载相对径同一 socket）。票载机器记录 `/tmp/treq-9ce5efb2e4ad*` **不存在**（ls rc=2）——本票验证命令未载 tee 路径，机器证据＝网关事件史＋DB＋仓内 commit，如上表 | **绿** |
| 交付摘要四节形 | 施工日志 `run/logs/ca93f994fca6da85.log` 尾部四节在：新增/修改面（三文件＋commit `1bd1c79`）／根因（列在而无生产写径→全表空→闸拒服）／验证引（事件 379＋0015 立派claim）／下一步（流断复位＋sudoers）〔令#E4：摘要非证据，证据仍是上表〕 | **绿** |

## 范围核对

diff＝commit `1bd1c79` 三文件全量新增、零码改（无 `.go`/`.proto`），对第 3 栏范围内
四条（设计档 commit／子票票面 commit＋立票／dispatch 派发／fill_spec＋Deliver）一一
对应；第 4 栏禁线「实现码」未碰。子票 `t/0015-project-link` 支上 `b7335de`/`a5b3aaa`
系 0015 自身交付，属子票范围非本票蔓延。当前工作区 `M bin/meta`、`M bin/meta.prev`、
`M config/caster.textproto`——`4-opencode.md` 时代树净（其 §范围核对），此三改为
交付后他线动作（疑 0015 部署自举在途），非本票交付面；本席仅加本档，不碰。

施工中两处非常规动作，核为非绕闸（依日志＋事件史＋DB，非自述）：①devin 壳行合并
（D-42 类）致哨兵行无法行首起——往席位扫描的 stdout 管道 fd 直写独占行，帧仍经
席位扫描→网关闸→admin_resp，fill_spec/create_ticket/dispatch 三帧 ok=true 且事件
379/380/382 落痕；②sendPump 死流后走官方收养径（per-seat UDS＋run_token 互证
Adopt）复位，一次性呼叫器仓外临时文件交付前已删、未入 commit。

## 暗债对账（令#E6）

交付 diff（`git show 1bd1c79`）`TODO|FIXME|XXX|绕过|临时|hack` grep **零中**
（rc=1，本席亲跑）。摘要「下一步」申报两债：席位流断/sendPump 死＋devin 行合并
（D-42 类缺陷本体未修）、sudoers `/etc/sudoers.d/meta-runner` 待业主——与日志实况
对上，非瞒报。

## 债项（移交，非否决依据）

1. **【新证·P1 待立查】ticket.spec 全表 proto 损毁**：术前快照
   `run/meta-pre-surgery.db` 中 spec 可完整解码（`protoc --decode_raw` rc=0——
   req-9ce5 十栏 field 1–10 全在 2864B；0010 6990B OK）；现行 `meta.db` **全表
   decode 失败**（0010/0013/0014/0015/req-9ce5 逐一 FAIL）。hex 实证：varint 长度
   字节 ≥0x80（如 `a1`）被 UTF-8 有损替换为 `EF BF BD`（U+FFFD），字段边界全毁。
   系交付后 DB 手术重写引入，**与本票交付无关**；凡 `Unmarshal(spec)` 径皆破——
   疑似与 0015 事件 536/551 `spawn_blocked 空提示拒起 role=审计` 相关（注入机
   读 spec 建提示→空→拒起→对账补发才救回），因果待查。立票/入台账。
2. **D-42 类缺陷本体**：devin 壳行合并＋sendPump 死流——施工与五任审计皆靠自救径
   完成，缺陷本体已另有 0016/0017 修复链在途（`accab10`/`5e9ce28` 支）。
3. **per-project 配方注入**：注入机 Recipes 为配置静态列——设计 §5 记债，
   子票票面明「记债台账」。
4. **`DispatchAction.project` 路由表行为**：本批只留字段口不改表（设计 §5）。
5. **sudoers 行**：业主动作，票面两处已载。
6. 小疵（记录非否决）：第 8 栏「ref＝设计文档 commit hash」，deliver 哨兵实载
   文档**路径**非 hash——hash `1bd1c79` 由摘要点名且唯一触及，唯一可解
   （同 2/3/4 档所记）。
7. 观察：521 `shells_exhausted` 后 574/590 对账补发仍派出本席——补发机制兜住，
   但「候选用尽」与补发关系宜入 CEO 视野；五任连死（哨兵漏发×2、429×2、respawn×1）
   暴露审计席哨兵义务与限流脆弱面。

## 质量三问（令#E5）

- **最优解**：六写径共用一处机械闸 `attachProject`（设计 L37），非六处各判；
  复用既存 `db.SetTicketProject` 并顺手消 `SetProject`/`SetTicketProject` 同义
  重复——复用现有、无新增间接层。
- **第一性**：确诊表逐条指到代码行（`schema.go:11` 列在而无生产写径／
  `exec.go:146-152` 拒点／`dispatch.go:94/113` 锁退化/跳过）；自举序 §4 正面解
  「部署需闸、闸需本功能」循环依赖（L78–92 五步钦定序），非 workaround。
- **洁净**：纯文档交付，diff 无重复实现/死码/TODO/调试残渣（grep 零中）。
- **禁线扫描**：attachProject 五判全属机械判（存在性/登记比对/相位集合/等值），
  合禁区「机械判合法」；存量回填径 CLI→RPC→网关，不破唯一写者；设计未引入
  周期任务/心跳/JSON/REST；AdminGateway 增 RPC 有先例，总设计不动有据。

## 判决

**通过**——交付（设计文档 commit `1bd1c79`＋摘要四节）与票面五锚闭合：设计档在
且三节名 grep 中、子票立且 spec 非空（4712B）、dispatch/claim 派发真、本票
fill_spec 事件 379 闸过落痕、验证命令全通、摘要四节形在；范围、形制、暗债、质量
三问均对账无误。五锚全绿，判**通过**。

## 可沉淀意见（令#E8，纯意见）

前判推荐的「席位流断→官方收养径」与「D-42 失明谱」**已沉淀**（实证：commit
`19e0c90` 经验库·哨兵三壳失明谱、`42a646e` 经验库·收养重挂帧泵空转）——沉淀链
工作正常。本席新增实证「spec TEXT 列经有损 UTF-8 重写全表损毁」宜随债项 1 一并
入库（坑型：二进制 blob 存 TEXT 列，迁移重写即毁）。

**下一步（系统）**：本席哨兵回流 → gateway audit_pass 边 → 相位 `审计中→待部署`。
