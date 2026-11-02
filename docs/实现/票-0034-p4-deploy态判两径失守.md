# 票 0034-p4：deploy 收口态判两径失守——席死径绕开 deployStateOK＋cmd_run 证据 restart 结构性缺席＋死席哨兵尾帧无排干（0034 物理全绿翻负第 26 发）

> 巡检席自立案（adjunct，不占相位不重派）。触发信号＝0034 `phase:deploy_failed`
> （事件 14701：部署中席死——先核回滚态，前任 run=e2ea3eccfcc48262）。判**真**：
> 部署四步物理全绿（integrate/check×2/probe×2 皆 exit=0，deploy 实成换装
> 83ef07a-deploy＋网关 18:47:25 重启后 socket 健），`META_DEPLOY:done` 已落
> sentinel.log，仍被翻负——p3 态判并主线后首个「全绿仍败」实证，证态判双径
> 失守。本轮巡检 run=788ae53120a08620。

### 1. 目标

修「p3 部署态判在生产径不可达」三处机制缺：

- **席死径挂态判**：attach 的 SIGCHLD+EOF 席死分支对部署中票直翻
  deploy_failed，全程不查 deployStateOK——态判仅挂 reconcileActor
  对账径单点，主死亡径裸奔。
- **态判证据源换代**：deployStateOK 唯一「已装」证据＝末次 deploy
  cmd_run exit=0，而 deploy 全径 restart 杀记录者（rec() 步完才写
  库）使该行结构性永不落库——0034 全史 37 行 cmd_run 零 deploy 步
  实证。换跨 restart 存活证据（installed stamp vs 期望 code-head、
  deploy exec 日志「已装」行），socket 探活保留。
- **死席哨兵尾帧排干**：sentinel.log 兜底泵是席侧件随席同死，网关
  对死席 artifacts 无末次扫描——done 在盘被蒸发。死判前先排干在
  盘哨兵/在途 Deliver 再裁。

### 2. 为什么（实害清单，全部机器可复跑）

- 事件 14701 note「部署中席死：先核回滚态」逐字出 attach.go:562——
  非对账径（对账径 note＝「对账：行动者无响应」，reconcile.go:228）。
- e2ea3ecc cmd_run 六行全绿：integrate 92→0、check 0×2、probe 0×2；
  `select step,count(*)` 对 0034＝check 16/integrate 14/probe 2/
  rollback 5——deploy 步全史 0 行。
- deploy 实成四源互证：run/exec/0034-deploy-1793616442326190093.log
  「已装 bin/meta.new → /opt/meta/bin/meta」；`bin/meta version`
  ＝83ef07a-deploy；码面 code-head 实算 83ef07a 逐字相等；网关
  restart 后 socket 对 meta CLI 即答。
- run/artifacts/e2ea3eccfcc48262/sentinel.log 五行齐含
  `META_DEPLOY:done`——票事件流无 deploy_done/deploy_ok 帧。
- run/logs/e2ea3eccfcc48262.log 实录四步绿＋done 双发＋stdout 块
  缓冲未泄——席非懒死，帧在盘在缓冲而死亡判定先到。

### 3. 范围内

- `internal/gateway/attach.go` 部署中席死分支：transition 前两级
  判——先排干死席 `$META_ART_DIR/sentinel.log` 未消费行
  （META_DEPLOY:done→deploy_done 边消费；failed→deploy_failed）；
  无终哨兵再查 deployStateOK——绿→deploy_ok＋放 integrate/deploy
  双锁，非绿→照旧 deploy_failed（D-2 语义不退）。
- `internal/gateway/reconcile.go` deployStateOK：「已装」证据换源
  ——installed stamp（`bin/meta version`）对期望 code-head（deploy
  exec 日志「已装」行或 deliver ref 换算），cmd_run 行留作旁证非
  必需；socket Probe 保留。
- 证据源取网关侧可达面（boot 后可查），落点/函数形由施工定，锚管
  行为不管名。
- 测试面：attach 径席死×态判真/假两支、sentinel 尾哨兵消费、全径
  restart 后态判可真（stamp/Probe 可桩）。
- config/gateway.textproto 如需探针或日志径参数随改。

