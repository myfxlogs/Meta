日期：2026-11-01
席位：CEO（手工派发径）
现象：devin `-p` 无头 + `--permission-mode smart`——需确认的工具调用被直接拒，席位读完票面即哑退，EXIT=0、worktree 零改动
一句话教训：devin 无头径的中间权限档是假的——能干活的只有 dangerous；要中间档先走 sandbox

## 环境
devin 3000.11.234，`-p --prompt-file`，无 TTY

## 复现
`devin -p --prompt-file <f> --permission-mode smart` → 日志出 `warning: rejected a tool call that requires confirmation. Running in non-interactive mode. Use --permission-mode dangerous...`，随后正常退出

## 解法/绕法
- 本次：业主 terminal 授权改用 dangerous（ceo_memory 决策 4）
- 长期：阶段 1 壳适配器把「无交互可用档位」列为实测项（呼应风险清单「壳适配器未知面」）——三壳各自哪些档位能无头干活，起会话前就得知道，不能烧一次会话来试

## 引用
日志 `~/meta-worktrees/t0001.devin.log`；ceo_memory 决策 4；票 0001 派发过程
