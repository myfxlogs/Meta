#!/usr/bin/env bash
# 前端板深验（票0023，决策71「半活→已验」）：scratch 自举端到端验收——
# 建独立 DB/socket 的 scratch 网关＋board_listen，用 meta board --connect
# （Connect 二进制径——与浏览器同一协议面）拉帧断言帧型；--live 对生产
# 127.0.0.1:9712 同径复验（token 只读 META_BOARD_TOKEN，禁令#19 不落码/仓）。
#
# 帧账（现码实测，board.go frames 映射）：
#   rule void 一次＝phase:void 事件(仅 TicketID)→1 枚 TICKET_UPSERT 帧＋
#   ruling 事件(TicketID＋CEO RunID)→SEAT_UPSERT＋TICKET_UPSERT 两帧
#   ——FULL＋一次 void 即 4 帧含 2 枚 TICKET_UPSERT，故 --n 缺省 4
#   （票面 n=3 之账未计 ruling 事件的 SEAT 帧；锚口径不变：FULL≥1＋UPSERT≥2）。
# CEO 裁决径：席位 env 的 META_RUN_TOKEN 是施工 role（ceoByToken 拒非
# CEO）；CEO token 唯一发放径＝meta ceo 交互面（token 不出进程），故 void
# 经 REPL 打点（stdin=FIFO——同一 CeoRule RPC）。
#
# 锚（票0023 §6，机器可判）：
#   A1  bash scripts/board-verify.sh → rc=0，stdout 末行
#       「BOARD_VERIFY OK full>=1 upsert>=2」
#   A2  BOARD_CONNECT=http://127.0.0.1:1 bash scripts/board-verify.sh → rc≠0
#   A3  trap 清场——退出后无残留 meta serve、TMP 已清
#   对抗 BOARD_NEED_UPSERT=99 → rc≠0（断言真卡帧数，非恒绿）
set -euo pipefail
cd "$(dirname "$0")/.."

