#!/usr/bin/env bash
# meta 部署步（项目配方 deploy_cmd，票0010）：仓内固定件——
# 构建候选 → meta upgrade 换装三件套晋升（候选 install＋prev 留底＋重启＋
# 健康自检）。以 run_as=mluser 跑（/opt/meta 属主）；票码从不进本步。
set -euo pipefail
cd "$(dirname "$0")/.."

NEW_BIN=${META_DEPLOY_BIN:-bin/meta.new}
INSTALL=${META_INSTALL:-/opt/meta/bin/meta}
RESTART=${META_RESTART_CMD:-"systemctl --user restart meta-gateway"}
CHECK_SOCK=${META_GATEWAY_SOCK:-/opt/meta/run/meta.sock}

mkdir -p bin
stamp="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)-deploy"
go build -ldflags "-X main.buildStamp=${stamp}" -o "$NEW_BIN" ./cmd/meta

# upgrade 三件套：预检 version→prev 快照→原子晋升→重启→socket 自检，
# 自检红自动回退 prev（见 internal/upgrade + 技能《二进制换装三件套》）。
"$NEW_BIN" upgrade --new "$NEW_BIN" --install "$INSTALL" \
  --restart-cmd "$RESTART" --check-socket "$CHECK_SOCK"
