# 判决：驳回（T15 一骑）——CLI 挂径只挂链接目录不解符号链接，三壳两壳仍死

支 `t/t15-cage-live-fixes` ref `4df716a` 基 master。审计日 2026-11-02。

## 逐锚对账

| 锚 | 判 | 证据 |
|---|---|---|
| extra_mount 绝对径闸 | PASS | seat.go 新增 src 段 `filepath.IsAbs` 闸，与 mask 同款静默跳过 |
| B1 锁取释配对 | PASS | dispatch.go deployCommitted defer 配对；TestT15_LockLeak 断言取2释0（claim前死→onRunDead释双锁）；两包 159 测绿（复算） |
| 笼内壳 CLI 通径（方案b）| **未达** | `LookPath(bin)`→`filepath.Dir` 挂 ro——**不解 symlink**。宿主实证：`~/.local/bin/devin`→`~/.local/share/devin/cli/_versions/3000.11.234/bin/devin`；`~/.npm-global/bin/claude`→`~/.npm-global/lib/node_modules/@anthropic-ai/claude-code/bin/claude.exe`——两真身目录全不在挂载集→笼内悬链→`executable not found` 原样复炸。唯 opencode（bin 目录即 ELF 本体）能活 |

## 驳回项（一修即可）

`cageArgv` CLI 挂径加 `filepath.EvalSymlinks` 解链：**链接目录＋真身目录双 ro 挂**
（PATH 查名要链接目录，解释器/本体要真身目录——缺一即死）。
反向锚：symlink 型 CLI（构造假 bin 目录软链到真 ELF）断言两目录俱在 argv。

## 附记

- shebangOf 对三壳全是死径（三壳俱 ELF 无 shebang）——无害留着，脚本壳来了才用得上
- 挂 `~/.local/bin` 整目录 ro 暴露面可议——本期可忍，记档（席笼本就是半信任面）
- 2/3 壳在挂载集外是**静态可判**事实，无需等活体——驳回不误伤
