# 设计-0034-p3：部署态判收口与席位 scope 豁免（restart 杀席≠部署败——三机制一链）

> 需求票 `0034-p3`（巡检自立案）→ CTO-A 设计 → 子票 0043 施工。
> 钦定轨（§1.4）：设计档 commit → 本票八栏回填 → 子票立＋派 → Deliver。
> 结论先行：票面三缺全部属实；起手支 `t/0034-p3-deploy-verdict`（/tmp/wt-p3
> 未提交稿）方向同构但态判输入结构性不可达（§0.3）——本设计以
> **pending 意向书＋自报 stamp 比对**收其神、纠其形。

## 0. 实证链复核（票面三缺俱实）

### 0.1 病灶在码位（已读码坐实）

- **部署席必死径**：席位＝网关 `exec.Command` 子进程（spawn.go:58），与网关
  同在 `meta-gateway.service` cgroup；`systemctl --user restart`＝
  cgroup 满门杀——部署席、他票席、adjunct、exec 代跑步同灭。
- **自检死码**：`upgrade.Do`（internal/upgrade/upgrade.go:72-89）在
  `restart()` 之后的 Probe/回退段进程已死——exec 日志俱止「已装」
  （票面证据 run/exec/0034-deploy-*.log 三份）。
- **收口靠席活**：`deploy_done` 唯一径＝部署席 Deliver 携
  `META_DEPLOY:done`（attach.go:340-367）；席死 → 对账
  `PhaseDeploying→待部署 deploy_failed`（reconcile.go:186-198）——
  成败判定寄生于「必死之席的存活」，结构性恒败。24 连败实证链成立。

### 0.2 §5.1 钦定未遂

「席位在独立 systemd scope 不随死」「收养」均已是总设计钦定语义
（tryAdopt/per-seat UDS/run_token 互证俱在码）——唯席位从未真的进
scope，收养径因此从未跑通过。本票是补「钦定已言而未行」不是新发明。

### 0.3 起手支体检（/tmp/wt-p3，t/0034-p3-deploy-verdict，未提交）

前人草案三洞，**照抄即死码**：

1. `deployStateOK` 读 `cmd_run` deploy 步 exit=0——但 restart 杀网关时
   `runExecStep` 的 `cmd.Wait()` 永不返回，**InsertCmdRun 行根本写不出**；
   判据输入在其唯一适用场景结构性不存在。
2. 同函数 `Probe(sock,2s)` 在启动对账内跑——`New()` 里 reconcile 先于
   `g.Serve()`：listener 已绑、grpc 未接客，拨通必超时——boot 时刻
   该判据恒红。
3. `spawnChecker` 固定 unit 名 `meta-upg-check`＋证据行只往 detached
   进程 stdout（journal/黑洞）——审计锚「exec 日志见自检行」落空。

可承部件：scope 包装方向、detached checker 骨架、`upgrade-check`
子命令名。均按下文纠形入子票。

## 1. 总机制（一图）

```
deploy 步（meta-deploy.sh，网关子进程，restart 时必死）
   │ build → meta.new → NEW_BIN upgrade
   │   预检 → prev 快照 → spawn checker（systemd-run scope，逃逸 cgroup）
   │   → install 原子换 → 写 pending{ticket,expect,install,prev,exec_log}
   │   → systemctl restart  ☠ 本步与本席/网关同灭；checker 独活
   ▼
checker（meta upgrade-check · transient scope）          新网关 boot
   │ 有界等 socket → Probe 绿                             │ New(): reconcile
   │ → install version==expect?                          │  读 pending（消费即删）
   │   绿→「健康自检绿」→exec-log＋last-verdict ok          │  部署中票+pending 匹配:
   │   红→prev 回写→restart→再 Probe→「回退 prev」行        │    SelfStamp==expect → deploy_done
   │ → meta reconcile poke（兜底触发）                     │    ≠ → deploy_failed（note 带戳）
   ▼                                                      │  无 pending → 老径（收养/判败）
席位（meta-seat-<runid>.scope，restart 不杀）────收养→续跑
```

**stamp 语义不变**（meta-deploy.sh 既有口径）：code-head＝
`git log -1 -- <码面路径spec>`；`buildStamp` 经 -ldflags 注入＝
`<code-head>-deploy`。pending.expect＝预检时 `NEW_BIN version` 第二段；
网关侧 SelfStamp＝main.buildStamp 传入 Config——**同源戳，不抄公式**。

## 2. 机制一：pending 意向书（部署意图书＝单次消费文件）

