# 票-T10 判决（席位容器笼码面）——CEO

支 t/0033-cage-egress→t/t10-seat-cage｜deliver_ref=bc1b821｜基 0c8d462

## 判决：驳回（两缺必返，返后复审）

## 逐锚对账
- 机制骨架: PASS（外壳持 token/sock 留宿主、壳子进程笼化、secrets ro、
  workdir 同径挂、env 白名单剥宿主全量——req-f88 §4 语义正确）
- env 透传链: PASS（dispatch trySpawn 落 META_SEAT_* 四件；空=旧径活）
- 单测面: PASS（cageArgv 成型+白名单外剥光，147 测绿）

## 驳回理由（逐锚——返修点）
1. **c.Env 值全丢**（internal/seat/seat.go cageArgv 尾环）：`"-e",
   strings.SplitN(e,"=",2)[0]` 只传键——docker 从席进程 env 取值非子
   env 值，adapter 注入件在笼内错值/缺失。**修**：传 `"-e", e` 全串；
   t_t10 断言改 `"-e FOO=1"` 并反向断言宿主同键不同值不污染。
2. **mount_mask 种子相对径**（config/gateway.textproto）：`seat_mount_mask:
   "run"`→`--tmpfs run` docker 要绝对径→笼启全席 spawn 崩。**修**：改
   `/opt/meta/run`；测试补一例相对径拒/转换。
3. **META_RUN_TOKEN 入笼**（卫生·建议级）：白名单全放 META_*→token 落
   不可信壳 env；socket mask 兜住无实害但不该给。**修**：白名单剔
   META_RUN_TOKEN/META_GATEWAY_SOCK/META_SEAT_SOCK。
4. user.Current() 未判空（低，同 T1 笼内坑型——seat 宿主面概率低）。

## 不扣分项
- 三壳活体锚本票天然到不了（部署后才证）——CEO 侧验收件，换装时实证。
- 收口形制偏（用 o 原文 [施工收口:T-0274] 连票号都错；钦定 META_DELIVER:
  <commit>）——记教训不驳。
