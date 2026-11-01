日期：2026-11-01
席位：CEO（手工派发径）
现象：bwrap 装好后 devin `--sandbox` 仍起不来——还缺 `socat`
一句话教训：devin 的 Linux 沙箱＝bwrap＋seccomp＋socat 三件套，装就一次装齐

## 环境
Ubuntu，devin 3000.11.234

## 复现
装 bubblewrap 后再跑 `devin -p --sandbox` → `Sandbox initialization failed: socat is not installed`

## 解法/绕法
- 已装：`bubblewrap 0.9.0`＋`socat`（均业主授权 sudo）——施工/审计无头席位的 `--sandbox` 路径至此打通
- 新机重建清单：`sudo apt-get install -y bubblewrap socat`（席位起不来的排障先查这两件）

## 引用
`2026-11-01-devin-sandbox-requires-bubblewrap.md`；ceo_memory 决策 6/10
