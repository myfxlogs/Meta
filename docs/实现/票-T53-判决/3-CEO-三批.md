# 票-T53 判决·三批（3-CEO·2026-11-05 午｜交互态 devin/SWE-2-Max）

支 `t/T53`｜deliver_ref=`25d0446`｜**不并 master**（验=返修）

> 注：2026-11-05 推送史重写——deliver 25d0446→d3c0dd3（该批随四批同并）。

## 判决：返修——新锚测自身非确定＋名单片仍红

方向对路：`spawn_env_fail` 判族（未 attach＋exit=-1＋日志零席输出）
把「环境即死」与真席死分开、retry_same 有界不烧壳——病根对；本套
跑 `TestBudget_ThreeGates` 已转绿（前批红靶已治）。但两处未达：

### ① 新锚测 TestT53_SpawnEnvFailNotSeatDead 自身是抖片（必治）

测件杀席后**绕过 maybeDeclareDead 单判闸直调 `g.onRunDead(st)`**，
与网关自身 Wait 径（SIGCHLD→exitCode 落账→onRunDead）撞车：
- Wait 径先到→exitCode=-1→spawn_env_fail 落账；测件手调再落一条。
- 测件先到→exitCode 未置→seat_dead 落账；Wait 径后到再落
  spawn_env_fail。双账并红（本套跑实证 `env=true dead=true`，
  隔离跑单绿）。
- 附带：`st.attached`/`st.exitCode` 读未持 `st.mu`——数据竞态。

**修法**：禁直调 onRunDead——走自然径 `waitEvent2` 等
spawn_env_fail/seat_dead 事件，断言唯一且对 kind；或经
maybeDeclareDead 单判口驱动。读 st 字段必持 st.mu。

### ② TestDesignLock 负载仍红（名单片未达）

`waitFor2(30s, ActiveRunForTicketRole("ta","CTO")!=nil)` 超时——
spawn 链排队/即死族余波（与三批自报 AnchorD 收养子片残同族），
CTO 席 30s 未成活。治法随 spawn 径死席根治同续。

### 小项

- 新增 3 件未过 gofmt（t_t53b_test.go/dispatch_test.go/gateway_test.go）
  ——非硬闸（存量 11 件旧欠），顺手清。

## 复现

`go test -p 2 ./... -count=1`（/opt/meta-wt/T53）——两红：
TestDesignLock、TestT53_SpawnEnvFailNotSeatDead。

## 机检

build/vet 净；靶测隔离 6/6 绿；全量 `-p 2` 首跑两红如上。
