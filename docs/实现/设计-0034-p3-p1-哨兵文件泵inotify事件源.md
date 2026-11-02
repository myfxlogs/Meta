# 设计-0034-p3-p1：哨兵文件泵解耦 stdout 活性——inotify 事件源第三泵

> 需求票 `0034-p3-p1`（巡检自立案 adjunct）→ CTO-A 设计 → 子票 `0045` 施工。
> 钦定轨（§1.4）：设计档 commit → 本票八栏回填 → 子票立＋派 → Deliver。
> 结论先行：票面「泵触发绑 stdout → 静默壳运行中动作哨兵饿死」断链逐条
> 坐实（§0）。核心钦定＝**`pumpSentinelFile` 增第三触发径：专用 watcher
> 协程以 inotify 盯 `$META_ART_DIR` 目录的 sentinel.log 名事件，阻塞于
> inotify-fd＋停管二元 select——壳存活期内与 stdout 完全解耦、零定时器；
> `cmd.Wait()` 后停管收口，原 :320 兜底泵仍压轴**。锁序钦定（§3）是本次
> 唯一新增险面；落选案（轮询/fsnotify/文件 watch/close 停读）逐条否决（§4）。

## 0. 实证链复核（票面断链俱实，本席逐条亲核）

- **泵点双绑 stdout**：`seat.go:317` 每 stdout 行处理后
  `s.pumpSentinelFile()`、`:320` `cmd.Wait()` 后最后一泵——除此外子进程
  存活期无任何泵源，逐字复核 seat.go:274-320 全段无第三调用点。
- **泵本体合法、触发失联**：`pumpSentinelFile`（:625-650）offset 增量读＋
  `\n` 收口行过滤语义正确；`scanSentinel`（:687-757）`META_EXEC:` 双源
  接入（:731-744）文件行内容上合法——病不在泵体，在**无人唤泵**。
- **死席自诊逐字**：`run/logs/dda6fb0f7567054d.log`——「`META_EXEC:check`
  未被泵进席位日志——泵挂在 stdout 活性上，工具输出不喂泵」，静默等回执
  ~8 分钟后以正文重发方获派单。
- **文件径写而无人泵**：`run/artifacts/dda6fb0f7567054d/sentinel.log`
  两行俱在（`META_EXEC:integrate a34bcda`＋`META_EXEC:check`）；
  `exec-resp.log` 只含 integrate 一条——check 请求从未入队。
- **派单时戳差**：`run/exec/0036-integrate-…8294…log`（19:18:14）→
  `0036-check-…8810…log`（19:26:50），间隔 ~8.6 分钟；check 即正文重发
  后的补泵产物，非文件径直发。
- **连环死实害**：0036 部署支 c075559e/7ab445fc exit=61、266e3e50
  exit=2、dda6fb0f exit=-1——死→回滚→放锁→补发→锁搅动，spawn_blocked
  风暴全项目部署支排队（事件 15751 deploy_failed 在案）。
- **防线空洞定位**：经验库 `2026-11-02-devin-stdout块缓冲-哨兵全季末落.md`
  盖**终态哨兵**（Wait 后兜底泵必跑）；本票＝同族**运行中动作哨兵**——
  文件写径在、触发径断，既有防线不盖。

## 1. 总机制（一图）

```
壳（devin -p 块缓冲，stdout 可全程静默）
   │ 双发义务：stdout 行 + sentinel.log 追加（D-42 契约不动）
   ▼
seat 内三泵源（★＝本票新增）
 ① stdout 行后泵  :317        —— 活跃径照旧（零变更）
 ② ★inotify watcher 协程      —— $META_ART_DIR 目录名事件
    sentinel.log IN_MODIFY|CLOSE_WRITE|CREATE|MOVED_TO
    → pumpSentinelFile（泵体/offset/配对语义全不动）
 ③ Wait 后兜底泵 :320          —— 终态哨兵压轴（零变更）
   │ scanSentinel(fromFile) → pairDedup → enqueue → ExecReq 帧
   ▼
网关三重闸 → sudo 代跑 → exec-resp.log 回执落盘（relayExecResp 照旧）
```

