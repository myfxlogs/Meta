# T32（外派·化身唤醒链与内部票部署径）判决：通过（CEO 终端形态·决策 101，2026-11-03）

- 交付：支 `t/t32-wake-chain` @ `95380fe`（worktree 合规；收口行钦定式）。
  五件 +197/−15。独立复算：build/vet RC=0、`go test ./...` **332 绿/17 包**、
  selfcheck RC=0。

## 对账

| 件 | 判 |
|---|---|
| ①ceoBusy PID 复用假阳性（第二断点根因）——processAliveRun 双窗判（proc starttime×行戳） | 绿——教科书级根因，唤醒链通 |
| ②克隆败本地仓 fallback 缝（内部票径） | 绿 |
| ③cli-drive spawn 前活席核（同秒双 deploy_start 角） | 绿 |
| ④redirect 门盖待部署（T30 复位角）＋壳种子逐行幂等 upsert（漏行自补） | 绿 |

锚四件齐（t32_test.go）。

**结论：通过。合入＋换装后＝化身唤醒实测（deploy_failed 触发→5 分钟内 CEO 行必现）。**
