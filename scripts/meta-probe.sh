#!/usr/bin/env bash
# meta 健康探针（项目配方 probe_cmd，票0010）：AdminGateway.Ping 一拨——
# 活即退 0；拨不通/超时非零退。部署时刻探针，非周期探活（R2/B3）。
set -euo pipefail
BIN=${META_BIN:-/opt/meta/bin/meta}
SOCK=${META_GATEWAY_SOCK:-/opt/meta/run/meta.sock}
TIMEOUT=${META_PROBE_TIMEOUT:-5s}
exec "$BIN" ping --socket "$SOCK" --timeout "$TIMEOUT"
