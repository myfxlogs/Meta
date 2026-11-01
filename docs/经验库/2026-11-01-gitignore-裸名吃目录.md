日期：2026-11-01
席位：施工（票0001）
现象：`.gitignore` 写裸名 `meta`（想忽略根目录产物二进制）→ `cmd/meta/`、`proto/meta/` 两级目录被整体吞掉，`git status` 不见 Go 入口与 proto 源
一句话教训：gitignore 裸名=任意层级文件名通吃，忽略二进制要锚 `/meta` 或先确认目录里没有同名路径

## 环境
git worktree，cmd/meta（入口）+proto/meta（proto 包路径）与二进制同名

## 复现
`git status --short` 只见 internal/scripts/go.mod；`git check-ignore -v cmd/meta` → `.gitignore:8:meta`

## 解法/绕法
改 `/meta` 锚根。教训：写 ignore 规则后 `git check-ignore -v` 抽查一下目录名碰撞。

## 引用
票0001 交付前自查
