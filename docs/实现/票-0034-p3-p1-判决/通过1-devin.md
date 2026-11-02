# 票-0034-p3-p1 判决（sentinel.log 泵挂 stdout 活性·设计票）——审计

支 master｜deliver_ref=96a5e5e｜基 96a5e5e^（设计票钦定形制：交付＝设计档
commit 直落主线，无 t/* 施工支——子票 0045 方走 worktree 径，前例
0034-p4=aeccb37 同式）

## 判决：通过

## 逐锚对账（每条「锚=PASS+证据锚｜未达+缺口」——只信diff与机器记录）

- 锚① 设计档在仓且含实证链/机制/时序/锁序/残险/子票锚预览：PASS
  （`docs/实现/设计-0034-p3-p1-哨兵文件泵inotify事件源.md`——§0 实证链
  六条、§1-3 两机制＋锁序钦定、§4 落选案否决、§5 时序、§6 残险记档、
  §8 子票锚预览俱在；`grep -c "inotify\|pumpMu\|pairMu\|Select\|停管"`
  ＝17＞0）
- 锚② 票-0045 在仓八栏零缺：PASS（`docs/实现/票-0045-哨兵文件泵inotify
  事件源.md`——`grep -n "^### "`＝§1 目标→§8 交付形制八节全量＋附＋
  判决史；ParseTicketMD 旁证＝事件流 16105 dispatch→16106 claim 无
  inject_failed 帧）
- 锚③ DB 票 0045 存在且 spec 非空、project=meta：PASS（sqlite
  `ticket` 表：`0045|meta|6776|施工中`——spec len=6776；`meta phase`
  ＝施工中）
- 锚④ 0045 事件史含 dispatch/claim 行：PASS（16105 dispatch 设计中→已派、
  16106 claim 已派→施工中 run=ebe3d4da8955c443，`meta runs` 示该席
  dead=false——派发真发生且施工席存活）
- 锚⑤ 本票 spec 已回填：PASS（`docs/实现/票-0034-p3-p1-设计票.md` 八栏＋
  附＋判决史全量在仓；事件 16115 fill_spec ack＋run 日志 admin_resp
  ok=true「spec 已回填」；库面 spec len=3922 非空）

## 偏差/债项（瞒报对账结论落此）

- 暗债对账：diff 三件 grep `TODO|FIXME|XXX|绕过|半成品|暂` 零命中；
  交付摘要「下一步」申报三项（0045 施工链在走／fill_spec 文件哨兵已写／
  残险见设计 §6）——设计 §6 记档五残险（enqueue 满直发既有并存面／
  异质 fs 事件缺席／容器笼径／事件风暴／exit=-1 真凶另案）与摘要逐字
  对上。申报全对上，无瞒报。
- 行号漂移核查无债：设计文引 seat.go:317/:320/:621-650/:655-666/
  :729-744/:261-272/:572-602，`git diff 96a5e5e..master --
  internal/seat/` 为空——引时与现码同位，零漂移。
- 管理帧回执噪声（记档非债）：run 日志 admin_resp 两帧 ok=false
  （create_ticket UNIQUE 冲突／dispatch「已有活席」）——D-42 双发契约
  的第二发被网关幂等拒，首发均已落账（事件 16102 立票／16105 派发在）。
  非缺陷，录此免后人误判为派发败。
- 其余偏差：无。

## 独立复算（机器输出锚——禁引施工自述）

- 范围核：`git show 96a5e5e --stat`＝3 文档件 +378/−0，逐件对设计票
  §3 白名单（设计档＋本票八栏回填件＋0045 票面）；零码改——「设计票
  不施工」禁线守；工作区 `git status` 净。
- 实证链复跑（设计 §0 六源逐条亲核）：
  - `grep -n pumpSentinelFile internal/seat/seat.go`＝:317 调用、:320
    调用、:625 定义——泵点双绑 stdout 实，全文件无第三调用点。
  - `run/artifacts/dda6fb0f7567054d/sentinel.log` 两行俱在
    （`META_EXEC:integrate a34bcda`＋`META_EXEC:check`）；
    `exec-resp.log` 仅 `step=integrate` 一录——check 从未入队实。
  - `ls run/exec/ | grep 0036`：integrate-1793618294（19:18）→
    check-1793618810（19:26）差 516s≈8.6min——饥饿窗实。
  - `run/logs/dda6fb0f7567054d.log` 自诊逐字在：「META_EXEC:check 未被
    泵进席位日志——泵挂在 stdout 活性上，工具输出不喂泵」。
  - `meta runs --ticket 0036`：c075559e exit=61／7ab445fc exit=61／
    266e3e50 exit=2／dda6fb0f exit=-1——四连环死实；事件 15751
    deploy_failed 在案（note 直挂 dda6fb0f 死席径）。
  - 姊妹档 `docs/经验库/2026-11-02-devin-stdout块缓冲-哨兵全季末落.md`
    在库（终态哨兵防线）——本票「运行中径空洞」主张与之径位分立，实。
- 码位引用亲核（设计票 §5／0045 §5 逐点对照）：seat.go:267 `cmd.Start`
  挂点＋:271 `go s.pumpGateway()` 同位（实）；:573-574 sendPump 唯一
  发件者注释＋:584-594 enqueue sendq 满则 direct 直发（实——设计 §6
  「并发 Send 非契约安全」残险记档有据）；:687 scanSentinel、:731
  `META_EXEC:` CutPrefix 双源接入（实）。
- 形制核：`run/artifacts/200da9c948b64116/sentinel.log` 四哨兵齐
  （fill_spec｜create_ticket|0045｜dispatch|0045|施工｜
  `META_DELIVER:96a5e5e`）；deliver 事件 16116 ref=96a5e5e exit=0；
  摘要四节在 run 日志在案（新增面/根因/验证引/下一步）。
- 禁区核：设计零定时器零轮询——inotify-fd＋停管二元 Select nil timeout
  无限阻塞（R2/禁区#1 合规）；纯 stdlib `syscall` 零新依赖（fsnotify
  落选案以「新依赖→停手报业主」条款否决有据）；JSON 不入内；泵体/
  scanSentinel 匹配/pairDedup D-42 配口语义钦定一字不改。
- 质量核：机制直击病灶——缺「文件写→唤泵」事件源，inotify 目录 watch
  为正解（文件 watch 于 lazy 首写前不可见、CREATE/MOVED_TO 唯目录径可见，
  §4 否决有据）；唯一新增险面（泵径二协程化）以锁序钦定
  `pumpMu → scanSentinel → {s.mu|pairMu}` 收口，且 0045 锚 5 提名对抗位
  （删 watcher 泵行→静默壳锚必红／摘 pairDedup 叶锁→-race 红），E5
  对抗锚义务合；降级径（init/addwatch/pipe 任一步败→不起 watcher＋
  stderr 留痕）保守不劣于现状。落选案七条逐条有实据否决，本席指不出
  更简等价解。

可沉淀打法候选：摘要「下一步」未单列候选节；唯设计档 §4「落选案逐条
否决表」（红线合规论证内嵌成表）与 §0「票面断链逐条亲核」两形制有
复用面——可沉淀（设计档自证合规免审计重查禁区，断链实证链可作巡检
立案票据模板）；属建议非否决依据。
