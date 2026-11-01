日期：2026-11-01
席位：CEO（手工派发径）
现象：`claude -p --dangerously-skip-permissions` 起审计席同样被 Claude Code 安全闸拦——理由：业主的 bypass 授权只点名过施工席（devin）的档位，笼统的「最高权限」不算点名
一句话教训：无审批自治代理的授权必须**逐席位、逐命令**点名；更好的解不是去授权，而是选不需要它的形态

## 环境
Claude Code CLI，auto 权限模式

## 复现
`claude -p --dangerously-skip-perpermissions ...`（后台 Bash）→ classifier 拒绝：Create Unsafe Agents

## 解法/绕法
- 审计席改用 **harness 原生子代理**（Agent 工具）：默认权限、只能读、判决出 stdout——反而更贴合 D4 审计卡「读机器记录、不亲自跑」的设计；独立性成立（全新实例、无施工上下文、异壳）
- **固化**：审计席起法＝harness 子代理（只读+stdout 判决）；施工席起法＝devin `-p`（dangerous，业主点名授权过）＋worktree 隔离。两者不再混用同一档位逻辑
- 若未来确需 claude CLI 无头全权（如施工备援），须业主在 settings 里加显式 permission rule，不靠口头

## 引用
ceo_memory 决策 9；施工席先例见 `2026-11-01-devin-smart-mode-headless-mute-exit.md`
