# World Director & Fun（Runtime Current）

## 1. Director 目标

Director 用于维持世界节奏，避免长期单一最优策略。

输入信号（滚动窗口）：
- Trade 活跃度
- 冲突/拒绝率
- 探索度
- 资源不均衡
- 公共基础设施占比

输出：
- 事件选择
- 事件实例化位置
- 事件窗口与公告

## 2. 事件调度策略

当前策略是“脚本节奏 + 反馈权重”混合：
- 赛季前 7 天优先脚本化节奏
- 其后按 `director_every_ticks` 周期评估并采样

权重修正方向：
- 交易低 -> 提高市场/协作类事件概率
- 探索低 -> 提高远征/遗迹类事件概率
- 冲突高 -> 提高治理/修复类事件概率
- 贫富差距高 -> 提高公共工程/治理窗口概率

## 3. 事件实例化

实例化流程：
1. 选中心点
2. 应用 spawn plan（资源簇、营地、公告板等）
3. 向 agent 投递 `WORLD_EVENT`
4. 事件窗口内收集行为信号

常见事件类别（由配置模板驱动）：
- 资源：`CRYSTAL_RIFT`, `DEEP_VEIN`
- 探索：`RUINS_GATE`
- 公共工程：`FLOOD_WARNING`, `BLIGHT_ZONE`
- 社会：`MARKET_WEEK`, `BLUEPRINT_FAIR`, `BUILDER_EXPO`
- 治安/治理：`BANDIT_CAMP`, `CIVIC_VOTE`

## 4. Fun Score 维度

当前保留 6 维：
- `NOVELTY`
- `CREATION`
- `SOCIAL`
- `INFLUENCE`
- `NARRATIVE`
- `RISK_RESCUE`

输出位置：
- `OBS.fun_score`（累计）
- `OBS.events` 中 `type="FUN"`（增量）

## 5. 反刷规则

- 同类行为有递减收益（窗口衰减）
- 结构相关得分要求存活与被使用
- 风险分依赖正向结果（救援/交付/事件目标）
- 社交收益受信誉影响（低信誉打折）

## 6. 结构与影响力

系统会跟踪 blueprint 结构：
- 延迟发放 creation 分（存活窗口后）
- 按 unique users/day 结算 influence 分
- 结构失效（被拆/不匹配）会剔除统计

## 7. 赛季联动

- 赛季切换会重置资源分布与部分运行态
- 保留组织身份等“文化资产”
- 事件系统在新赛季重启节奏
