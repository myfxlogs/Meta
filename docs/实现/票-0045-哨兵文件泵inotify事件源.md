# 票-0045：哨兵文件泵 inotify 事件源——运行中动作哨兵解耦 stdout 活性

### 1. 目标

按设计档 `docs/实现/设计-0034-p3-p1-哨兵文件泵inotify事件源.md` 落
`pumpSentinelFile` 第三触发径：① watcher 协程——`$META_ART_DIR` 目录
inotify watch（`IN_MODIFY|IN_CLOSE_WRITE|IN_CREATE|IN_MOVED_TO`，名滤
`sentinel.log`；`IN_Q_OVERFLOW` 无条件泵；`IN_IGNORED` 净退），阻塞于
inotify-fd＋停管二元 `Select`（零定时器），事件批排干后单泵；
② 并发收口——泵体串行锁（泵三源互斥）＋`pairDedup` pending 叶锁，
锁序 `pumpMu → scanSentinel → {s.mu|pairMu}` 单向；③ 起止挂点——
`cmd.Start()` 成功后与 `go s.pumpGateway()` 同位起，`cmd.Wait()` 后
停管归队再走 :320 终泵；init/addwatch/pipe 任一步败→不起 watcher＋
stderr 留痕（降级不劣于现状）。

### 2. 为什么

泵点全绑 stdout：`:317` 行后泵、`:320` Wait 后泵——devin -p 块缓冲
静默期 sentinel.log 写而无人泵。死席 dda6fb0f 实证：`META_EXEC:check`
在盘 44B 两行俱在、exec-resp.log 只 integrate 一条、派单时戳差
~8.6 分钟（正文重发方获派单）——席等回执而请求从未入队＝结构性自锁，
0036 部署支四连环死＋B1 锁搅动全项目部署支排队。

### 3. 范围内

- `internal/seat/`（seat.go 或同包新文件，施工定——锚管行为不管名）：
  - watcher 协程：`syscall.InotifyInit1(IN_NONBLOCK|IN_CLOEXEC)`＋
    `InotifyAddWatch(ArtifactDir, IN_MODIFY|IN_CLOSE_WRITE|IN_CREATE|
    IN_MOVED_TO)`＋`Select`/`FdSet`/`Pipe2(O_NONBLOCK|O_CLOEXEC)`
    纯 stdlib（linux/amd64 已实测）；**禁 fsnotify 等第三方包**。
    停管＋select 归队禁带 timer；事件批非阻塞排干后一次
    `pumpSentinelFile()`；名过滤 `filepath.Base==「sentinel.log」`。
  - 并发收口：泵体串行锁罩 `pumpSentinelFile` 全体（sentinelOff/
    open/seek/read 在内）；`pairDedup` 内自握叶锁护 pending map；
    s.mu 值类字段照旧。**锁序钦定**：pumpMu → scanSentinel →
    {s.mu|pairMu}，不得反向，pairDedup 不取 s.mu。
  - 起止：`cmd.Start()` 成功＋`ArtifactDir!=""` 方起；`cmd.Wait()`
    返回后关 stop 写端→watcher done 归队→:320 终泵（终泵仍最后
    读者）。`cmd.Start()` 失败径不起。
- 测试面（internal/seat 测试包约定，伪适配器＋桩网关范式）：
  - **静默壳主锚**：脚本 sentinel.log 写 `META_EXEC:<step>` 后 sleep
    静默（stdout 零行）→断言子进程存活期内（≤10s 窗）ExecReq 帧
    入桩网关（step 相符）。须 run 协程化＋截止前探桩，施工定细节。
  - 静默壳 `META_DELIVER:` 文件径→ExitClean＋Deliver.ref 对。
  - 半行不消费：`META_EXEC:x` 无 `\n` 不写→补 `\n` 后单发一次。
  - 活跃径回归：stdout 吐哨兵照常即泵（既有测试群不红即证）。
- `config/`、proto/RPC、adapter 面零变更。

### 4. 范围外（禁线）

- 不改 B1 串行闸语义、scanSentinel 匹配语义、pairDedup 配对契约
  （原文键/流↔文件计数/D-42）、`pumpSentinelFile` offset 与 `\n`
  收口语义、:317/:320 两既有泵点、META_* 哨兵契约文本。
- 不代 0044（死后排干）/0038-p1-p2（放锁）的活；不修 stdout 块缓冲
  壳本体、spawn_blocked 连发本体、enqueue 满→直发既有并发面（设计
  §6 记档项）。
