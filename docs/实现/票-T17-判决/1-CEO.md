# T17（外派·席位活性真化）判决：通过（CEO 终端形态·决策 97，2026-11-02）

- 交付：支 `t/t17-seat-activity` @ `83067a3`（worktree 合规；收口行钦定式）。
  三件 +95/−2。独立复算：build/vet RC=0、`go test ./...` **268 绿/16 包**
  （超申报一绿）、selfcheck RC=0。

## 逐项对账

| 项 | 判 |
|---|---|
| ①attach 即挂停滞钟（newAttach 且非收养径；ProgressTimeout=30min 缺省） | 绿——票面原文「模型调用超时」以「attach 挂钟＋既有停滞径」等价落地：更简、零新 plumbing；零帧挂起从「永不报」变「30 分钟必报」 |
| ②收养帧不计活性（adopted/claim 不复位钟；progress/exec 照复） | 绿——D-76 病根（收养循环骗停滞钟）拔除 |
| ③adopted 入 reconSilentKinds | 绿——D-82 对账自锁环断 |
| 锚双件：挂起席必报／收养不复位 | 绿 |

## 观察项（非缺陷）

30 分钟窗对挂起席偏宽（今日三次挂起各耗约 5 分钟即人工处决）——窗值
可配（ProgressTimeout），运营如嫌慢调小即可，不动码。

**销号**：D-76、D-82。

**结论：通过。合入 master，随下一班部署生效。**
