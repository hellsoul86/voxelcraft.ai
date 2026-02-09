# Config Reference (v0.9 defaults)

## Runtime
- Tick rate: 5Hz (`1 tick = 200ms`)
- Day ticks: 6000
- Chunk size: 16×16×64
- Observation radius: 7 (15×15×15 cube)
- World boundary radius: 4000 blocks
- Snapshot frequency: every 3000 ticks
- Director evaluation frequency: every 3000 ticks
- Rate limits: see `rate_limits` in `configs/tuning.yaml`

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

## Persistence Layout
- Tick events (JSONL zstd): `data/worlds/<world_id>/events/events-YYYY-MM-DD-HH.jsonl.zst`
- Audit (JSONL zstd): `data/worlds/<world_id>/audit/audit-YYYY-MM-DD-HH.jsonl.zst`
- Snapshots (zstd): `data/worlds/<world_id>/snapshots/<tick>.snap.zst`