### 4. 范围外（禁线）

- 不改票 0034 交付物本体；不手并 master；不手改 phase 字段/手补
  deploy_ok 事件/手跑 meta-deploy.sh 冒充收口。
- 不动 p1/p2/p3 已判修复件语义、B1 部署/集成锁语义、D-2「部署中
  席死＝部署失败」对真败径的判定——无已装证据仍须翻负。
- 不修 stdout 块缓冲/泵滞本体（族症另案——本票只管「泵滞致席死
  后判法」，缓冲修不修态判都须能收口）。
- 不动 handleExec 三重闸、哨兵契约文本、meta-deploy.sh 幂等闸语义。

### 5. 设计引用

- internal/gateway/attach.go:558-586（席死分支无态判直翻）；
  reconcile.go:111-128（deployStateOK 证据源＝cmd_run）、212
  （态判单挂点）；exec.go:197-207（rec() 步完才落 cmd_run——
  restart 杀之则 cmd_run/exec 事件/ExecResp 三痕俱无）。
- internal/seat/seat.go:320,621-（pumpSentinelFile 席侧件，随席
  同死则无末泵）。
- docs/实现/设计-0034-p3-部署态判收口与席位scope豁免.md（原设计
  「装着 stamp＝期望＋socket 健康→deploy_ok」——实现落成
  cmd_run 证据换源未达设计）；票-0034-p3 判决档（通过2/通过3，
  d4a1f7e/728adeb 并主线）。
- 经验库 2026-11-02-devin-stdout块缓冲-哨兵全季末落（泵滞族根因；
  本票修判法不修缓冲）。
- scripts/meta-deploy.sh（幂等闸＋「已装」行＝stamp 事实源先例）。

### 6. 验收锚（全部机器可判）

1. **席死径态判生效**：部署中席死＋installed stamp＝期望 code-head
   ＋socket 健 → events 见 deploy_ok（完成），非 deploy_failed。
2. **全径后态判可真**：deploy 真换装＋restart（cmd_run 无行）场景
   态判返 true——cmd_run 缺席不再必然翻负。
3. **尾哨兵消费**：sentinel.log 含 META_DEPLOY:done 的席死 →
   deploy_done 边落库；含 failed → deploy_failed。蒸发径断。
4. **反锚（删/翻必红）**：席死＋stamp 不符/socket 不健康/无已装
   行 → 仍 deploy_failed——弱化成「席死恒 deploy_ok」即红。
5. **签名绝迹**：本票落地后 0034 事件流再出「物理全绿翻负」签名
   ＝锚红。

### 7. 验证命令

```bash
sqlite3 run/meta.db "select step,exit_code from cmd_run where ticket_id='0034' and step='deploy'"   # 修前恒空＝证据缺实证
go build ./... && go vet ./... && gofmt -l internal/
go test -count=1 ./internal/gateway/... -run 'Deploy|Attach|Death|StateOK'
meta events --socket "$META_GATEWAY_SOCK" --ticket 0034    # 对 14701 类 note 复跑判：同签名不得再出 deploy_failed
```

手验：停一缓冲中部署席进程组（模拟冤杀）＋装好 stamp →
attach/对账径判 deploy_ok。

### 8. 交付形制

照例：worktree 支 `t/0034-p4-*` 交付（不手并 master），本票文件＋
判决档随支；全绿＋§6 锚全过方提判。

## 附

- 证据档：run/logs/e2ea3eccfcc48262.log（四步绿＋done 双发＋缓冲未
  泄全录）；run/artifacts/e2ea3eccfcc48262/sentinel.log（五行含
  done）；run/exec/0034-deploy-1793616442326190093.log（「已装」行）；
  run/exec/0034-probe-1793616626{694521235,780845061}.log（pong 健）；
  事件 14535/14701（双死亡径 note 对照）；0034 cmd_run 37 行零
  deploy 步。
- 并发同族（不入本票）：stdout 块缓冲泵滞致对账冤杀（经验库已记，
  修不修本票判法均须兜底）；0043 收缩残面（9375b27 已判决）。
- 优先级 P1（部署链自愈债）；预算/壳序缺省。

## 判决史

（空）
