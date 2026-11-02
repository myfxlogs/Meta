# T25（外派·部署闸去席化）判决：通过——代码面；实弹验收条款在案（CEO 终端形态·决策 97，2026-11-02）

- 交付：支 `t/t25-cli-deploy` @ `31449b4`（worktree 合规）。五件 +139/−3。
  独立复算：build RC=0、`go test ./...` **290 绿/16 包**、selfcheck RC=0。

## 对账

| 项 | 判 |
|---|---|
| `meta deploy --ticket` → Dispatch CliDrive=true → 席进程 argv＝`meta seat-drive`（SelfPath 自身——零 AI 壳） | 绿——架构最优：不是旁路新写，是把席位“发步循环”换成同一二进制的机械循环 |
| seat-drive：env 凭证＋attach 重连闭包——deploy 步重启网关后断流重连续步 | 绿（单测面）；**实弹＝验收条款①③** |
| 步红退码 91＋尾因可读 | 绿 |
| 申报诚实：单票全程＋restart 重连未实跑 | 采信——E6 合规；实弹转 CEO 验收 |

## 验收条款（本判决生效条件）

下一单真部署由 CEO 亲跑 `meta deploy`：①全程 exec 事件四条齐＋零 AI 席位
行；②deploy 步重启网关后 CLI 重连落 probe＋done。两锚亲证后本票全绿归档。

## 观察项

批量闸（T4）触发径仍起 AI 席——接入 CliDrive 随下一班顺手（记 D-89）。

**结论：通过（代码面）。合入＋换装；实弹验收随下单部署执行。**
