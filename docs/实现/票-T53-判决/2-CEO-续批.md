# 票-T53 判决·续批（2-CEO·2026-11-05 早｜交互态 devin/SWE-2-Max）

支 `t/T53`｜deliver_ref=`2731627`｜并 master=2225d5c（21 件，+122/−78——
含生产码 2 件：attach.go/reconcile.go 互斥记账，须换装评估）

## 判决：续批通过（分批验收·票未闭）

- **waitFor2 全量接订阅**：46 个 func 字面量多行调用点接入事件订阅道
  （订阅+轮询双醒——非事件变真条件由 15ms tick 兜底，设计对路）；
  D-93 名单调用面至此全罩。
- **三处根治实证方向正**：
  - deploy_failed 双写竞窗→`deployFailMu` 罩 check+append 原子化
    （attach/reconcile 两径同窗恰一条账——T49 病根除）；
  - DesignLock 断流→谓词内代驾帧重投至锁放（st.Send 静默丢帧链实证）；
  - spawn/adopt/claim 径窗抬 30s（负载下进程调度排队实测>15s）。
- **waitEvent 定径**：走 g.events 广播道（256 缓冲回读）——订户道只见
  订后事件，AnchorE「先 adopted 后 waitEvent」序实证根。
- **机检**：build/vet 净；开发面 `-p 2` 全 17 包 341 测两连绿（R2/R3）。

## 续批遗留（下一批靶——锚未足：名单片零抖仍见一红）

- **R1 红实证**：`TestBudget_ThreeGates` 首套跑红——负载下 helper 席
  spawn 即死（self_exit_or_external，systemd scope 起不来/排队超时），
  调和器烧穿壳列表→shells_exhausted→blocked→claim 30s 空转。
  机理＝席 spawn 环境时敏（非押窗族——waitFor2 改造面外）；
  隔离复跑 9 连绿、负载窗内 2/3 红。票保持 open 待下批治此根。
- 票面锚不变：开发面 `-p 2 ./...` 连 5 绿名单片零抖方闭票。
