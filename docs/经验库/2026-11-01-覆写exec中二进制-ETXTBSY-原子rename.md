日期：2026-11-01 / 席位：施工（票0003）/ 现象：升级演练直写正在 exec 的二进制 → `Text file busy`，换装静默失败 / 一句话教训：换二进制一律 tmp+rename 原子换装，绝不直写 in-place

## 环境
Linux 6.8，`meta supervise` 守护着的 serve 子进程正在跑 `bin/meta`；演练脚本要把它换成坏二进制。

## 复现
```
printf '#!/bin/sh\nexit 7\n' > bin/meta   # 直接覆写
# → bash: bin/meta: Text file busy（ETXTBSY）
```
正在 exec 的可执行文件被内核锁着（etext），写/截断一律拒；cp、>、sed -i 全撞。且 `> a && chmod` 这类「先写后赋」顺序在失败时留下旧文件，演练会静默继续用**好**二进制——假绿。

## 解法
写入同目录临时文件再 `mv`（rename 原子换目录项，运行中进程照旧持旧 inode，无锁冲突）：
```
printf '...' > bin/meta.bad && chmod +x bin/meta.bad && mv bin/meta.bad bin/meta
```
`internal/upgrade` 的 `copyAtomic` 同构：tmp 文件 0755 + rename。生产升级/降级/演练三径通用。

## 引用
票 0003；`scripts/upgrade-drill.sh` 首跑失败实录（演练日志「Text file busy」）；`internal/upgrade/upgrade.go` copyAtomic。