停管（self-pipe）与 inotify-fd 二元 `Select`（nil timeout＝无限阻塞，
**零定时器零轮询**——内核文件事件即唯一唤醒源）：

```
cmd.Start() 成功 ──► watchSentinel 协程起（ArtifactDir==""/init 败→不起）
   Select{ ifd, stopR }
     ifd 可读 → 非阻塞排干事件批 → 名过滤 → 一次 pump
     stopR 可读 → 退
cmd.Wait() 返回 ──► 关 stopW → 等 watcher done 归队 ──► :320 终泵
```

## 2. 机制一：inotify 目录 watch＋watcher 协程

- **watch 对象＝目录非文件**：`ArtifactDir` 整目录一把 watch，事件名过滤
  `basename == "sentinel.log"`。文件 watch 落选——席起时文件常不存在
  （壳 lazy 首写），且 CREATE/MOVED_TO 需目录径才可见。
- **事件掩码**：`IN_MODIFY|IN_CLOSE_WRITE|IN_CREATE|IN_MOVED_TO`；
  `IN_Q_OVERFLOW`→无条件泵一次（溢队列＝可能漏行，保守补泵）；
  `IN_IGNORED`（watch 被摘/目录消失）→watcher 净退。
- **每次写都泵、每批一次**：非阻塞 `Read` 排干一批事件→一次
  `pumpSentinelFile()`。泵体 offset 语义天然幂等——空泵零成本
  （文件缺/无新完整行直返），过泵无害。
- **起止挂点**：`runChild` 内 `cmd.Start()` 成功后、与 `go s.pumpGateway()`
  同位起；`ArtifactDir==""` 或 `InotifyInit1/AddWatch/Pipe2` 任一步败→
  **不起 watcher＋stderr 留痕**，既有两泵点语义一毫不退（降级不降级于
  现状）。`cmd.Start()` 失败径不起 watcher，无泄漏。
- **停**：`cmd.Wait()` 返回后关 stop 写端→watcher 经 done 通道归队
  （停管恒定可读，select 无超时即可确定性归队，禁 join 加 timer）→
  再走 :320 既有终泵——终泵仍是最后一个读文件者，与今语义一致。
- **纯 stdlib**：`syscall.InotifyInit1(IN_NONBLOCK|IN_CLOEXEC)`、
  `InotifyAddWatch`、`Select`、`FdSet`、`Pipe2(O_NONBLOCK|O_CLOEXEC)`——
  linux/amd64 已实测全在（本席 `go run` 探针 init+addwatch 成）。
  代码落点 seat.go 或同包新文件由施工定（锚管行为不管名）。

## 3. 机制二：并发收口（本票唯一新增险面，锁序钦定）

watcher 使泵路径首次二协程化，三把锁序钦定（缺一＝-race 红＋配对错）：

- **pumpMu（新增，串行泵体）**：`pumpSentinelFile` 全体在锁内——三调用源
  （stdout 行泵、watcher 泵、Wait 终泵）互斥。`sentinelOff`、offset 续读、
  文件 open/seek/read 全在其罩下。
- **pairMu（新增，配对表叶锁）**：`pairDedup` 的 `pending` map 现可从
  流源（stdout 环）与文件源（watcher 泵下 scanSentinel）并发触——
  `pairDedup` 内部自握叶锁，键值增减＋判定原子化；语义（原文配对、
  流↔文件计数）一字不改。
- **s.mu（沿用）**：值类字段（verdict/deliver/deployNote 等）照今。
- **锁序**：`pumpMu → scanSentinel → {s.mu | pairMu}`，单向不可逆——
  任何路径不得在 s.mu/pairMu 内回取 pumpMu；pairDedup 不碰 s.mu；
  pumpSentinelFile 不碰 s.mu 直取。
- `enqueue` 并发已立（sendq 通道＋满则直发，pumpGateway 径今已并发），
  watcher 泵出帧同径不新险。

## 4. 落选案逐条否决（红线合规论证）

