# Config Reference (v0.9 defaults)

## Runtime
- Tick rate: 5Hz (`1 tick = 200ms`)
- Day ticks: 6000
- Season length: 42000 ticks (7 in-game days) (`season_length_ticks`)
- Chunk size: 16×16×64
- Observation radius: 7 (15×15×15 cube)
- World boundary radius: 4000 blocks
- Snapshot frequency: every 3000 ticks
- Director evaluation frequency: every 3000 ticks
- Rate limits (defaults; see `rate_limits` in `configs/tuning.yaml`):
  - `say`: window `50`, max `5`
  - `market_say`: window `50`, max `2`
  - `whisper`: window `50`, max `5`
  - `offer_trade`: window `50`, max `3`
  - `post_board`: window `600`, max `1`

Defaults live in:
- `configs/tuning.yaml`

## Systems (MVP defaults)
- Claim maintenance: every `day_ticks`
- Maintenance cost: `1x IRON_INGOT + 1x COAL` (`claim_maintenance_cost`)
- Maintenance downgrade: stage `0=ok`, `1=late`, `2=unprotected` (visitors treated as "wild" perms)
- Law timings: notice `3000` + vote `3000` ticks (`law_notice_ticks`, `law_vote_ticks`)
- Access pass "core" radius: 16 blocks (capped by claim radius) (`access_pass_core_radius`)
- Blueprint auto-pull range: 32 blocks (same-land `CHEST/CONTRACT_TERMINAL`) (`blueprint_auto_pull_range`)
- Blueprint build speed: 2 blocks/tick (`blueprint_blocks_per_tick`)
- Fun score decay: window `3000` ticks, multiplier base `0.70^(n-1)` (`fun_decay_window_ticks`, `fun_decay_base`)
- Fun creation survival delay (blueprint structures): 3000 ticks (`structure_survival_ticks`)

## Content Catalogs
- Blocks: `configs/blocks.json`
- Items: `configs/items.json`
- Recipes: `configs/recipes.json`
- Blueprints: `configs/blueprints/*.json`
- Law templates: `configs/law_templates.json`
- Event templates: `configs/events/*.json`

## Observability
- Prometheus text endpoint: `GET /metrics`
- Metrics (MVP):
  - `voxelcraft_world_tick`
  - `voxelcraft_world_agents`
  - `voxelcraft_world_clients`
  - `voxelcraft_world_loaded_chunks`
  - `voxelcraft_world_queue_depth{queue="inbox|join|leave|attach"}`
  - `voxelcraft_world_step_ms`
  - `voxelcraft_director_metric{metric="trade|conflict|exploration|inequality|public_infra"}`
  - `voxelcraft_stats_window{metric="trades|denied|chunks_discovered|blueprints_complete"}`
  - `voxelcraft_stats_window_ticks`

## Persistence Layout
- Tick events (JSONL zstd): `data/worlds/<world_id>/events/events-YYYY-MM-DD-HH.jsonl.zst`
- Audit (JSONL zstd): `data/worlds/<world_id>/audit/audit-YYYY-MM-DD-HH.jsonl.zst`
- Snapshots (zstd): `data/worlds/<world_id>/snapshots/<tick>.snap.zst`
- Season archives: `data/worlds/<world_id>/archives/season_<NNN>/` (contains `meta.json` + end-of-season snapshot copy)
