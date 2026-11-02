#!/usr/bin/env bash
# T21 五段迁移（四裁定两必补）：mluser→metaseat 网关降权。
# 分段执行：t21-migrate.sh seg1|seg2|...|seg5|rollback|verify
# 每段幂等＋自验；段间停手检查（CEO 判决：逐段验真逐段报）。
set -euo pipefail
META=/opt/meta; HOME_MS=/home/metaseat
log(){ echo "[t21] $*"; }

seg1(){ # 立户 -m nologin（裁定2）
  id metaseat 2>/dev/null || useradd -r -m -d "$HOME_MS" -s /usr/sbin/nologin metaseat
  loginctl enable-linger metaseat   # 裁定1 user 域常驻
  id metaseat && log "seg1 ok"; }

seg2(){ # shellbin 副本（裁定3 禁穿家）＋鉴权档（必补A）
  mkdir -p $META/shellbin
  for n in devin claude opencode; do
    src=$(command -v $n || ls /home/mluser/.local/bin/$n /home/mluser/.npm-global/bin/$n /home/mluser/.opencode/bin/$n 2>/dev/null | head -1) || true
    [ -n "$src" ] && cp -a "$src" $META/shellbin/$n && chmod 755 $META/shellbin/$n && log "shellbin $n <- $src"
  done
  # 必补A：鉴权档副本（真调验证前必备）
  for d in .local/share/devin .claude .config/opencode; do
    [ -d /home/mluser/$d ] && cp -a /home/mluser/$d "$HOME_MS/${d%/*}" 2>/dev/null || true
  done
  chown -R metaseat:metaseat $META/shellbin "$HOME_MS" 2>/dev/null || true
  log "seg2 ok"; }

seg3(){ # 单元随迁 user 域（裁定1）
  mkdir -p $HOME_MS/.config/systemd/user $HOME_MS/.config/meta
  cp /home/mluser/.config/systemd/user/meta-*.service $HOME_MS/.config/systemd/user/ 2>/dev/null || true
  cp /home/mluser/.config/meta/gateway-env $HOME_MS/.config/meta/ 2>/dev/null || true
  # PATH 换 shellbin 径（壳鉴权 HOME=metaseat）
  sed -i "s|/home/mluser/.local/bin|/opt/meta/shellbin|;s|/home/mluser/.npm-global/bin||;s|/home/mluser/.opencode/bin||" $HOME_MS/.config/systemd/user/meta-gateway.service
  chown -R metaseat:metaseat $HOME_MS/.config
  log "seg3 ok"; }

seg4(){ # 写域迁（裁定4 只迁 run/）＋docker 组（必补B）
  usermod -aG docker metaseat
  touch $META/run/.meta-gateway.migrated   # 双单元防双写闸
  systemctl --user -M metaseat@ daemon-reload 2>/dev/null || sudo -u metaseat XDG_RUNTIME_DIR=/run/user/$(id -u metaseat) systemctl --user daemon-reload
  chown -R metaseat:metaseat $META/run
  log "seg4 ok"; }

seg5(){ # 启服验真
  systemctl --user -M metaseat@ enable --now meta-gateway meta-caster 2>/dev/null || \
    sudo -u metaseat XDG_RUNTIME_DIR=/run/user/$(id -u metaseat) systemctl --user enable --now meta-gateway meta-caster
  systemctl --user stop meta-gateway meta-caster 2>/dev/null || true  # mluser 侧停旧
  sleep 2; $META/bin/meta version >/dev/null && log "seg5 启服——跑 verify"; }

rollback(){ # §4 回退
  systemctl --user -M metaseat@ stop meta-gateway meta-caster 2>/dev/null || true
  rm -f $META/run/.meta-gateway.migrated
  chown -R mluser:mluser $META/run
  systemctl --user start meta-gateway meta-caster
  log "回退完成"; }

verify(){ # 验真清单
  log "== metaseat 户 =="; id metaseat
  log "== docker 组 =="; id metaseat | grep -q docker && echo "docker 组 ok"
  log "== run 归属 =="; stat -c '%U:%G %a %n' $META/run/vault.key $META/run/meta.db | head -3
  log "== 单元态 =="; systemctl --user -M metaseat@ is-active meta-gateway 2>/dev/null || true
  log "== socket =="; $META/bin/meta phase --socket $META/run/meta.sock 2>&1 | head -3; }

case "${1:-}" in seg1|seg2|seg3|seg4|seg5|rollback|verify) "$1" ;; *) echo "usage: $0 seg1|seg2|seg3|seg4|seg5|rollback|verify" ;; esac
