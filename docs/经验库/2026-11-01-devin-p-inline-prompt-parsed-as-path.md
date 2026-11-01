日期：2026-11-01
席位：CEO（手工派发径）
现象：`devin -p -c --sandbox "<提示词>"` 报 `--print [<PROMPT>] cannot be used with [PATH]...`——内联提示词被解析成位置路径参数
一句话教训：devin 无头一律用 `--prompt-file`，内联提示词与 `-c`/路径参数会撞

## 环境
devin 3000.11.234

## 复现
任何 `-p` ＋内联文本＋`-c` 组合即触发（EXIT=2）

## 解法/绕法
提示词全部落临时文件走 `--prompt-file /tmp/xxx.md`；续跑指令同理

## 引用
`t0001.devin-audit-continue.log`
