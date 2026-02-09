# World Director & Fun Score v0.9

目标：让世界长期“有故事”，同时避免 agent 只会刷一个指标。

## 1. Fun Score（多维回馈信号）

推荐 6 维：
- Novelty（新奇）
- Creation（创造）
- Social（社交）
- Influence（影响力）
- Narrative（叙事参与）
- Risk & Rescue（冒险与救援）

MVP 协议落点：
- `OBS.fun_score`：累计分面板（整数）
- `events[].type == "FUN"`：每次加分的明细（dim/delta/total/reason）

通用规则：
- 同类行为短时间重复，收益衰减（diminishing returns）
- 高分尽量来自外部性：他人使用/参与/认可，而非自循环

反刷要点（Checklist）：
- 必须有外部性：建筑分看使用量，不看摆块数
- 重复衰减：滑动窗口内指数衰减
- 结构需要存活：建筑分需在 N ticks 内未被拆毁
- 风险必须有成果：只冒险不给分，救援/交付才给分
- 信誉联动：低信誉社交分打折

MVP 简化说明：
- 事件类 Novelty/Narrative 更偏向“在事件窗口内完成正向结果”触发（而不是仅收到事件广播就给分）

## 2. 世界事件生成器（Director）

核心思路：用世界状态做反馈控制，补短板。

指标（示例）：
- exploration
- trade volume
- conflict
- inequality
- guild dominance
- public infra ratio

调度：
- 每天（现实时间）至少 1 个轻量事件
- 每 2–6 小时 1 个中型事件
- 每 1–2 天 1 个大型事件

事件模板库（MVP 12 个）：
见 `configs/events/*.json`

伪代码（实现参考）：
```python
def director_tick(world_state):
    metrics = compute_metrics(world_state)
    weights = base_event_weights()
    if metrics.trade < 0.4:
        weights["MARKET_WEEK"] += 0.25
        weights["BLUEPRINT_FAIR"] += 0.15
    if metrics.exploration < 0.3:
        weights["CRYSTAL_RIFT"] += 0.20
        weights["RUINS_GATE"] += 0.20
    if metrics.conflict < 0.1:
        weights["DEEP_VEIN"] += 0.15
        weights["BANDIT_CAMP"] += 0.10
    elif metrics.conflict > 0.25:
        weights["CIVIC_VOTE"] += 0.25
        weights["MARKET_WEEK"] += 0.10
        weights["BUILDER_EXPO"] += 0.10
    if metrics.inequality > 0.5:
        weights["CIVIC_VOTE"] += 0.20
        weights["FLOOD_WARNING"] += 0.10
    return sample_event(weights)
```
