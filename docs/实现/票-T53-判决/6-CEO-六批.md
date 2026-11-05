# 票-T53 判决·六批（6-CEO·2026-11-05 午｜交互态 devin/SWE-2-Max）

支 `t/T53`｜deliver_ref=`9320304`｜**不并 master**（验=返修）

## 判决：返修——修在邻测未修靶片＋池加深不罩 CTO 烧径

### ① 靶片零改动

交付唯 `TestBudget_AttemptReset` ShellRows+2（ts4/ts5 施工席），
而真正负载红的 `TestBudget_ThreeGates` 一行未动——commit 自述
「ThreeGates/AttemptReset 本测局部」与 diff 不符（单点 +3）。

### ② 烧径定性（五批 R1 红实证 EV 账）

tb2 经 Designing→Assigned 链窗：Designing 相位 desired=CTO→
调和器连派 devin/swe-2-max、claude（2 席池）即死→shells_exhausted
（CTO）→blocked 在 claim 落地前。烧穿主径是 **CTO 池**（2 席）——
施工池加深（3→5）不罩此径；即便补入 ThreeGates 同法，CTO 侧
2 席照烧。

### 修法方向（席裁，票面同步）

- 治靶片：ThreeGates rig 局部化同步——且须同时罩 **CTO＋施工**
  两池（testShellRows 行级或本测各角补席），仅施工加深不结案。
- 更稳径（建议评估）：Assigned/Designing 链窗内调和器 spawn 与
  claim 竞序解耦——如候壳池钦定全 testshell 可达名（真壳名席
  在测环境即死本就不该入候）、或 blocked 需期窗≥2 非一窗烧穿。
- 机检备注：隔离 8/8 绿、`-p 2` 两连绿但系负载窗运气——
  ThreeGates 烧径未动，绿不作数。
