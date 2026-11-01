日期：2026-11-01
席位：施工（票0001）
现象：selfcheck.sh 全仓禁式 grep 命中数百条 Ticker/Sleep/json——全在 worktree 本地 `.gomodcache/`（GOMODCACHE 被 shell 指到 worktree 内）里的第三方源码
一句话教训：机械闸的扫描面必须钉「本仓源码」，别把依赖缓存/vendor 当被告——R2/R3 约束 meta 自有码，不约束依赖内部实现

## 环境
GOMODCACHE=/home/mluser/meta-worktrees/t0001/.gomodcache（shell 注入），仓内出现依赖源码树

## 复现
`grep -rn --include='*.go' -E 'time\.Ticker' .` 从 `.` 扫 → grpc/modernc/x/tools 全命中

## 解法/绕法
grep 加 `--exclude-dir=.git --exclude-dir=.gomodcache --exclude-dir=.gopath --exclude-dir=vendor --exclude-dir=node_modules`；同坑连带 `.gocache/` 进 `.gitignore`（GOCACHE 也可能被注到 worktree）。依赖内部的 Ticker/Sleep/json 属禁令#1 边界条款「第三方库内部保活/重试不受此限」，不查。

## 引用
票0001 验收锚3；scripts/selfcheck.sh；commit（本交付）
