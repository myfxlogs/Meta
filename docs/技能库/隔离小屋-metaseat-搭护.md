沉淀日：2026-11-01
来源：决策 19/20；经验库 metaseat 三坑＋sudo 假阳性
沉淀依据：六路隔离全拒＋三壳活体可达的验证矩阵实录（ceo_memory 决策 19）
适用：新机重建席位隔离层；D-10 凭据副本维护；任何「席位要碰本机」的新场景

# 隔离小屋（metaseat）搭护

## 建立一次性（新机照抄）
1. `useradd -m -s /bin/bash metaseat`（无 sudo 组）
2. **先收紧后开放**：`~/.claude`→700（转录有密码！）；`/home/mluser`、`~/.local`、`~/.local/share`、`/opt/meta-archive`、`/opt/o/secrets`、`/opt/o-research`→711
3. 开 CLI 树（只读）：`chmod -R o+rX ~/.local/bin ~/.local/share/devin ~/.npm-global ~/.opencode`；**开放后必扫树内敏感件**（credentials.toml 原件收 600）
4. 凭据副本三形：devin＝`credentials.toml` 副本；claude＝`.claude-env`（BASE_URL/TOKEN）＋`profiles/` 目录（真源在 `profiles/glm.json` env 块）；opencode 无需
5. 信任库：`trusted_workspaces.json` 只写自家路径（+需要读的工作区绝对路径）
6. 工作区：`/home/metaseat/worktrees/`

## 验证矩阵（改完必复跑）
- 隔离（全拒）：`~/.ssh/id_ed25519`、`~/.claude`、`/opt/meta-archive/feishu-manage.env`、`/opt/o/secrets/o_master_key`、`/opt/o-research/env`、`~/.local/share/rtk`
- 可达：三 CLI `--version`；devin 活体「只回答两个字：正常」
- **坑**：逐条 sudo 验证会因时间戳过期出假「拒✓」——单次喂密码进 root，内部 `runuser -u metaseat --` 执行全部

## 维护（D-10）
- devin token 轮换→同步副本；claude 换 profile/模型→刷新 `.claude-env`＋profiles 副本；网关激活后由 `Secret(name)` RPC 替代全部副本
