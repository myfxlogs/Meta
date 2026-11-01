#!/usr/bin/env bash
# 升级回滚演练（票0003 验收锚5）——隔离实例：独立 --db/--socket/--rundir 于
# 临时目录，全程不碰生产 meta-gateway 守护。
# 判据＝退出码。流程：build 好版（buildStamp=drill-good）→装 bin/meta 与
# bin/meta.prev→supervise 守护起 serve→植入坏二进制→杀 serve 子进程→
# 连崩熔断→自动回退 prev→serve 恢复。全程日志留档 $D/drill.log。
set -u
cd "$(dirname "$0")/.."

D=$(mktemp -d /tmp/meta-drill.XXXXXX)
LOG="$D/drill.log"
BIND="$D/bin"
META="$BIND/meta"
SOCK="$D/meta.sock"
SUP=""
say()  { echo "drill: $*" | tee -a "$LOG"; }
fail() { say "FAIL: $*"; tail -15 "$LOG" | sed 's/^/  /'; say "日志留档: $LOG"; exit 1; }

wait_sock() { # 有界等 socket（脚本侧轮询合法——驱动者不是系统件）
  for i in $(seq 1 "${2:-150}"); do [ -S "$1" ] && return 0; sleep 0.1; done
  return 1
}
alive() { "$META" runs --socket "$SOCK" --ticket __probe >/dev/null 2>&1; }

say "隔离实例 rundir=$D"
go build -ldflags "-X main.buildStamp=drill-good" -o "$D/meta-build" ./cmd/meta >>"$LOG" 2>&1 \
  || fail "build"
mkdir -p "$BIND"
cp "$D/meta-build" "$META"; cp "$D/meta-build" "$META.prev"
GOOD=$(sha256sum "$META" | cut -d' ' -f1)

# 起守护（burst=3 加速熔断；stable 大窗防误复位）
"$META" supervise --bin "$META" --db "$D/meta.db" --socket "$SOCK" --rundir "$D/run" \
  --burst 3 --stable 120s --backoff 200ms >>"$LOG" 2>&1 &
SUP=$!
trap 'kill "$SUP" 2>/dev/null; pkill -f "$BIND/meta serve" 2>/dev/null; say "日志留档: $LOG"' EXIT

wait_sock "$SOCK" || fail "serve 未起"
alive || fail "serve 不应答"
say "serve 起立（supervise pid=$SUP）"

# 植入坏二进制——tmp+rename 原子换装：直写 exec 中二进制撞 ETXTBSY。
printf '#!/bin/sh\nexit 7\n' > "$META.bad"; chmod +x "$META.bad"; mv "$META.bad" "$META"
say "已植入坏二进制 → 杀 serve 子进程触发链崩"

pkill -9 -f "$BIND/meta serve" || fail "serve 子进程未找到" # SIGKILL＝模拟崩死

ok=0
for i in $(seq 1 300); do
  if alive && [ "$(sha256sum "$META" | cut -d' ' -f1)" = "$GOOD" ]; then ok=1; break; fi
  sleep 0.1
done
[ "$ok" = 1 ] || fail "30s 内未恢复 serve / 未回退 prev"

v=$("$META" version 2>/dev/null || true)
[ "$v" = "meta drill-good" ] || fail "version 戳不符（应 drill-good）: $v"
grep -q "已回退" "$LOG" || fail "守护日志无回退记录"
grep -q "连崩熔断" "$LOG" || fail "守护日志无熔断记录"
say "植入坏版→熔断→自动回退 prev→serve 恢复：全绿"
say "日志留档: $LOG"
exit 0
