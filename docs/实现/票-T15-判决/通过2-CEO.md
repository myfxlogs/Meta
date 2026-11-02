# 判决：通过（T15 二骑）——驳回单点全修，三雷俱灭

支 `t/t15-cage-live-fixes` ref `b965232`（骑1 `4df716a`→骑2）。2026-11-02。

## 逐锚对账

| 锚 | 判 | 证据 |
|---|---|---|
| EvalSymlinks 双目录挂 | PASS | seat.go：LookPath→EvalSymlinks→链接目录+真身目录双 ro 挂，seen 去重。devin/claude symlink 场景由此闭合（/home/mluser/.local/bin + _versions 目录俱挂） |
| 反向锚 symlink 断言 | PASS | TestT10_CageArgvSymlink：假 bin 软链→真 ELF，断言两 `-v dir:dir:ro` 俱在——正是驳回项钦定形态 |
| 骑1 采信面零动 | PASS | diff 只触 seat.go 挂径环+t_t10_test.go 新增测 |
| 测绿复算 | PASS | seat 包 22 测绿（worktree b965232 实跑）；selfcheck 申报采信（同构件骑1 已验） |

## 记债

- **D-86**：shebang 解释器目录挂径可能遮笼内系统目录（脚本壳
  `#!/usr/bin/env node`→挂宿主 /usr/bin:/usr/bin→笼内自带工具被影
  蔽+库失配）。三壳俱 ELF 触不到此径——立债待脚本壳进场再修
  （修法方向：挂非冲突径+PATH 前置，或系统目录白名单拒挂）。
- D-84/85 仍开（T7 旧债，与笼正交）。

## 启用序提醒（票面改法4，部署面件）

T15 并入后 config 复 `seat_image: "golang:1.27"` → 三壳各一席存活
>60s＋`docker ps` 实证笼内跑 → 再回部署波。本轮启用窗口建议
与 0034-p3（restart 杀全席）修复同批——restart 连坐未修前开笼，
部署 restart 照样灭笼内席。
