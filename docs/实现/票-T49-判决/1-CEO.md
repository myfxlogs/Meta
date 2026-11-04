# 票-T49 判决（1-CEO·2026-11-04 晚｜交互态 devin/SWE-2-Max）

支 `t/T49`｜deliver_ref=`d8fe6b2`｜并 master=`fbc8ab0`
（attach.go/reconcile.go＋新 t_t49_test.go，+70/−9）

## 判决：通过

- **锚·同窗恰一条**：`epochHasKind` 共用判据落 reconcile.go——attach
  席死径两支（哨兵 failed／席死无据）与 reconcile 径同窗双记全
  归一去重闸（attach.go:591/619＋reconcile.go:427）——0051 实证
  的同秒双账凑 cap 回 blocked 活锁根断。
- **锚测**：新 `TestT49_AttachReconcileSingleDeployFailed` 双径同窗
  断言恰 1 条——严断言成立。
- **机检**：build/vet 净；`TestT49|TestT48` 6 测全绿；gateway 开
  发面全量 **197 测零 FAIL**；换装 `d8fe6b2-deploy` 生效。

## 遗留

- 票面「关联」项未做：`TestT48_DeployFailedDedupe` 首轮锚仍「≥1」
  容差未收回「恰1」——**严断言已由 T49 新锚测同景覆盖**，宽口无
  损但记一笔（下次测试卫生顺手收回）。
- D-93 片本体慢偿、D-94、D-98 在册不变。
