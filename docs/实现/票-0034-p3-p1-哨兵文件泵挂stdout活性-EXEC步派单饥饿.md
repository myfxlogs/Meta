# 票 0034-p3-p1：sentinel.log 文件泵挂 stdout 活性——静默席 META_EXEC 派单饥饿，部署席连环死

> 巡检席自立案（adjunct，不占相位不重派）。巡检 run 22b8f60c3a22bf33
> 于事件 15751 窗口（0036 deploy_failed）复核死席日志判真信号，
> `meta ticket new` 入网关账。触发：spawn_blocked 连发巡检
> （0034-p3 · CTO · 「项目 meta 部署/集成锁占用（B1 串行闸）」）——
> spawn_blocked 本体判噪声（B1 设计语义，二十六轮先例同判），
> 立案对象是诊断中新坐实的派单饥饿根因。

### 1. 目标

运行中动作类哨兵（`META_EXEC:` 及同族 `META_RULE:`/`META_ADMIN:`）
经文件信道 `$META_ART_DIR/sentinel.log` 上报时，派单不得挂壳
stdout 活性——壳零输出期（devin -p 块缓冲管道）文件行也须有界
时延内被泵走。

### 2. 为什么

泵点全事件驱动零定时器：`seat.go:317` 每 stdout 行处理后
`s.pumpSentinelFile()`、`seat.go:320` `cmd.Wait()` 后最后一泵。
`scanSentinel`（seat.go:731-743）双源都接 `META_EXEC:`，故文件
行内容上合法——但**泵的触发仍绑 stdout**。终态哨兵
（DELIVER/DEPLOY/VERDICT）有 Wait 后兜底泵盖；运行中 EXEC 无此
兜底——席在等 exec-resp.log，而请求从未入队＝结构性自锁。

实证链（本席逐条亲核）：

- 死席 run `dda6fb0f7567054d`（0036 部署支）日志
  `run/logs/dda6fb0f7567054d.log` 自诊逐字：「`META_EXEC:check`
  未被泵进席位日志——泵挂在 stdout 活性上，工具输出不喂泵」；
  静默等回执 ~8 分钟后以正文重发方获派单。
- `run/artifacts/dda6fb0f7567054d/sentinel.log`（44B）两行俱在：
  `META_EXEC:integrate a34bcda`＋`META_EXEC:check`——文件径写了，
  无人来泵。
- 派单时戳差坐实饥饿：integrate 派单日志
  `0036-integrate-1793618294…`（19:18:14）→ check 派单日志
  `0036-check-1793618810…`（19:26:50），间隔 ~8.6 分钟——check
  是在 integrate 回执后即发的。
- check 跑通（log 内 8 包测试全 ok）但席旋即死（exit=-1，
  ~19:27:2x），exec-resp.log 无 check 回执行，配方止于半途
  （deploy/probe/META_DEPLOY 皆无）→ 事件 15751 deploy_failed
  回滚待部署。
- 0036 部署席已连环死：c075559e exit=61、7ab445fc exit=61、
  266e3e50 exit=2（退化净退无哨兵）、dda6fb0f exit=-1——每次
  死→回滚→放锁→补发→锁搅动，spawn_blocked 风暴全项目排队。
- 经验库 `2026-11-02-devin-stdout块缓冲-哨兵全季末落.md` 已立
  「缓冲挨饿」防线，但其盖的是**终态哨兵**（Wait 后兜底泵必跑）；
  本坑＝同族**运行中动作哨兵**——文件写径在、触发径断，防线不盖。

### 3. 范围内

- `internal/seat/seat.go`：pumpSentinelFile 触发径改造——子进程
  存活期内与 stdout 解耦的有界泵（定时轮询或 fsnotify/inotify
  于 sentinel.log），现有两处调用点保留不删。
- 回归测试：模拟块缓冲静默壳＋sentinel.log 写 META_EXEC →
  断言有界时延内 ExecReq 入队/派单。

### 4. 范围外（禁线）

- 不动 B1 串行闸本体语义（同项目部署串行＝设计特性，票0010）。
- 不代 0044（死后态判/排干 META_DEPLOY）与 0038-p1-p2（放锁）
  的活——本票修的是**活席运行中**派单径，死后径各归其票。
- 不动 scanSentinel 匹配语义、pairDedup 双发去重契约（D-42）。
- spawn_blocked 事件连发本体不视为待修异常。

### 5. 设计引用

- `internal/seat/seat.go:317`（行后泵）`:320`（Wait 后泵）
  `:621-650`（pumpSentinelFile 本体）`:655-666`（pairDedup）
  `:729-743`（META_EXEC 双源接入）。
- 死席实证：`run/logs/dda6fb0f7567054d.log`、
  `run/artifacts/dda6fb0f7567054d/{sentinel.log,exec-resp.log}`、
  `run/exec/0036-{integrate,check}-*.log` 时戳差。
- 姊妹档：`docs/经验库/2026-11-02-devin-stdout块缓冲-哨兵全季末落.md`
  （终态哨兵防线，本票补其运行中空洞）；
  `docs/实现/票-0038-p1-p2-B1部署锁孤儿死席不放锁.md`（死后放锁）；
  `docs/实现/票-0044-部署态判双径失守三补.md`（死后态判）。
- 票0010（B1 闸＋META_EXEC 代跑契约）、D-42（哨兵双发）。

### 6. 验收锚（机器可判）

- 壳 stdout 全程静默（块缓冲无冲刷）下，写
  `META_EXEC:<step>` 入 sentinel.log → ≤30s 内
  `run/exec/<票>-<step>-*.log` 出现且 exec-resp.log 收回执。
- 既有泵点行为不回退：stdout 活跃径派单时延不劣化；
  Wait 后兜底泵仍捕终态哨兵。
- `go test ./internal/seat` 新增静默壳用例全绿。

### 7. 验证命令

```bash
go test -count=1 ./internal/seat/ -run 'Sentinel|Exec|Pump'
/opt/meta/bin/meta events --socket /opt/meta/run/meta.sock --ticket 0036 | tail -12
cat /opt/meta/run/logs/dda6fb0f7567054d.log   # 死席自诊原文
ls -la /opt/meta/run/exec/ | grep 0036        # 派单时戳差
```

### 8. 交付形制

实现＋回归测试随票交付；exec/events 档自证静默派单径；
Deliver 摘要四节（新增/修改面、根因、验证引、下一步）。

## 附

- 优先级 P1（部署支连环死＋锁搅动全项目部署支排队）。
- 依赖/相邻：与 0044、0038-p1-p2 同为死席簇但径位不同——
  彼二票修「死后」，本票修「活着被饿」。D-76 停滞检测若更早
  咬死静默席，本票饥饿面会表现为「加速死亡」，不冲突。
- 触发：spawn_blocked 巡检（0034-p3）诊断支链；第 27 次巡检
  claim 本票窗口坐实。
- 备注：seat 死因未亲证到 killer（exit=-1，疑静默超时类看门狗
  在饥饿窗口内收席）——立案主张不依赖死因定性，饥饿 8.6 分钟
  本身即缺陷。

## 判决史

（空——首判前无）
