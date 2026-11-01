日期：2026-11-01
席位：CEO（降权座搭建）
现象：metaseat（降权用户）下跑 devin 连过三坎——①`env: devin: Permission denied`（`~/.local/share` 700 卡整条遍历链）②`Login canceled`（凭据发现随 HOME，metaseat 名下没有）③`Refusing to run in an untrusted workspace`（信任库不随 config.json 走）
一句话教训：降权用户的 CLI 通路＝**遍历链＋凭据副本＋信任库**三件，一件不通整条死

## 环境
Ubuntu 24.04，devin 3000.11.234，席位用户 metaseat

## 解法（已成固定位）
1. **遍历链**：`chmod 711` 放 `/home/mluser`、`~/.local`、`~/.local/share`（只放行遍历不放列目录）；CLI 树 `chmod -R o+rX`（`~/.local/bin`、`~/.local/share/devin`、`~/.npm-global`、`~/.opencode`）——**先收紧再开放**：`~/.claude`→700（转录含密码！）、`/opt/meta-archive`、`/opt/o/secrets`、`/opt/o-research`→711
2. **凭据副本制**：`cp credentials.toml` 到 `/home/metaseat/.local/share/devin/`（600，metaseat 属主）——席位有独立凭据副本，不写回 mluser 树；网关时代由 `Secret(name)` RPC 替代（凭证清单正解）
3. **信任库**：真身在 `~/.local/share/devin/cli/trusted_workspaces.json`（config.json 里写 `respect_workspace_trust:false` 无效、CLI 旗不存在）；metaseat 版 `trusted_paths:["/home/metaseat"]`——**只信自己家，不信 /opt 与 /**

## 验证矩阵（复测时照抄）
隔离（metaseat 应全拒读）：`~/.ssh/id_ed25519`、`~/.claude`、`/opt/meta-archive/feishu-manage.env`、`/opt/o/secrets/o_master_key`、`/opt/o-research/env`、`~/.local/share/rtk`
可达：三 CLI `--version`；活体：`devin -p "只回答两个字：正常"`

## 引用
ceo_memory 决策 19；债务台账 D-10
