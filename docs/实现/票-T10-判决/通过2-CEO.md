# 票-T10 判决二骑（席位容器笼码面）——CEO

支 t/t10-seat-cage｜deliver_ref=73f2769｜基 0c8d462（含驳回1-CEO 返修）

## 判决：通过（并入主线）

## 返修四项逐锚复核
- ①c.Env 全串: PASS（`"-e", e` 传全 K=V；子 env 也过 ban 滤——adapter 无法回注 token；测试断言 `-e FOO=1`）
- ②mask 绝对径: PASS（`filepath.IsAbs` 拒相对径 continue；测试 `"run,/home"` 仅 `/home` 存活＋反向断言；种子改 `/opt/meta/run`）
- ③token/sock 剔: PASS（ban 三件 META_RUN_TOKEN/GATEWAY_SOCK/SEAT_SOCK 双环同滤——宿主 env 与子 env 两径都堵）
- ④user.Current 判空: PASS（nil→os.Getuid/Getgid 回退）

## 独立复算
build/vet 绿；**257 测全绿/16 包**（较基线+9——返修断言净增）。三壳活体
＋笼外径退役＝部署侧验收件（seat_image 非空生效需换装，届时实证）。

## 交付形制
哨兵未亲见但 ref 在支可核；返修 commit `[角色:施工][T10返修]` 合规。
