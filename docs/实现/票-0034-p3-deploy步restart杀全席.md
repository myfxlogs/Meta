# 票 0034-p3：deploy 步 restart 灭 meta-gateway cgroup 满门——部署席/upgrade 自检同亡，部署完成判定无径可落（0034 部署 24 连败 0 胜，幂等重试收敛失效，adjunct 连坐）

> 巡检席自立案（adjunct，不占相位不重派）。触发信号＝0034 `phase:deploy_failed`
> （事件 14416：对账行动者无响应——先核回滚态 run=baaabbd921ddef1e）。判**真**：
> 同签名 24 连发、deploy_ok/retro 零记录、实害可复跑。本轮巡检
> run=0fe8f896b577070f。

### 1. 目标

修「部署完成判定依赖必死之席」机制层缺口三处：

- **完成判定改态判**：deploy 收口改由 restart 后状态判定（装着二进制
  stamp＝期望＋socket 健康→deploy_ok），不再由部署席存活回报——部署席
  必死于自己发出的 restart，回报径结构性不存在。
- **upgrade 自检/回退径复活**：现 upgrade 进程死于自己发出的
  `systemctl restart`（同属 meta-gateway.service cgroup），exec 日志
  俱止「已装」、自检绿/回退行俱无——自检与安全回退现实不可达，须移到
  restart 存活执行体（transient unit/setsid/网关 boot 自检）才不是死码。
- **restart 连坐处置**：部署 restart 灭 cgroup 满门——本席＋全部他席
  （adjunct、他票部署/工作席）同死，须有 quiesce/豁免/复活径而非裸杀。

### 2. 为什么（实害清单，全部机器可复跑）

- **0034 部署 24 连败 0 胜**：`meta events --ticket 0034 | grep -c
  deploy_failed`＝24，`deploy_ok`/`retro` ＝0。每轮烧：spawn→integrate
  →check（gateway 套件 27.6s）→deploy→restart→席死→对账翻转→补发。
- **upgrade 死码实证**：run/exec/0034-deploy-1793611197413317464/
  -732469666821/-582924057427 三日志俱仅一行「已装」——upgrade.go:88
  「健康自检绿」与 :80「回退 prev」两分支从未落地输出；进程在
  restart() 调用后被 SIGTERM 杀灭（cgroup 实证见下）。
- **幂等重试收敛失效**：meta-deploy.sh 幂等闸设计＝下次部署见
  `cur==stamp` 跳过免 restart 收敛。实测 17:18–17:35 master code-head
  三连跳 bc1b821→73f2769→4fd621c：重试抵 deploy 步时 stamp 已漂→走
  全径→restart→死。收敛＝重试撞见 master 静窗，是运气非机制。
- **换装痕迹矛盾待查**：`meta.prev`=73f2769-deploy——末次 upgrade 换装
  的是 73f2769；然 journal 17:25–17:35 无 restart 记录，该二进制何径
  装入不明。且 18:01/18:05/18:12/18:13 四次 restart 无对应 0034 deploy
  exec 日志——他票部署/手升/异常换装径嫌疑，归施工查清。
- **全席连坐实证**：`systemctl status meta-gateway` 示 seat 进程
  （meta seat→devin -p…）皆 meta-gateway.service cgroup 子进程——
  restart＝满门杀。事件侧 adjunct_dead 簇：14408-09（前任巡检
  4186e985 claim 后秒死）、12845、12904-05、12974-75、13027；
  shells_degraded「CTO 候选用尽」反复；spawn_needed/spawn_blocked
  风暴（patrol_skip budget_frozen 满屏）。
- **回滚态清白**（非病灶）：三次 rollback 步 exit=0「本部署无破坏可回
  （merge已并历史不回退；二进制回退由upgrade自检兜底）」——注意该
  「兜底」即上述死码，实际无兜底。
- **当前在跑**：run 4d8d5b1fc864d8d9（integrate 中）；现装
  4fd621c-deploy＝现 code-head——若 master 静窗到其 deploy 步，本轮
  或经幂等跳过收敛；收敛不消本票，机制缺仍在（下票同雷）。

### 3. 范围内

