#!/usr/bin/env bash
# 真壳冒烟（票0002 验收锚7/8，live——烧真钱，与默认 go test 严格分离）。
# 用法: bash scripts/smoke-shells.sh
# 判据: 脚本退出码。报告落 <rundir>/smoke-report.txt，用量读不到如实标「不可得」。
set -u
cd "$(dirname "$0")/.."

RUND="${SMOKE_DIR:-$(mktemp -d /tmp/meta-smoke.XXXXXX)}"
SOCK="$RUND/meta.sock"
DB="$RUND/meta.db"
META="$RUND/meta-bin"
PROMPT="$RUND/prompt.txt"
WORKDIR="$RUND/seatwork"
REPORT="$RUND/smoke-report.txt"
GWPID=""

mkdir -p "$WORKDIR"
printf 'Reply with exactly this line and nothing else: SMOKE_OK' > "$PROMPT"

echo "smoke: rundir=$RUND" | tee "$REPORT"

go build -o "$META" ./cmd/meta || { echo "build 失败" | tee -a "$REPORT"; exit 1; }

"$META" serve --db "$DB" --socket "$SOCK" --rundir "$RUND/run" --stagger 1s >>"$RUND/gateway.log" 2>&1 &
GWPID=$!
trap 'kill $GWPID 2>/dev/null' EXIT

# 等 socket（有界）
for i in $(seq 1 50); do [ -S "$SOCK" ] && break; sleep 0.1; done
[ -S "$SOCK" ] || { echo "网关未起" | tee -a "$REPORT"; cat "$RUND/gateway.log"; exit 1; }

# wait_phase <ticket> <相位> <秒> ——脚本侧轮询合法（驱动者不是系统件）。
wait_phase() {
  local want=$2 lim=$(( ${3:-180} * 10 )) i
  for i in $(seq 1 "$lim"); do
    local p
    p=$("$META" phase --socket "$SOCK" --ticket "$1" 2>/dev/null | awk '{print $2}')
    [ "$p" = "$want" ] && return 0
    sleep 0.1
  done
  return 1
}

# usage_of <ticket> ——budget total；0=不可得如实报。
usage_of() {
  "$META" budget --socket "$SOCK" --ticket "$1" 2>/dev/null | awk '{print $5}' | cut -d= -f2
}

fail=0

# ---------- 锚7：三壳各起一真会话 ----------
for SH in devin claude opencode; do
  T="smoke-$SH"
  "$META" ticket new --socket "$SOCK" --id "$T" --title "smoke $SH" >/dev/null
  "$META" dispatch --socket "$SOCK" --ticket "$T" --role 施工 \
    --prompt-file "$PROMPT" --workdir "$WORKDIR" --prefer "$SH" >/dev/null
  if wait_phase "$T" "待审" 300; then
    U=$(usage_of "$T")
    RUNS=$("$META" runs --socket "$SOCK" --ticket "$T")
    FIRST=$(echo "$RUNS" | head -1 | sed 's/\tshell=/ shell=/')
    if [ "${U:-0}" -gt 0 ]; then UTXT="$U tokens"; else UTXT="用量不可得"; fi
    echo "锚7[$SH] 通过→待审 exit0 usage=$UTXT | $FIRST" | tee -a "$REPORT"
  else
    P=$("$META" phase --socket "$SOCK" --ticket "$T" 2>/dev/null)
    echo "锚7[$SH] 未达待审（$P）" | tee -a "$REPORT"
    "$META" runs --socket "$SOCK" --ticket "$T" | tee -a "$REPORT"
    "$META" events --socket "$SOCK" --ticket "$T" | tail -8 | tee -a "$REPORT"
    fail=1
  fi
done

# ---------- 锚8：spawn devin→SIGKILL→自动换 claude 重起→正常收尾 ----------
T="smoke-switch"
"$META" ticket new --socket "$SOCK" --id "$T" --title "换壳一镜到底" >/dev/null
"$META" dispatch --socket "$SOCK" --ticket "$T" --role 施工 \
  --prompt-file "$PROMPT" --workdir "$WORKDIR" --prefer devin >/dev/null
if wait_phase "$T" "施工中" 120; then
  PID=$("$META" runs --socket "$SOCK" --ticket "$T" | tail -1 | awk '{print $3}' | cut -d= -f2)
  echo "锚8: devin 席 pid=$PID 施工中→SIGKILL(进程组)" | tee -a "$REPORT"
  kill -9 -- "-$PID" 2>/dev/null || kill -9 "$PID" 2>/dev/null
  if wait_phase "$T" "待审" 300; then
    R=$("$META" runs --socket "$SOCK" --ticket "$T")
    N=$(echo "$R" | wc -l)
    D=$(echo "$R" | head -1)
    C=$(echo "$R" | tail -1)
    echo "锚8: $N runs | 死席: $D | 收尾席: $C" | tee -a "$REPORT"
    echo "$C" | grep -q "shell=claude" && echo "$C" | grep -q "exit=0" \
      && echo "锚8 通过：devin 死→claude 重起→正常交付" | tee -a "$REPORT" \
      || { echo "锚8 部分：收尾席非预期" | tee -a "$REPORT"; fail=1; }
  else
    echo "锚8 未达待审" | tee -a "$REPORT"
    "$META" events --socket "$SOCK" --ticket "$T" | tail -10 | tee -a "$REPORT"
    fail=1
  fi
else
  echo "锚8: devin 席未起（施工中未达）" | tee -a "$REPORT"
  "$META" events --socket "$SOCK" --ticket "$T" | tail -8 | tee -a "$REPORT"
  fail=1
fi

echo "smoke: 报告=$REPORT 网关日志=$RUND/gateway.log 席位日志=$RUND/run/logs/"
kill $GWPID 2>/dev/null
exit $fail
