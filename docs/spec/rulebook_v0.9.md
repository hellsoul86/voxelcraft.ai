# VoxelCraft 规则书（Runtime Current）

本文档描述当前后端已落地的业务规则与边界。

## 1. 世界形态

- 世界模型：2D tilemap（`chunk=16x16x1`）
- 写入约束：所有写世界动作必须 `y==0`
- 时间：`1 tick = 200ms`（5Hz）
- 昼夜：`day_ticks=6000`

## 2. 五世界分层（当前实现）

配置来源：`configs/worlds.yaml`

固定世界：
1. `OVERWORLD`
2. `MINE_L1`
3. `MINE_L2`
4. `MINE_L3`
5. `CITY_HUB`

每个世界可配置：
- 资源与边界：`seed_offset`, `boundary_r`
- 重置：`reset_every_ticks`, `reset_notice_ticks`, `allow_admin_reset`
- 规则开关：`allow_claims/mine/place/laws/trade/build`
- 入口：`entry_points[]`（位置 + 半径 + enabled）

世界切换：
- 动作：`SWITCH_WORLD`
- 强约束：必须满足 route + 入口点半径 + 冷却
- 失败返回：`E_WORLD_DENIED/E_WORLD_COOLDOWN/E_WORLD_BUSY`

## 3. 核心循环

探索 -> 采集 -> 生产 -> 建造 -> 交易 -> 组织 -> 治理 -> 事件 -> 赛季

## 4. 生存与任务

- 生存条：HP / Hunger / Stamina
- 任务槽：每个 agent 最多
  - 1 个 movement task
  - 1 个 work task
- 常用工作任务：`MINE/GATHER/PLACE/CRAFT/SMELT/BUILD_BLUEPRINT`
- 工具使用：隐式选择背包最优工具（无需 `EQUIP`）

## 5. 建造与蓝图

- 蓝图动作：`BUILD_BLUEPRINT`
- 支持 resume-safe：已正确放置的目标块会被跳过
- 材料不足时可自动拉取同领地附近 `CHEST/CONTRACT_TERMINAL`（范围见 tuning）
- 蓝图完成后会进入结构统计（用于 fun / influence）

## 6. 产权与治理

- Claim 支持类型：
  - `DEFAULT`
  - `HOMESTEAD`
  - `CITY_CORE`
- 维护费：按 `day_ticks` 结算，欠费进入降级
- 法律：参数化模板执行（税率、宵禁、罚款、核心区通行）
- 组织：成员与元数据跨世界收敛；资金按 world 分账（`TreasuryByWorld`）

## 7. 经济与合约

- 交易：P2P 报价/接受/拒绝
- 税：按地块法律执行 market tax
- 合约：`POST/ACCEPT/SUBMIT/CLAIM_OWED`
- 终端托管：reward/deposit 与欠账结算

## 8. 世界事件与 Fun

- Director 周期评估指标并调度事件
- 事件模板来自 `configs/events/*.json`
- Fun 维度：
  - Novelty
  - Creation
  - Social
  - Influence
  - Narrative
  - RiskRescue
- 反刷核心：递减收益 + 外部性验证 + 结构存活门槛

## 9. 反破坏与恢复

- 未授权写入在 claim 内直接拒绝
- 全量审计：block/entity 变更可追溯
- 快照 + 日志 + 回放
- rollback 目前主能力为 block 级回滚（非全状态回滚）

## 10. 赛季与重置

- 赛季长度可配置（默认 7 天）
- 矿层重置频繁，主世界/城镇重置谨慎（由 `allow_admin_reset` 守卫）
- 重置前会发 notice，重置后发 done 事件
