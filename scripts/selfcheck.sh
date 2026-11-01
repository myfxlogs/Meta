#!/usr/bin/env bash
# 禁式自检（票0001 验收锚3）：红线 R2/R3 的机械闸。
# 命中任一禁式＝退出码非 0。零命中＝0。
# 扫描范围＝本仓源码（排除模块缓存/vcs/第三方目录——R2/R3 约束 meta 自有码，
# 不约束依赖内部实现，见禁令#1 边界条款）。
set -u
cd "$(dirname "$0")/.."

fail=0

hit() { # $1=描述 $2=grep 结果
  if [ -n "$2" ]; then
    echo "selfcheck: 禁式命中[$1]"
    echo "$2"
    fail=1
  fi
}

GREPEXCL="--exclude-dir=.git --exclude-dir=.gomodcache --exclude-dir=.gopath --exclude-dir=vendor --exclude-dir=node_modules"

# R2：禁周期任务/禁心跳——Ticker/Tick/Sleep 全仓零命中（含测试；一次性闹钟走 AfterFunc/After）。
hit "time.Ticker/time.Tick" "$(grep -rn $GREPEXCL --include='*.go' -E 'time\.(NewTicker|Ticker|Tick\()' . || true)"
hit "time.Sleep/无界等待" "$(grep -rn $GREPEXCL --include='*.go' -E 'time\.Sleep' . || true)"

# R3：禁 JSON 进 meta 内部——encoding/json 全仓零命中（adapter/ 尚不存在）。
hit "encoding/json" "$(grep -rn $GREPEXCL --include='*.go' -E '"encoding/json"' . || true)"

# 无界 for{...sleep} 巡检由上面的 time.Sleep 全禁条覆盖（全仓含测试零 Sleep）。
exit $fail
