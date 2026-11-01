日期：2026-11-01 / 席位：施工（票0003）/ 现象：回滚演练 `pkill serve` 后守护不熔断——serve 净退（exit 0）守护语义判定「正常停机」直接收工 / 一句话教训：模拟崩溃用 SIGKILL 或非零退码；SIGTERM 在守护语义里＝管理员叫停，不是崩

## 环境
`meta supervise` 守护 `bin/meta serve`；演练要制造「连崩」触发熔断回滚。

## 复现
```
pkill -f "bin/meta serve"   # 默认发 SIGTERM
# serve: 收信号 terminated → exit 0 → supervise 视净退 → 自己也退 0
# 后果：没有崩溃计数、没有熔断、没有回滚；演练等到超时红
```

## 解法
模拟崩死一律 `pkill -9`（SIGKILL 不可捕＝真崩）或让子进程非零退；守护语义对照表：

| 子进程死法 | supervise 判读 | 动作 |
|---|---|---|
| SIGKILL/段错误/非零退 | 崩溃 | 计数+退避重起，达 burst 熔断回滚 prev |
| SIGTERM→exit 0 | 净退 | 跟随退 0（systemd 不误判 failure）|
| 守护自身收 SIGTERM | 停机令 | 转发 SIGTERM 给子进程后退 0 |

同名坑在 systemd 侧：`systemctl kill -s TERM` 触发的是 `Restart=` 语义的「正常退」，要测 `on-failure` 也得用 SIGKILL/非零。

## 引用
票 0003；`scripts/upgrade-drill.sh` 第二轮失败实录；`internal/supervise/supervise.go` runChild。
