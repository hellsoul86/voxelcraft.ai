# VoxelCraft: AI Civilizations (Headless Backend) v0.9

This repo is the headless server implementation of **VoxelCraft: AI Civilizations**.

## Quickstart

Requirements:
- Go 1.22+

Run server:
```bash
go run ./cmd/server -addr :8080 -world world_1 -seed 1337
```

Resume from snapshots:
- By default the server will load the latest snapshot under `data/worlds/<world>/snapshots/` if present.
- To start fresh: `-load_latest_snapshot=false`
- To load a specific snapshot: `-snapshot /path/to/<tick>.snap.zst`

Run a simple bot client:
```bash
go run ./cmd/bot -url ws://localhost:8080/v1/ws -name bot1
```

Endpoints:
- `GET /healthz`
- `GET /metrics` (minimal Prometheus text)
- `GET /debug/pprof/` (pprof)

Persistence (defaults under `./data`):
- tick log: `data/worlds/<world>/events/*.jsonl.zst`
- audit log: `data/worlds/<world>/audit/*.jsonl.zst`
- snapshots: `data/worlds/<world>/snapshots/*.snap.zst` (every 3000 ticks)

Admin tools:
- Rollback a region using audit logs (offline):  
  `go run ./cmd/admin rollback -data ./data -world world_1 -aabb 0,0,0:32,64,32 -since_tick 10000`

## Protocol
- WebSocket endpoint: `/v1/ws`
- Protocol version: `0.9`

See:
- `docs/spec/agent_protocol_v0.9.md`
- `schemas/*.schema.json`
