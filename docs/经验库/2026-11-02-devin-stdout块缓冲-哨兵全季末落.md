# 坑：devin -p stdout 全季块缓冲——席位扫描器全程挨饿，哨兵帧只在退壳冲刷时落

**实证**：票 0038 CTO-A 设计席（run 272dc8de9157addd，2026-11-02，devin/
swe-2-max）——会话干了 ~25 分钟、数百行工具输出，席位日志
`run/logs/<run>.log` 全程 **0B**：`meta seat` 用 `cmd.StdoutPipe()` 扫壳
stdout，而 `devin -p` 对管道**块缓冲**——一行都未冲出。于是：

- 零 progress/usage 帧出站（票事件流 claim 后死寂，但 run attached=true）；
- `META_ADMIN:`/`META_DELIVER:` 的 stdout 副本全部堵在壳侧缓冲区；
- `$META_ART_DIR/sentinel.log` 文件行也一样等——`pumpSentinelFile` 只在
  每行扫描后跑，扫描器饿则泵不醒。

诊断信号三连：`seat_run.attached=true` 且事件流 claim 后零进展行
`＋ logs/<run>.log == 0B` ＝ 缓冲挨饿（区分真卡死：PID 活、无任何帧）。

## 结果与防线（本票实证有效）

1. **哨兵全部走文件径先行**：META_ADMIN/META_DELIVER 行写进
   sentinel.log 即算数——`cmd.Wait()` 后 `pumpSentinelFile` 兜底泵
   必跑（净退闸判前最后一泵），壳退出时 stdout 冲刷亦会扫描全量。
   帧序＝文件行序，admin→deliver 在 gRPC 流上 FIFO 保序
   （`drain()` 落尽才发 Deliver，网关逐帧同步处理）。
2. **立票/派票等确定性动作走 CLI 不等冲刷**：
   `meta ticket new --spec-file`（客户端 ParseTicketMD＋marshal，
   NULL-spec 票零可能）＋ `meta dispatch`——直送 AdminGateway，
   本票 0038-p1 即此径立派。哨兵行照旧双发留痕，落时撞「已存在/
   已有活席」闸无害。
3. fill_spec 无 CLI 等价件，只能随冲刷落——因其闸是「本票施工中」，
   而施工中持续到 Deliver 帧被处理，序自洽，晚落不误。
4. 文件泵先行次序的另一面：**不要在确认落地前发 stdout 副本**——
   截断半行若先于完整文件行被处理，`create_ticket` 会落 NULL-spec
   僵票（姊妹坑 2026-11-02-哨兵长行截断）。

姊妹坑：`2026-11-02-哨兵长行截断-spec路径半行-NULL票.md`（同族：
devin stdout 缓冲 vs 哨兵协议——彼坑＝行内腰斩，本坑＝全流不冲）。
