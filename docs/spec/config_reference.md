# Config Reference（Runtime Current）

## 1. Global Runtime（`configs/tuning.yaml`）

- `tick_rate_hz`: 默认 5
- `day_ticks`: 默认 6000
- `snapshot_every_ticks`: 默认 3000
- `director_every_ticks`: 默认 3000
- `season_length_ticks`: 默认 42000
- `obs_radius`: 默认 7
- `boundary_r`: 默认 4000

2D 固定语义：
- `chunk_size=[16,16,1]`
- `height=1`
- 写世界动作必须 `y==0`

## 2. Worldgen（`worldgen`）

- `biome_region_size`
- `spawn_clear_radius`
- `ore_cluster_prob_scale_permille`
- `terrain_cluster_prob_scale_permille`
- `sprinkle_stone_permille`
- `sprinkle_dirt_permille`
- `sprinkle_log_permille`

## 3. Starter Items

`starter_items` 默认：
- `PLANK: 20`
- `COAL: 10`
- `STONE: 20`
- `BERRIES: 10`

## 4. Rate Limits

`rate_limits` 默认：
- `say_window_ticks=50`, `say_max=5`
- `market_say_window_ticks=50`, `market_say_max=2`
- `whisper_window_ticks=50`, `whisper_max=5`
- `offer_trade_window_ticks=50`, `offer_trade_max=3`
- `post_board_window_ticks=600`, `post_board_max=1`

## 5. Governance / Build / Fun

- `law_notice_ticks=3000`
- `law_vote_ticks=3000`
- `blueprint_auto_pull_range=32`
- `blueprint_blocks_per_tick=2`
- `access_pass_core_radius=16`
- `claim_maintenance_cost`（默认 `IRON_INGOT + COAL`）
- `fun_decay_window_ticks=3000`
- `fun_decay_base=0.70`
- `structure_survival_ticks=3000`

## 6. Multi-World（`configs/worlds.yaml`）

顶层：
- `default_world_id`
- `worlds[]`
- `switch_routes[]`

`worlds[]` 字段：
- 基础：`id`, `type`, `seed_offset`, `boundary_r`
- reset：`reset_every_ticks`, `reset_notice_ticks`, `allow_admin_reset`
- 切换：`switch_cooldown_ticks`, `entry_point_id`, `entry_points[]`
- 规则开关：`allow_claims`, `allow_mine`, `allow_place`, `allow_laws`, `allow_trade`, `allow_build`

`entry_points[]`：
- `id`, `x`, `z`, `radius`, `enabled`

`switch_routes[]`：
- `from_world`, `to_world`
- `from_entry_id`, `to_entry_id`
- `requires_permit`

## 7. Content Catalogs

- `configs/blocks.json`
- `configs/items.json`
- `configs/recipes.json`
- `configs/blueprints/*.json`
- `configs/law_templates.json`
- `configs/events/*.json`

## 8. Persistence Paths

- Events: `data/worlds/<world>/events/*.jsonl.zst`
- Audit: `data/worlds/<world>/audit/*.jsonl.zst`
- Snapshot: `data/worlds/<world>/snapshots/*.snap.zst`
- Archive: `data/worlds/<world>/archives/season_<n>/`
- Index DB (local/dev): `data/worlds/<world>/index/world.sqlite`
- Index DB (Cloudflare runtime): D1 tables via `/_cf/indexdb/ingest`
- Global state: `data/global/state.json`

## 9. Metrics（`/metrics`）

单世界指标：
- `voxelcraft_world_tick{world}`
- `voxelcraft_world_agents{world}`
- `voxelcraft_world_clients{world}`
- `voxelcraft_world_loaded_chunks{world}`
- `voxelcraft_world_queue_depth{world,queue}`
- `voxelcraft_world_step_ms{world}`
- `voxelcraft_director_metric{world,metric}`
- `voxelcraft_stats_window{world,metric}`
- `voxelcraft_stats_window_ticks{world}`

多世界指标：
- `voxelcraft_world_online_agents{world}`
- `voxelcraft_world_switch_total{from,to,result}`
- `voxelcraft_world_reset_total{world}`
- `voxelcraft_world_resource_density{world,resource}`

## 10. Security Runtime Env Toggles

- `VC_ENABLE_ADMIN_HTTP`：开启/关闭 admin HTTP 面（Cloudflare staging/prod 默认 `false`）
- `VC_ENABLE_PPROF_HTTP`：开启/关闭 pprof（默认 `false`）
- `VC_WS_ALLOW_ANY_ORIGIN`：WS 是否允许任意 Origin（staging/prod 默认 `false`）
- `VC_OBSERVER_ALLOW_ANY_ORIGIN`：observer WS 是否允许任意 Origin（staging/prod 默认 `false`）
- `VC_MCP_REQUIRE_HMAC`：MCP sidecar 是否强制 HMAC（staging/prod 默认 `true`）
- `VC_MCP_HMAC_SECRET`：MCP sidecar 的 HMAC 密钥（等价于 `cmd/mcp -hmac-secret`）