| 案 | 否决理由 |
|---|---|
| 定时轮询/Ticker 泵 | **R2/禁区#1 直踩**——周期任务禁令本体。 |
| 一次性闹钟续臂兜泵 | 无事件源可重置闹钟——闹钟自身即成变相周期。 |
| fsnotify 包 | 新第三方依赖 → 禁区「停手报业主」级；stdlib `syscall` 全覆盖（本席实测），无需外引。 |
| x/sys/unix | 已在 module graph（间接依赖），可作施工备选；stdlib 优先——go.mod 零扰动。 |
| watch 文件非目录 | 席起时 sentinel.log 常不存在；CREATE/MOVED_TO 目录径才见。 |
| close(ifd) 停读 | Linux 下 close 不保证唤醒他线程阻塞 read（可挂至下一事件）→停管＋select 为确定性正解。 |
| 网关侧代泵 | 泵体是席侧件（ArtifactDir 语义、offset、pairDedup 皆席内状态）——外移＝跨进程复制状态，复杂度倒挂。 |

## 5. 时序（静默壳 META_EXEC 命途）

```
t0 壳写 sentinel.log「META_EXEC:check\n」＋stdout 双发（被块缓冲压住）
t0+ε 内核 dir 事件 → watcher Select 醒 → 排干批 → pump
   → scanSentinel(fromFile) → pairDedup(+1/-1 依流文件先后序)
   → ExecReq{step:check} → sendq → 网关切闸 → 代跑
   → exec-resp.log 回执（relayExecResp 既有径）
壳 stdout 行后到（若有）→ :317 泵 → 同文行 pairDedup 消化（D-42 不动）
cmd.Wait() → 停 watcher → :320 终泵（尾行/残段照常收口）
```

## 6. 记档（残险与判词）

- **enqueue 满→直发径**：队列满时 `st.Send` 与 sendPump 并发 Send 是既有
  并存面（gRPC 流并发 Send 非契约安全）——本票不恶化（泵频率受文件事件
  限），记档不归修。
- **ArtifactDir 落远端/异质 fs**：inotify 对非本地 fs 事件可缺席——降级
  面即今现状（stdout/终泵双点仍在），AddWatch 成败亦留痕，错配方向保守。
- **容器笼径**：watcher 在宿主席进程、watch 宿主 ArtifactDir——笼内壳
  经 bind mount 写同 inode，事件照常（docker -v 宿主径语义）。
- **事件风暴**：壳狂写文件→事件批频泵——泵体幂等且批量合并，sendq 容量
  ＋pairDedup 收口；极端面残余可受。
- **seat exit=-1 真凶未坐实**（票面自注）：饥饿 8.6min 本体即缺陷；
  修后饥饿窗绝迹，静默看门狗若另有 kill 口径归另案。

## 7. 范围外（子票不碰）

- B1 串行闸本体语义；scanSentinel 匹配语义；pairDedup D-42 双发契约；
  META_* 哨兵契约文本；adapter 层与 proto/RPC 面零变更。
- 0044（死后态判/排干 META_DEPLOY）与 0038-p1-p2（放锁）的活——本票
  修**活席运行中**派单径，死后径各归其票。
- spawn_blocked 事件连发本体不视为待修异常；stdout 块缓冲壳本体不修。
- 不加任何定时器/轮询/心跳件；JSON 不入内；不并修邻票。

## 8. 子票锚预览（详见票-0045 八栏）

- 静默壳：stdout 全程零行下写 `META_EXEC:<step>` 入 sentinel.log →
  子进程存活期内（测试窗 ≤10s）ExecReq 帧入桩网关——watcher 泵实证。
- 静默壳写 `META_DELIVER:` → ExitClean＋Deliver.ref 对（姊妹回归）。
- 半行不消费：`META_EXEC:x` 无 `\n` 静默写下不发，补 `\n` 后单发。
- 活跃径不退：stdout 活跃壳哨兵照常即泵（既有测试群回归）。
- `-race` 绿：`go test -race ./internal/seat`——新协程并发面强制覆盖。
- **对抗提名位（删/翻即红）**：watcher 泵调用行/watcher 启动行——
  任一删除则静默壳用例必红（防「泵源名存实亡」弱化）。
- `go build ./... && go vet ./... && go test -count=1 ./...` 全绿＋
  `gofmt -l internal/` 零输出。
