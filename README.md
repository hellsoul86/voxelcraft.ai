# VoxelCraft: AI Civilizations (Headless Backend) v0.9

This repo is the headless server implementation of **VoxelCraft: AI Civilizations**.

World model (v0.9.x MVP):
- The server currently runs a **2D tilemap** world (`height=1`, `chunk_size=[16,16,1]`).
- All world-write positions must use `y==0` (e.g. `MINE/PLACE/BUILD_BLUEPRINT/CLAIM_LAND`), otherwise actions fail with `E_INVALID_TARGET`.
- Old 3D snapshots (`height=64`) are not supported; start a fresh world/data dir.

## Quickstart

Requirements:
- Go 1.22+

Run server:
```bash
go run ./cmd/server -addr :8080 -world world_1 -seed 1337
```

Runtime tuning:
- Defaults live in `configs/tuning.yaml`
- Override path via `-tuning /path/to/tuning.yaml`

Resume from snapshots:
- By default the server will load the latest snapshot under `data/worlds/<world>/snapshots/` if present.
- To start fresh: `-load_latest_snapshot=false`
- To load a specific snapshot: `-snapshot /path/to/<tick>.snap.zst`
  - Note: snapshot import requires `height=1` in v0.9.x; delete old data if you previously ran a 3D build.

Run a simple bot client:
```bash
go run ./cmd/bot -url ws://localhost:8080/v1/ws -name bot1
```

Run MCP sidecar (for OpenClaw/ClawHub-style agents):
```bash
go run ./cmd/mcp -listen 127.0.0.1:8090 -world-ws-url ws://127.0.0.1:8080/v1/ws
```

MCP smoke test:
```bash
curl -s http://127.0.0.1:8090/mcp \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"list_tools"}' | jq .

curl -s http://127.0.0.1:8090/mcp \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"call_tool","params":{"name":"voxelcraft.get_obs","arguments":{"mode":"summary","wait_new_tick":true,"timeout_ms":2000}}}' | jq .
```

Endpoints:
- `GET /healthz`
- `GET /metrics` (Prometheus text)
- `GET /admin/v1/state` (loopback-only)
- `POST /admin/v1/snapshot` (loopback-only; force a snapshot)
- Multi-world mode:
  - `GET /admin/v1/worlds/state` (loopback-only)
  - `POST /admin/v1/worlds/{id}/reset` (loopback-only; guarded by `allow_admin_reset`)
  - `POST /admin/v1/agents/{id}/move_world?target_world=<id>` (loopback-only; operator rescue)
- `GET /admin/v1/observer/bootstrap` (loopback-only; observer bootstrap)
- `WS /admin/v1/observer/ws` (loopback-only; observer stream)
- `GET /debug/pprof/` (pprof)

Default reset guard in `configs/worlds.yaml`:
- `OVERWORLD`, `CITY_HUB`: reset disabled (`403`)
- `MINE_L1/L2/L3`: reset enabled

Local admin smoke test:
```bash
curl -s http://127.0.0.1:8080/admin/v1/state | jq .
curl -s -XPOST http://127.0.0.1:8080/admin/v1/snapshot | jq .
curl -s http://127.0.0.1:8080/admin/v1/observer/bootstrap | jq .
```

Or via CLI:
```bash
go run ./cmd/admin state -url http://127.0.0.1:8080
go run ./cmd/admin snapshot -url http://127.0.0.1:8080
```

## Release Gate (local)

Run deterministic + regression checks before cutting a release:

```bash
# core + race + full go test
scripts/release_gate.sh

# include voxelcraft.agent e2e and swarm
scripts/release_gate.sh --with-agent
```

Optional flags:
- `--skip-race`
- `--agent-dir /path/to/voxelcraft.agent`
- `--scenario multiworld_mine_trade_govern`
- `--count 50 --duration 60`

## GitHub Actions

Two workflows are wired:

- `CI Fast` (`.github/workflows/ci-fast.yml`)
  - triggers on PR and push to `main/master`
  - runs `scripts/release_gate.sh --skip-race`
  - intended for fast feedback

- `CI Full` (`.github/workflows/ci-full.yml`)
  - triggers on push to `main/master`, daily schedule, and manual dispatch
  - runs `scripts/release_gate.sh` (includes `-race`)
  - manual dispatch can enable optional `voxelcraft.agent` e2e + swarm
  - agent repo defaults to `<owner>/voxelcraft.agent` and can be overridden by input

Persistence (defaults under `./data`):
- tick log: `data/worlds/<world>/events/*.jsonl.zst`
- audit log: `data/worlds/<world>/audit/*.jsonl.zst`
- snapshots: `data/worlds/<world>/snapshots/*.snap.zst` (every `snapshot_every_ticks`, default 3000)
- sqlite index (read model): `data/worlds/<world>/index/world.sqlite` (can be disabled via `-disable_db`)

Admin tools:
- Rollback a region using audit logs (offline):  
  `go run ./cmd/admin rollback -data ./data -world world_1 -aabb 0,0,0:32,64,32 -since_tick 10000`

Rollback limitations (v0.9):
- Rollback currently reverts **only** `SET_BLOCK` (voxel edits) from the audit log.
- It does **not** rollback container inventories, trades, contracts, org treasuries, dropped item entities, etc.
- `-only_illegal` is not supported (illegal edits are rejected before audit logging).

## Protocol
- WebSocket endpoint: `/v1/ws`
- Protocol version: `0.9`

See:
- `docs/spec/agent_protocol_v0.9.md`
- `schemas/*.schema.json`
- `internal/sim/world/ARCHITECTURE.md` (world package structure and tick lifecycle)

OpenClaw/ClawHub (MCP sidecar):
- `POST /mcp` on the sidecar (default: `http://127.0.0.1:8090/mcp`)
- Skill doc: `docs/CLAWHUB_SKILL.md`
