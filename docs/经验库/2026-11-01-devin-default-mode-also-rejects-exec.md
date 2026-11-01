日期：2026-11-01
席位：CEO（审计派发）
现象：devin 无头**默认档**（auto）与 smart 档同病——席位起手想跑命令即被拒，草草收场只留一句开场白（t0002 正审首跑实录）
一句话教训：无头审计席的提示词**起手就钉死「只用只读文件工具（读/搜/glob），不跑任何命令；git 态直读 .git 内文件」**——不要等它撞了再续跑

## 环境
devin swe-2-max，metaseat 降权座，`-p --prompt-file`，默认 permission mode

## 复现
正审首跑：日志即 `warning: rejected a tool call that requires confirmation`，stdout 只剩开场白一句

## 解法/绕法
- 同会话续跑＋「只读文件工具」指令（`-p -c --prompt-file`，本条已验证）
- **固化**：此后每张票的审计注入前缀，材料节后直接带这段只读指令（模板已含）；壳的档位结论汇总见同日三条无头档位实测（devin/claude/opencode）
- 一次性教训链齐了：smart 哑退／default 拒 exec／sandbox Write 拒／-p 内联撞路径——devin 无头四坑全谱在库

## 引用
`t0002.audit-err.log`；ceo_memory 决策 22

## 补记（同日第四次实证，CEO）
t0003 正审第四次死于同因（提示词已点名「禁 tail/grep/cat/ls/sqlite3、只用原生文件工具」仍无效）——**devin 只读审计形态在本环境实证不可靠，弃用**。审计席正审改 harness 原生子代理（claude，默认权限，读文件原生），devin swe-2-max 保留为预审/诊断席。路由偏离已报备业主（决策 36）。
