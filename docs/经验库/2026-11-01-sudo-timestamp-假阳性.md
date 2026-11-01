日期：2026-11-01
席位：CEO（降权座搭建）
现象：逐条 `sudo -u metaseat test -r <敏感路径>` 输出「拒✓」——**全是假的**：sudo 时间戳过期，拒的是 sudo 自己，不是文件权限
一句话教训：验证隔离时先确认拒的是**权限**还是**sudo**——`sudo` 报错与 `test` 失败在 shell `||` 里长得一模一样

## 环境
Claude Code Bash（每调用新会话，sudo 时间戳不延续）

## 复现
隔一次调用后再 `sudo -u x test -r f` → stderr 出 `sudo: a terminal is required...` 而退出码非零，被 `||` 吃成「拒✓」

## 解法/绕法
单次喂密码进 root、内部用 `runuser -u metaseat --` 落权执行全部验证——root→runuser 不再要密码，验证一次成型；或每条 sudo 前 `echo <pw> | sudo -S -p ''`

## 引用
ceo_memory 决策 19
