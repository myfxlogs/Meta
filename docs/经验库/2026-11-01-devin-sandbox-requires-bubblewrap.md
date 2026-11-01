日期：2026-11-01
席位：CEO（手工派发径）
现象：devin `--sandbox` 起不来——bubblewrap 未装即退（EXIT=1）；且 `--sandbox` 隐含 autonomous 档，`--permission-mode smart` 被忽略
一句话教训：sandbox 不是独立档位，是「危险档＋OS 隔离」；没有 bwrap 就没有中间路线

## 环境
Ubuntu（Linux 6.8），devin 3000.11.234

## 复现
`devin -p --sandbox ...` → `Sandbox initialization failed: bubblewrap (bwrap) is not installed`；前一行 `Warning: --sandbox always uses the autonomous permission mode; ignoring --permission-mode smart`

## 解法/绕法
- 本次：不用 sandbox，走 dangerous＋worktree 隔离
- **已解决（2026-11-01 当日）**：业主 terminal 授权安装，`bubblewrap 0.9.0` 已就位——后续施工席起法升级为 `--sandbox`（自主档＋OS 隔离，exec 只许写 workspace）；票 0001 在席那次来不及换档，dangerous＋worktree 隔离跑完

## 引用
日志 `~/meta-worktrees/t0001.devin.log`；ceo_memory 决策 4