NEED_FULL=${BOARD_NEED_FULL:-1}                 # 正锚阈值（对抗锚可抬到 99 必红）
NEED_UPSERT=${BOARD_NEED_UPSERT:-2}
N=${BOARD_N:-4}                                 # 收帧数：FULL＋SEAT＋2×UPSERT
WAIT_FULL_S=${BOARD_WAIT_FULL_S:-10}            # 等 FULL 帧上屏（同步订户口）
WAIT_EXIT_S=${BOARD_EXIT_WAIT_S:-15}            # 等 client 收满退场（票面 ≤15s）
LIVE_URL=${BOARD_CONNECT:-http://127.0.0.1:9712} # --live 连接点（A2 覆盖径）

say() { printf '%s\n' "$*" >&2; }               # 诊断走 stderr——stdout 只留判行

TMP=""
SERVE_PID=""; CLIENT_PID=""; CEO_PID=""
cleanup() {
  # set -e 下 trap 必须｜行｜全兜底——一条 kill 失败（进程已死）就中断
  # 清场＝泄漏 serve（首跑实证：client 先死→kill 失败→后半清场未执行）。
  [ -n "$CLIENT_PID" ] && kill "$CLIENT_PID" 2>/dev/null || true
  [ -n "$CEO_PID" ] && kill "$CEO_PID" 2>/dev/null || true
  [ -n "$SERVE_PID" ] && kill "$SERVE_PID" 2>/dev/null || true
  sleep 0.2
  [ -n "$CEO_PID" ] && kill -9 "$CEO_PID" 2>/dev/null || true
  [ -n "$SERVE_PID" ] && kill -9 "$SERVE_PID" 2>/dev/null || true
  exec 3>&- 2>/dev/null || true
  [ -n "$TMP" ] && rm -rf "$TMP" 2>/dev/null || true
  return 0
}
trap cleanup EXIT

# verdict <client_rc> <frames档>：rc=0＋FULL≥NEED_FULL＋UPSERT≥NEED_UPSERT 才绿。
verdict() {
  local rc=$1 frames=$2 full ups
  full=$(grep -c '^FULL ' "$frames" 2>/dev/null || true)
  ups=$(grep -c '^TICKET_UPSERT' "$frames" 2>/dev/null || true)
  full=${full:-0}; ups=${ups:-0}
  say "frames: full=$full upsert=$ups client_rc=$rc"
  if [ "$rc" -eq 0 ] && [ "$full" -ge "$NEED_FULL" ] && [ "$ups" -ge "$NEED_UPSERT" ]; then
    echo "BOARD_VERIFY OK full>=${NEED_FULL} upsert>=${NEED_UPSERT}"
    exit 0
  fi
  say "--- 证据：frames ---"; tail -20 "$frames" 2>/dev/null >&2 || true
  say "--- 证据：client.err ---"; tail -5 "$TMP/client.err" 2>/dev/null >&2 || true
  echo "BOARD_VERIFY FAIL full=$full need=$NEED_FULL upsert=$ups need=$NEED_UPSERT rc=$rc"
  exit 1
}

# wait_client <pid>：有界等退（≤WAIT_EXIT_S）；到点未退＝杀＋红。
wait_client() {
  local pid=$1 i
  for i in $(seq 1 $((WAIT_EXIT_S * 10))); do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 0.1
  done
  say "红：client ${WAIT_EXIT_S}s 未收满 $N 帧——杀"
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  say "--- 证据：frames ---"; tail -20 "$TMP/frames" 2>/dev/null >&2 || true
  echo "BOARD_VERIFY FAIL rc=timeout(${WAIT_EXIT_S}s)"
  exit 1
}

if [ "${1:-}" = "--live" ]; then
  # 生产复验径：只读拉流，不动生产任何面；token 只从 env 读（禁令#19）。
  TOKEN=${META_BOARD_TOKEN:-}
  [ -n "$TOKEN" ] || { say "红：--live 需 META_BOARD_TOKEN（env；不落码/仓）"; exit 1; }
  BIN=${META_BIN:-/opt/meta/bin/meta}
  TMP=$(mktemp -d /tmp/board-verify.XXXXXX)
  say "live: $BIN board --connect $LIVE_URL --n $N"
  "$BIN" board --connect "$LIVE_URL" --token "$TOKEN" --n "$N" \
    >"$TMP/frames" 2>"$TMP/client.err" &
  CLIENT_PID=$!
  wait_client "$CLIENT_PID"
  RC=0; wait "$CLIENT_PID" || RC=$?
  verdict "$RC" "$TMP/frames"
fi

# ---------- scratch 自举径 ----------
REPO=$PWD
TMP=$(mktemp -d /tmp/board-verify.XXXXXX)
DB=$TMP/meta.db; SOCK=$TMP/meta.sock; RUND=$TMP/run; KEY=$RUND/vault.key
META=$TMP/meta-bin
PORT=$(python3 -c 'import socket
s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')
say "scratch: tmp=$TMP board=127.0.0.1:$PORT"

go build -o "$META" ./cmd/meta

# ① token 落种（值走 stdin 既定径；key 缺则 put 自建——现码 LoadOrCreateVaultKey）
printf 'tok\n' | "$META" secret put --db "$DB" --key "$KEY" secret.board.token 2>"$TMP/secret.err"

# ② scratch 网关（--config 显式指仓内配置：二进制在 TMP，缺省路径解析不到；
#    board_listen 旗标压配置行——绝不碰生产 9712）
"$META" serve --db "$DB" --socket "$SOCK" --rundir "$RUND" \
  --board-listen "127.0.0.1:$PORT" --config "$REPO/config/gateway.textproto" \
  >"$TMP/serve.log" 2>&1 &
SERVE_PID=$!
for i in $(seq 1 100); do [ -S "$SOCK" ] && break; sleep 0.1; done
[ -S "$SOCK" ] || { say "红：scratch 网关 10s 未起"; tail -20 "$TMP/serve.log" >&2; exit 1; }

# ③ 先立票 vfy-a（先于接入→FULL 帧含非终态票）
"$META" ticket new --socket "$SOCK" --id vfy-a --title 验A >"$TMP/ticket-a.log" 2>&1

# ④ CEO 裁决通道：meta ceo REPL（stdin=FIFO——CEO token 唯一发放径在
#    交互面，不出进程；rule 打点即同一 CeoRule RPC）。先于接入开 run→
#    CEO 席进 FULL，接入后不再有开 run 噪声帧。
FIFO=$TMP/ceo.in
mkfifo "$FIFO"
exec 3<>"$FIFO"                                  # O_RDWR 持写端——REPL 读开不阻塞
"$META" ceo --socket "$SOCK" <"$FIFO" >"$TMP/ceo.out" 2>"$TMP/ceo.err" 3>&- &
CEO_PID=$!
for i in $(seq 1 100); do grep -q '已开 run=' "$TMP/ceo.err" 2>/dev/null && break; sleep 0.1; done
grep -q '已开 run=' "$TMP/ceo.err" 2>/dev/null \
  || { say "红：CEO 交互面 10s 未开"; tail -5 "$TMP/ceo.err" "$TMP/serve.log" >&2; exit 1; }

# ⑤ 板客户端（Connect 二进制径，与浏览器同一协议面）
CONNECT=${BOARD_CONNECT:-http://127.0.0.1:$PORT}
"$META" board --connect "$CONNECT" --token tok --n "$N" \
  >"$TMP/frames" 2>"$TMP/client.err" 3>&- &
CLIENT_PID=$!

# 等 FULL 上屏＝订户口已立（先订后装，board.go serve）——此后事件不丢窗
for i in $(seq 1 $((WAIT_FULL_S * 10))); do
  grep -q '^FULL ' "$TMP/frames" 2>/dev/null && break
  kill -0 "$CLIENT_PID" 2>/dev/null || break    # client 已死（反锚径）——不再空等
  sleep 0.1
done
grep -q '^FULL ' "$TMP/frames" 2>/dev/null || say "warn：未见 FULL 帧（client 已死？反锚径预期红）"

# ⑥ 变化帧泵：立 vfy-b→void vfy-b→void vfy-a（FULL＋2 upsert 达标；vfy-a 为兜底）
"$META" ticket new --socket "$SOCK" --id vfy-b --title 验B >"$TMP/ticket-b.log" 2>&1
ceo_cmd() {
  echo "$1" >&3 2>/dev/null || { say "红：CEO REPL 写入失败（已死？）"; \
    tail -5 "$TMP/ceo.err" "$TMP/ceo.out" >&2; exit 1; }
}
ceo_cmd 'rule void vfy-b confirm'
ceo_cmd 'rule void vfy-a confirm'

# ⑦ 有界等 client 收满退场→断言
wait_client "$CLIENT_PID"
RC=0; wait "$CLIENT_PID" || RC=$?

# REPL 收口（quit＋关写端＝EOF 双保险）
echo 'quit' >&3 2>/dev/null || true
exec 3>&- 2>/dev/null || true
for i in $(seq 1 50); do kill -0 "$CEO_PID" 2>/dev/null || break; sleep 0.1; done

verdict "$RC" "$TMP/frames"