- deploy 相位完成判定径：gateway 对账/reconcile 的 deploy 中席死分支
  （「先核回滚态」翻转）→ 改为 restart 后态判（installed stamp vs
  期望 ref 对应 code-head＋socket 健康）
- internal/upgrade 自检/回退执行体移位（谁活过 restart 谁自检；
  或自检推迟到网关 boot 自检步，红则回退 prev 再重启一次）
- deploy restart 前的席 quiesce/豁免径，或 restart 后对账复活规则；
  adjunct 巡检席免连坐（不占相位者被相位外因杀＝哑弹）
- config/gateway.textproto deploy_cmd 若需随改（transient unit 包装等）

### 4. 范围外（禁线）

- 不改票 0034 交付物本体（e109692 内容无责）；不手并 master
- 不动 p1/p2 已判修复件语义；不动 B1 部署/集成锁语义本体
- **禁以手改 phase 字段/手补 deploy_ok 事件/手跑 meta-deploy.sh 冒充
  修复**——机制径收口才可判
- 禁把「重试撞静窗收敛」当修复——锚 2 明确反锚

### 5. 设计引用

- scripts/meta-deploy.sh 幂等闸注释（自认「重启会杀部署席自身流
  （收养单次握手窗口竞态）」——本票证该竞态在并压下不收敛）
- internal/upgrade/upgrade.go:35-88（自检绿/回退两输出支现实不可达）
- 票 0029（幂等闸初心：非必要不重启；存量核实短路）、票 0034-p2
  （验树脱靶——同部署链他病勿并修）、票 0038-p1-p2（B1 锁孤儿死席
  不放锁——同族邻病）、票 0037-p2-p2（死席对账撞非法边静默楔死——
  同族邻病）
- 经验库《收养重挂后帧泵空转》（seat↔网关断代无声族）

### 6. 验收锚（全部机器可判）

1. **部署收口不再靠静窗**：部署期 master 有码并入（stamp 漂移）亦收口
   ——deploy_ok 由态判落，非部署席存活报。机器判：并一发码改后跑部署，
   events 见 deploy_ok 而非新一轮 deploy_failed
2. **自检径可达**：deploy exec 日志出现「健康自检绿」或「回退 prev」
   行——二者必有其一落地（现俱无＝锚在）
3. **坏二进制回退实证**：注入编译过但 boot 必败的二进制走 deploy →
   自动回退 prev＋事件留痕，非挂死（现径：自检死码→挂死无人知）
4. **连坐止**：部署 restart 前后 adjunct_claim 席不再 adjunct_dead
   连死；他票在跑席有豁免/复活径（事件可判）
5. 票 0034 出待部署相位（deploy_ok 落库）

### 7. 验证命令

- 基线：`meta events --socket $META_GATEWAY_SOCK --ticket 0034 |
  grep -cE "deploy_failed"`（修前 24）
- 收口判：`meta events --ticket 0034 | grep -E "deploy_ok|retro"`；
  `meta phase --ticket 0034`
- 自检可达判：部署后 `grep -l "健康自检绿\|回退 prev" run/exec/
  <票>-deploy-*.log`
- 对抗锚：构造 `init/panic` 或 socket 拒答二进制走 deploy →
  应见回退行＋prev 复装（`meta.prev` 版本回读一致）
- 连坐判：部署窗口内挂一 adjunct 巡检，events 无新增 adjunct_dead

### 8. 交付形制

照例：worktree 支 `t/0034-p3-*` 交付，本票文件＋判决档随支；全绿＋
§6 锚全过方提判。

## 附

- 证据档：run/exec/0034-deploy-{1793611197413317464,1793612732469666821,
  1793614582924057427}.log（俱止「已装」）；run/logs/baaabbd921ddef1e.log
  （integrate+check 双绿后 deploy 步失联）；bin/meta.prev=73f2769-deploy；
  journal 17:00–18:20 restart×7；事件 14369/14387/14411/14426
  deploy_start→failed 链；adjunct_dead 14408-09 等簇
- 观察项（不入本票范围）：ddebcb9a 部署席 spawn 即死「opencode 不在
  笼内 PATH」——壳画像缺陷，与本案并发不同因，留施工或另行立案

## 判决史

（空）