- 禁任何定时器/轮询/心跳件（R2）——select nil timeout 无限阻塞即
  唯一等待形；禁 fsnotify/新第三方依赖；JSON 不入内；不手并 master；
  不改 phase/事件；不部署。

### 5. 设计引用

- `docs/实现/设计-0034-p3-p1-哨兵文件泵inotify事件源.md`（唯一权威——
  §0 实证链/§2 watcher/§3 锁序/§4 落选案/§6 残险）
- `docs/实现/票-0034-p3-p1-哨兵文件泵挂stdout活性-EXEC步派单饥饿.md`
  （需求原文＋实证链）
- `internal/seat/seat.go:274-320`（泵点双绑现状）`:621-650`（泵体）
  `:652-666`（pairDedup）`:687-757`（scanSentinel）`:572-602`
  （sendPump/enqueue 并发面）`:261-272`（cmd.Start 挂点位）
- `internal/seat/seat_test.go`（runSeatSession/fakeAdapter/桩网关/
  parseAdapter 范式；:481-497 文件兜底先例）；syscall 包
  InotifyInit1/InotifyAddWatch/Select/FdSet/Pipe2（stdlib 实测可用）
- `docs/接手者禁区.md` #1（禁周期）；`docs/总设计.md` §1 R2；
  `docs/经验库/2026-11-02-devin-stdout块缓冲-哨兵全季末落.md`

### 6. 验收锚（全部机器可判）

1. `go build ./... && go vet ./... && go test -count=1 ./...` 全绿＋
   `gofmt -l internal/` 零输出；`go test -race -count=1 ./internal/seat`
   绿（新协程并发面强制覆盖）。
2. **静默壳主锚**：stdout 全程零行＋sentinel.log 写 `META_EXEC:<step>`
   →子进程存活期内（测试窗 ≤10s）桩网关收 `ExecReq{step}`——watcher
   泵实证，非 Wait 后兜底产物。
3. **静默壳 DELIVER 姊妹锚**：stdout 零行＋sentinel.log 写
   `META_DELIVER:ref`→ExitClean＋Deliver.ref==ref。
4. **半行锚**：无 `\n` 残段不触发 ExecReq；补 `\n` 后恰发一次。
5. **对抗提名位（删/翻必红）**：watcher 内 pump 调用行／watcher 启动行
   ——任一删除则锚 2 必红（防「watcher 名存实亡」弱化）；pairDedup
   叶锁摘除则 `-race` 红。
6. **降级锚**：ArtifactDir 空/init 失败布景→席位照常跑、stderr 有痕、
   既有两泵语义不变（测试或代码评审皆可证）。

### 7. 验证命令

```bash
export PATH=/usr/local/go/bin:$PATH
cd /opt/meta-wt/0045-sentinel-pump-watch
go build ./... && go vet ./... && gofmt -l internal/
go test -count=1 ./...
go test -race -count=1 ./internal/seat -run 'Sentinel|Exec|Pump|Silent|Watch'
```

手验布景（可选）：本仓 seat 起 devin 壳＋META_ART_DIR 预置，
sentinel.log 写 `META_EXEC:probe`＋stdout 静默 → run/exec/<票>-probe-*
出现且 exec-resp.log 收回执（≤30s）。

### 8. 交付形制

- worktree `/opt/meta-wt/0045-sentinel-pump-watch`＋支
  `t/0045-sentinel-pump-watch`（以 master 现状为基另立新支，推 origin）。
- commit 格式 `[角色:施工][0045] <为什么>`；交付＝推支＋
  `META_DELIVER:<commit>`（哨兵双发：stdout 独占行＋同文追加
  `$META_ART_DIR/sentinel.log`）。
- 摘要四节（新增/修改面、根因、验证引、下一步——全量遗留申报）；
  逐锚对账行不可缺；全绿＋§6 锚全过方 Deliver。

## 附

- 优先级 P1（部署链自愈债·静默席饥饿径）；依赖：无；预算/壳序缺省；
  workdir=/opt/meta。
- 上游：需求票 0034-p3-p1（巡检自立案）、设计档同票 commit；
  派位：注入机径。
- 残险记档在设计 §6（enqueue 满直发既有并存面/异质 fs 事件缺席/
  事件风暴/exit=-1 真凶另案）——范围内照做不扩修。

## 判决史

（空——首判前无）
