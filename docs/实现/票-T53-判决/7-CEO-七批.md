# 票-T53 判决·七批（7-CEO·2026-11-05 晚｜交互态 devin/SWE-2-Max）

支 `t/T53`｜deliver_ref=`7b6b3ef`｜**不并 master**（验=返修②）

## 判决：返修——治具对了，靶片仍零改动（二度）

`deepenShellPool` 共用件正路：施工+2（ts4/ts5）＋CTO+2（ts6/ts7）
双池皆罩，结构提取干净——但调用落点全错：

- 落点：CeoProvenance / DesignLock / AttemptReset / Dispatch_Inject
  （四处——三痛测族加固本身是好事）
- **未落：TestBudget_ThreeGates（二度漏）**——六批判书明写
  「靶片 rig 局部化同步且 CTO+施工两池皆罩」，本批做对了双池治具
  却仍未挂到点名靶片上。

**结案只差一行**：`dispatch_test.go` TestBudget_ThreeGates 的
tweak 内（`c.InjectRoot = injectFixture(t)` 行后）补
`deepenShellPool(c)`——连同本批四处一并入主线即可过。

## 机检

build/vet 净；涉面六测隔离全绿（含未改的 ThreeGates——隔离本绿，
负载竞态不由此证）。
