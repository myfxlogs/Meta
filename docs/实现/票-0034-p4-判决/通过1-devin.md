# 票-0034-p4 判决（deploy态判两径失守·设计票）——审计

支 master｜deliver_ref=aeccb37｜基 aeccb37^（设计票钦定形制：交付＝设计档
commit 直落主线，无 t/* 施工支——子票 0044 方走 worktree 径）

## 判决：通过

## 逐锚对账（每条「锚=PASS+证据锚｜未达+缺口」——只信diff与机器记录）

- 锚① 设计档在仓且含实证链/三机制/时序/子票锚预览/残险记档：PASS
  （`docs/实现/设计-0034-p4-席死径态判排干与证据换源.md`——§0 实证链、
  §2-4 三机制、§5 时序三径、§8 子票锚预览、§6 残险记档俱在；
  `grep -c "排干\|stamp\|deployStateOK\|sentinel"`＝35＞0）
- 锚② 票-0044 在仓八栏零缺：PASS（`docs/实现/票-0044-部署态判双径失守
  三补.md`——§1 目标→§8 交付形制＋附＋判决史全量；ParseTicketMD 旁证＝
  事件流 15431 dispatch→15432 claim 无 inject_failed 帧）
- 锚③ DB 票 0044 存在且 spec 非空、project=meta：PASS（sqlite
  `ticket` 表：id=0044｜project=meta｜spec len=6877；`meta phase`＝施工中）
- 锚④ 0044 事件史含 dispatch/claim 行：PASS（15431 dispatch 设计中→已派、
  15432 claim 已派→施工中 run=65df1a0f605edabb——派发真发生且施工席已接）
- 锚⑤ 本票 spec 已回填：PASS（`docs/实现/票-0034-p4-设计票.md` 八栏＋附＋
  判决史全量在仓；事件 15427 fill_spec ack；库面 spec len=4015 非空）

## 偏差/债项（瞒报对账结论落此）

- 暗债对账：diff 四件 grep `TODO|FIXME|XXX|待补|半成品|TBD` 零命中；
  交付摘要「下一步」申报四件（devin stdout 块缓冲族症＋冲刷自救法补记、
  boot 对账 Probe 结构性红、master 漂移窗、NewBin 覆写窗）——后三件与
  设计档 §6 记档逐字对上，头一件票面钦定范围外。申报全对上，无瞒报。
- 行号漂移核查无债：设计文引 attach.go:558-563，master 现码该 case 在
  568——`git show aeccb37:internal/gateway/attach.go` 实证 **引时即在
  558**，漂移系 dbfbd06 并入 T17（attach.go +14 行）所致，非引证失实。
- 其余偏差：无。

## 独立复算（机器输出锚——禁引施工自述）

- 范围核：`git show aeccb37 --stat`＝4 文档件 532+/0-，逐件对 §3 白名单
  （设计档＋需求原文档收编＋0044 票面＋本票 fill_spec 件）；零码改——
  「设计票不施工」禁线守。
- 实证链复跑（设计 §0 五源逐条亲核）：
  - `sqlite3 run/meta.db "select step,count(*) from cmd_run where
    ticket_id='0034' group by step"`＝check16/integrate14/probe2/rollback5；
    `step='deploy'` 计 0——「证据结构性缺席」实。
  - 事件 14701 note「部署中席死：先核回滚态（前任 run=e2ea3eccfcc48262…）」
    逐字对 attach.go 死席分支 note；对照事件 14535「对账：行动者无响应
    ——先核回滚态 run=4d8d5b1f…」出 reconcile.go:228——双径失守主张实。
  - `run/artifacts/e2ea3eccfcc48262/sentinel.log` 末行
    `META_DEPLOY:done` 在盘——尾哨兵蒸发实。
  - `run/exec/0034-deploy-1793616442326190093.log` 尾行
    「upgrade: 已装 bin/meta.new → /opt/meta/bin/meta（prev=true）」＋
    `bin/meta version`＝`meta 83ef07a-deploy`——部署实成实。
- 码位引用亲核（§5 设计引用逐点对照）：reconcile.go:111-128
  deployStateOK 证据源＝cmd_run 末次 deploy exit=0＋Probe（实）；
  reconcile.go:206/212/228 死席分支·态判单挂点·对账 note（实）；
  exec.go:197-207 rec() 写库在 cmd.Wait 后（实）；seat.go:621-650
  pumpSentinelFile 席侧件（实）；upgrade.go:62「已装」行格式（实）；
  meta-deploy.sh:20 code_head pathspec 同式（实）；ceo.go:236/
  dispatch.go:463 artifacts 推径同式（实）；gateway.go New() 内
  reconcile 先于 Serve()——boot Probe 结构性红残险记档有据（实）。
- 形制核：`run/artifacts/59fe50e249637468/sentinel.log` 四哨兵齐
  （fill_spec｜create_ticket｜dispatch｜`META_DELIVER:aeccb37`）；
  deliver 事件 15476 ref=aeccb37 exit=0；摘要四节在 run 日志在案。
- 质量核：设计直击判法根因而非缓冲本体（族症票面钦定另案，切分正当）；
  排干一处两用（attach＋reconcileActor 双挂点）超票面单点名挂点——
  属必要延展且双向披露（设计 §2 注＋设计票附注）：boot 径不收则
  「签名绝迹」锚在 boot 场景失守，延展有据非暗扩；D-2 真败径语义在
  子票锚 4/5 双保（无已装证据仍翻负），「席死恒 deploy_ok」弱化被
  反锚钉死；stamp 期望源钦定优先序（NewBin 文件互证免 master 漂移）为
  实测更优解。

可沉淀打法候选：交付摘要未单列候选节，唯「下一步」提名经验库补记
「灌文本冲刷」自救法——可沉淀（泵滞族已立案在库，冲刷法作同族自救
补笔有复用面）；属建议非否决依据。
