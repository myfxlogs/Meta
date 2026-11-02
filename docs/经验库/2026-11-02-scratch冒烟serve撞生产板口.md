日期：2026-11-02
席位：施工（票0021）
现象：scratch 冒烟起 `meta serve` 用仓内 config/gateway.textproto——其中
`board_listen: "127.0.0.1:9712"` 与生产网关已挂板口冲突，serve 拒起
（bind: address already in use），socket 永不出现。
一句话教训：**scratch 冒烟用生产配置必须带 --board-listen 覆盖空闲口**
（或 --board-listen "" 关板）——配置文件与生产同根，旗标才是冒烟隔离位。

## 复现
`meta serve --config config/gateway.textproto --db $TMP/db --socket $TMP/sock`
→ serve.log `board_listen 127.0.0.1:9712: bind: address already in use`。

## 解法
`--board-listen 127.0.0.1:<空闲口>`（脚本内取空闲端口）或 `--board-listen ""`。
票0023 board-verify 脚本同坑同法。

## 引用
票0021 meta reconcile 真网关冒烟；日志见 /tmp 冒烟目录（已清）。
