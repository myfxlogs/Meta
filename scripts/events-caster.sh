#!/usr/bin/env bash
# events-caster.sh —— 播报员薄壳（票0004）：引擎已晋升 `meta caster` 子命令，
# 本脚本只剩 exec——生产单元 ExecStart 零改动，新引擎随二进制换装生效。
exec /opt/meta/bin/meta caster \
  --socket /opt/meta/run/meta.sock \
  --routes /opt/meta/config/caster.textproto \
  --state-dir /opt/meta/run/caster \
  --run-dir /opt/meta/run \
  --notify-cmd /opt/meta/scripts/feishu-notify.sh