- 落点：`<rundir>/upgrade/pending`（部署锁串行→单文件成立；写后 restart
  前落盘，消费＝读+删）。
- 载荷（`k: v` 行，非 JSON——R3）：`ticket / expect / install / prev /
  exec_log / ts`。
- 写者＝`upgrade.Do`（NEW_BIN 跑），读者＝新网关 reconcile。**旧二进制
  不写不读**——版本回落自然降级回老径，无迁移负担。
- 语义：「上次有人装了二进制并发过 restart」的不可变意向——比任何
  事后重算都真（master 漂移、静窗运气全免疫——票面锚1 落点）。

## 3. 机制二：checker 存活执行体（自检/回退复活）

`meta upgrade-check`（子命令名沿起手支）＋`upgrade.Check`：

```
argv: --sock --install --prev --expect --exec-log --verdict --restart-cmd --have-prev
1. Probe(sock, 上限90s 有界重试——R2 边界合法)
2. 绿 → `<install> version` 读装着戳
   - ==expect → exec-log 追加「upgrade-check: 健康自检绿 <sock>」＋
     <rundir>/upgrade/last-verdict 写 ok 行 → meta reconcile poke → 退 0
   - ≠/Probe红 → have-prev 则 prev 原子回写 install → restart-cmd 一次
     → 再 Probe → 「upgrade-check: 健康自检失败——回退 prev」/
     「回退后仍红——交外层报警」→ verdict rolledback|failed → poke → 退码记果
```

- spawn 径：`upgrade.Do` 内 `systemd-run --user --scope --collect --quiet
  --unit=meta-upg-check-<unix>`（**unit 名带时间戳**，修草案撞名）；
  无 systemd-run → setsid 兜底；两径俱败→**还 prev 回报错**（此时
  install 未动，安全失败）。
- Do 新序：预检(提取 expect)→prev 快照→**spawn checker**→install→
  写 pending→restart。restart 后旧 Probe 段仅 RestartCmd=="" 径保留
  （本就不死）；RestartCmd 非空时该行改打「自检已移交 checker」。
- 证据落点：票面锚2 的「exec 日志见自检行」由 checker 追加写
  `--exec-log`（＝网关 `META_EXEC_LOG` 注入的当步 out 文件）——
  死进程的日志文件仍是合法追加目标（fd 无关，按路径追加）。

## 4. 机制三：reconcile 态判（deploy_done 不靠席活）

`reconcileActor` 的 PhaseDeploying 分支前置态判（reconcile.go）：

```
pend := consumePending()        // 文件在才读+删；全 reconcile 一次
for 部署中 t:
  pend!=nil && pend.Ticket==t.ID:
     cfg.SelfStamp==pend.Expect → transition(完成,EdgeDeployDone)
        note="态判收口：装着=期望 stamp=<s>（socket 实证见 checker/last-verdict）"
        → 残席标死+killRunGroup（相位终态，席无续跑义）
     ≠ → transition(待部署,EdgeDeployFailed)
        note="态判：装着 <got> ≠ 期望 <exp>（回退态见 run/upgrade/last-verdict）"
        → 残席同处置
  pend 为 nil / ticket 不符 → 老径（收养 / 席死 deploy_failed）
  pend 匹配不到任何部署中票 → deploy_pending_stale 事件+弃
```

- **为何敢在 Serve 前判**：`New()` 内 `net.Listen` 已成功才走到
  reconcile；态判本质是「上次 restart 装没装对」——stamp 比对是唯一
  权威，socket 健康是部署效果的独立证据位（checker ping 落档），
  不挡判。残险明写：Serve 起手死→checker 回退 prev→prev 旧码
  无 pending 感→票留「完成」与回退态错配——概率≈0，错配归 CEO 翻，
  不为此加扫描窗。
- 无 pending＝无部署级 restart 发生过 → 老径语义原样（收养先行）。
- 部署席若在 scope 幸存被收养：态判先行——判定是客观态不靠席续跑；
  判后残席一律标死杀组，防「完成票上挂着还在跑步的僵尸席」。

## 5. 机制四：席位 scope 豁免（连坐止＋adjunct 收养）

- `Gateway.New()` 一次性探测：`systemd-run --user --scope --quiet
  --collect --unit=meta-seatprobe-<rand> -- true`——成败落
  `g.seatScope`＋一次事件 `seat_scope=on|off`（探测缝注入测试）。
  **不逐 spawn LookPath**——scope 坏时每次 spawn 都「起后秒死」，
  会把全壳误标 dead（shells_exhausted 风暴，0007 类冤案）。
