# T21 设计档判决：批准（带四裁定两必补）（CEO 终端形态·决策 98，2026-11-02）

设计面采信：盘点实证（路径/权限全亲证口径）、五段序列逐段可回退、双单元
并存期 migrated 标记闸防双网关写库、壳 HOME 一次性损耗与 scope 退化如实
申报。

## 四裁定（决策点落锤）

1. **域＝user 域＋linger**——本迁移的目的是收权，转 system 域却要新增
   root NOPASSWD sudoers 行＝自拆台；user 域最小变更面、sudoers 零新增。
   （注：§2 表内"定夺：转 system 域"与 §6 推荐 user 域自相矛盾——以本
   裁定为准，施工时改表。）
2. **立户＝`-m -d /home/metaseat -s /usr/sbin/nologin`**——壳档可写
   HOME 必备；nologin 只禁交互登录不碍席位。
3. **shellbin＝`/opt/meta/shellbin/` 统一径，落副本**——**禁 symlink 穿
   mluser HOME**（穿家执行链＝隔离白做；三壳各装一遍的成本照付）。
4. **写域只迁 run/**（bin/config/roles/docs 只读保持）——按设计。

## 两必补（施工段硬条件，缺一不收口）

A. **壳鉴权档迁移**：设计只写了"会话档首跑重建"，未写鉴权（devin
   credentials.toml／claude 登录态／opencode key）——无鉴权三壳全哑。
   壳验证从「各验 --version」**升级为各壳一发真实模型调用**。
B. **docker 面缺口**：设计通篇未提 docker——部署闸 check 步在笼里跑，
   metaseat 无 docker 权即断。裁定：**metaseat 入 docker 组**，威胁模型
   口径入档写明（本降权目标＝与业主账隔离，非 root 容器化；不可信代码
   边界仍是笼）。施工段补组成员＋笼内 check 步实证。

**结论：设计批准。按四裁定两必补续作施工；回退方案照设计 §4。**
