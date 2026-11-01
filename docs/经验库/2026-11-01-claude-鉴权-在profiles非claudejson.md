日期：2026-11-01
席位：CEO（降权座补钥匙）
现象：把 `.claude.json` 副本给 metaseat 后 claude 仍报「Not logged in」——真源在 `~/.claude/profiles/glm.json` 的 **env 块**（`ANTHROPIC_BASE_URL`＋`ANTHROPIC_AUTH_TOKEN`＋模型映射）
一句话教训：claude 壳的钥匙＝**env 令牌**，不在配置文件里——副本制要复制的是「可 source 的 env 文件＋profiles 目录」

## 环境
claude CLI 2.1.270，多 profile 制（deepseek/glm/opencode/s1/ss），网关=open.bigmodel.cn 的 anthropic 兼容端

## 复现
仅复制 `.claude.json` → `claude -p` 报 `Not logged in · Please run /login`（51ms 即退，非网络问题）

## 解法（固定位）
1. `/home/metaseat/.claude-env`（600，metaseat 属主）：`export ANTHROPIC_BASE_URL=…`＋`export ANTHROPIC_AUTH_TOKEN=…`——spawn 前 source
2. `/home/metaseat/.claude/profiles/` 副本（模型映射随行）
3. 验证口径：`claude -p "只回答两个字:正常"` 出「正常」
- **鉴权即拒（exit 63）的正确读法**：②b 配额/授权级——返修冒烟实录：claude ②b 标死→devin 递补交付，机制自证
- 网关时代：适配器从 `Secret(name)` 取值注入 spawn env（凭证清单正解）；env 副本文件是手工期等效物

## 引用
ceo_memory 决策 26；债务台账 D-10（claude 半边已闭合）；返修申报 1（环境缺口）