- `startRun`：seatScope=on → argv 前缀
  `systemd-run --user --scope --quiet --collect --unit=meta-seat-<runid> --`；
  off → 原径直跑（降级非死）。
- kill 径：先 `systemctl --user kill --kill-whom=all
  meta-seat-<id>.scope`（错吞），再 `kill(-pid)` 兜底（跨 cgroup pgid
  仍在，双保险）。
- adjunct 收养（D-55 环扩）：ActiveRuns 逐条——adjunct＋pid 活＋票非终态
  → `tryAdopt`→MarkAdopted＋runState＋adopted 事件（票终态→杀，同现
  D-55 收尸语义）。不占相位者从此免连坐且成果不丢。
- **exec 代跑步不 scope 化**（明写）：步是票相位内的一次性件，死由
  态判/重跑收口；scope 化反而让超时杀在网关死时失效（杀器在旧进程）。
- systemd-run --scope 契约须实证入测试：前台等待、退码传播、
  父子关系保 SIGCHLD（若实证不符→回退直跑＋事件，不硬上）。

## 6. 配套小面

- `runExecStep` envs 增：`META_RUN_DIR=<g.cfg.RunDir>`、
  `META_EXEC_LOG=<outPath>`（pending/checker 落点参数化——D-24 缝同构）。
- `meta-deploy.sh`：upgrade 行加
  `--pending "${META_RUN_DIR:-/opt/meta/run}/upgrade/pending"
  --ticket "$META_TICKET_ID" --exec-log "${META_EXEC_LOG:-}"`。
- `Config.SelfStamp`（serve() 传 `buildStamp`；测试注入）。
- `docs/技能库/二进制换装三件套.md` 证错同票修：第 3 条「回退=熔断
  触发」补「升级时刻自检执行体=checker transient scope（0034-p3：
  自检死于自发 restart 的修正）」。

## 7. 时序三径

- **绿径**：装→pending→restart→新 boot reconcile→pending 匹配＋
  SelfStamp==expect→deploy_done→票完成；checker ping 绿→exec-log
  「健康自检绿」。席存活与否无关。
- **坏二进制径**：装→pending→restart→boot 败（Serve/侦听前崩）→
  checker Probe 红→prev 回写→再 restart→prev 起→票→deploy_failed
  （prev 老码走老径/或新码 pending≠got 同果）→事件留痕＋exec-log
  「回退 prev」行。挂死无人知从此消失。
- **幂等径**：cur==stamp→meta-deploy.sh 短路退 0（不装不写 pending
  不 restart）→席活→probe→META_DEPLOY:done。原样不动。

## 8. 范围外（子票不碰）

- 票 0034 交付物本体语义、B1 锁语义、部署四步序与配方步名、
  proto/RPC 面（pending/verdict 走文件＋既有 reconcile RPC——零协议
  变更）、meta-gateway.service unit 本体、supervise、patrol/caster。
- 邻病不并修：0038-p1-p2（B1 孤儿锁）、0037-p2-p2（死席对账撞非法
  边）、ddebcb9a（opencode PATH 壳画像）——同族不同票。
- exec 步 scope 化（§5 明写理由）、心跳/周期件（红线）。

## 9. 记档

- **票 0034 出部署相位**（票面锚5）＝机制生效的自然果：0043 部署后
  0034 下轮部署走绿径/幂等径收口——非本票测试锚，是部署域观察位。
- **坏码票部署循环经济学**：rollback 后 deploy_failed→待部署→
  batchDeployCheck 补发会再部署同坏码——烧票级硬顶止血（既有），
  「deploy_poison 阻重试」可作后续改进票，本票不扩。
- **0034 部署 24 连败的收敛真相**：幂等跳过只在「master 静窗」下
  收口＝运气非机制；态判后无论静窗与否 restart 径恒收口。
- **起手支处置**：/tmp/wt-p3(t/0034-p3-deploy-verdict) 留作 0043 施工
  接替参考——三洞见 §0.3，可取骨架不可取判据。

## 10. 子票锚预览（详见票-0043 八栏）

态判三态用例 / pending 单次消费＋stale 留痕 / scope argv 包装断言 /
checker 绿红双径（exec-log 行＋prev 回写＋restart 调用痕）/ adjunct
收养 / exec envs / bash -n 脚本闸 / `go build+vet+test ./...` 全绿；
对抗锚＝态判判等行、pending 删除行、scope 包装行各为一名刽子手提名位。
