日期：2026-11-01
席位：CEO（手工派发径）
现象：`bwrap --ro-bind / / echo` 报 `setting up uid map: Permission denied`——devin sandbox 下 exec 全废的根因即此
一句话教训：Ubuntu 24.04 默认 `kernel.apparmor_restrict_unprivileged_userns=1`，bwrap 型沙箱全灭；诊断一行命令见下

## 环境
Ubuntu 24.04，kernel 6.8.0-147-generic

## 复现
```
bwrap --ro-bind / /usr/bin/echo ok        # → setting up uid map: Permission denied
sysctl kernel.apparmor_restrict_unprivileged_userns   # → 1
```

## 解法/绕法（**待业主明示，CEO 不擅动系统**）
- 方案 A（快、面宽）：`sudo sysctl kernel.apparmor_restrict_unprivileged_userns=0`＋持久化 `/etc/sysctl.d/`——恢复 23.10 前默认态；代价＝全系统非特权进程可用 userns（攻击面略增）
- 方案 B（慢、面窄，Ubuntu 官方推荐）：只为 devin/bwrap 二进制建 AppArmor profile 放行 userns，系统其余维持收紧
- 方案 C（现状，已受控）：不动系统——审计席 sandbox 只读＋stdout 判决（D4 本就禁它跑命令，exec 废恰好是完美形态）；施工席 dangerous＋worktree（业主点名授权过）
- 分类器已拦一次 CEO 的系统级放开尝试（理由正当：Security Weaken 未被业主点名）——动系统级缓解必须业主明示

## 引用
`t0001.devin-audit.log`（exec 全废实证）；ceo_memory 决策 15
