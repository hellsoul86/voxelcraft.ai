# Config Reference (v0.9 defaults)

## Runtime
- Tick rate: 5Hz (`1 tick = 200ms`)
- Day ticks: 6000
- Chunk size: 16×16×64
- Observation radius: 7 (15×15×15 cube)
- World boundary radius: 4000 blocks
- Snapshot frequency: every 3000 ticks

Defaults live in:
- `configs/tuning.yaml`

## Systems (MVP defaults)
- Claim maintenance: every `day_ticks`
- Maintenance cost: `1x IRON_INGOT + 1x COAL`
- Maintenance downgrade: stage `0=ok`, `1=late`, `2=unprotected` (visitors treated as "wild" perms)
- Access pass "core" radius: 16 blocks (capped by claim radius)
- Blueprint auto-pull range: 32 blocks (same-land `CHEST/CONTRACT_TERMINAL`)
- Fun score decay window: 3000 ticks, multiplier base `0.70^(n-1)`
- Fun creation survival delay (blueprint structures): 3000 ticks

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
