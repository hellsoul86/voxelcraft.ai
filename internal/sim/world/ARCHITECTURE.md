# World Package Architecture

This document describes the `internal/sim/world` package after the 1.0 refactor.

## Design Goals

- Keep simulation behavior deterministic: one world runtime, one authoritative tick order.
- Keep protocol behavior stable while allowing internal module refactors.
- Keep large logic split by responsibility so changes stay local and testable.

## Tick Lifecycle (Authoritative Order)

Runtime entrypoints:
- `runtime_loop.go`: `Run`, `StepOnce`
- `runtime_step.go`: `stepInternal`

Per tick order in `stepInternal`:
1. season/reset notice hooks
2. joins/leaves at boundary
3. transfer out/in (multi-world migration)
4. maintenance tick
5. apply ACT envelopes in receive order
6. systems in fixed order:
   - movement
   - work
   - conveyor
   - environment
   - laws
   - director
   - contracts
   - fun score
7. build and publish OBS
8. observer stream
9. tick digest and optional snapshot
10. metrics update and tick increment

Do not change this order without explicit determinism review.

## Package Layout

The package uses a "world facade + pure subpackages" layout.

World facade (`internal/sim/world`):
- Owns mutable runtime state and tick ordering.
- Performs all state mutation, auditing, and event emission.
- Adapts pure helpers from subpackages.

Pure subpackages:
- `internal/sim/world/logic/mathx`
- `internal/sim/world/logic/ids` for stable identifier parsing/formatting
- `internal/sim/world/logic/observerprogress`
- `internal/sim/world/logic/conveyorpower`
- `internal/sim/world/logic/directorcenter`
- `internal/sim/world/logic/movement`
- `internal/sim/world/logic/blueprint`
- `internal/sim/world/logic/rates`
- `internal/sim/world/io/snapshotcodec`
- `internal/sim/world/io/obscodec`
- `internal/sim/world/io/digestcodec`
- `internal/sim/world/policy/rules`
- `internal/sim/world/feature/admin`
- `internal/sim/world/feature/session`
- `internal/sim/world/feature/transfer`
- `internal/sim/world/feature/movement`
- `internal/sim/world/feature/work`
- `internal/sim/world/feature/economy`
- `internal/sim/world/feature/contracts`
- `internal/sim/world/feature/governance`
- `internal/sim/world/feature/director`
- `internal/sim/world/feature/observer`

Dependency rules:
1. `world` can import subpackages.
2. Subpackages do not import `internal/sim/world`.
3. `feature/*`, `io/*` and `policy/*` may depend on `logic/*`.
4. `feature/*` stays stateless and receives data/callbacks from `world` facade.
5. `logic/*` packages stay pure and deterministic.

## Main Runtime Modules

Core runtime:
- `world.go`: world construction (`New`) only
- `types_world_runtime.go`: runtime state shape (`World`)
- `runtime_loop.go`, `runtime_step.go`, `runtime_api.go`
- `session_lifecycle.go`

Action handling:
- `action_apply.go`: ACT entry + routing
- `instant_dispatch.go` and `instant_*_handlers.go`
- `task_handlers_*.go`
- `action_types.go`: canonical action names and dispatch validation

System loops:
- `movement_system.go` (uses `feature/movement` and `logic/movement`)
- `task_handlers_work.go` + `work_ticks_mine_place.go` + `work_ticks_interact.go` + `work_tick_craft_smelt.go` + `work_tick_blueprint.go` (uses `logic/blueprint` and `feature/work`)
- `conveyor_system.go` (uses `logic/conveyorpower`)
- `environment_system.go`
- `director_*.go` (uses `logic/directorcenter`)

Observation:
- `obs_builder.go`: top-level OBS assembly
- `obs_events.go`: task/entity/event projection
- `obs_voxels.go`: voxel window + delta/RLE (uses `io/obscodec`)
- `observer_*.go`: observer stream exports

Determinism and persistence:
- `state_digest*.go` (uses `io/digestcodec`)
- `snapshot_export*.go`, `snapshot_import*.go` (uses `io/snapshotcodec`)
- `audit_helpers.go`

Domain types:
- `feature/governance/laws.go` owns `Law`/`LawStatus`
- `feature/contracts/*` owns contract-domain pure rules

## Invariants

- 2D world semantics are enforced for write operations (`y == 0`).
- All mutable world state is touched only from world loop goroutine.
- Dispatch maps must stay in sync with supported action constants.
- OBS and digest serialization order must remain deterministic.

## Safe Refactor Checklist

Before moving/changing behavior:
1. keep function signatures unchanged unless all call sites are updated in one change
2. keep tick order and event ordering unchanged
3. preserve key sorting in digest and payload assembly
4. preserve OBS field ordering and voxel scan order
5. rerun agent E2E and swarm gate

## Testing Layout

- Integration tests stay in `internal/sim/world` (they need unexported state + deterministic tick wiring).
- Pure unit tests must live in their owning subpackages (`logic/*`, `feature/*`, `policy/*`, `io/*`).
- In Go, colocated `_test.go` files are the default for white-box tests. Separation is done by package boundaries, not a global `/tests` folder.
- New test default:
  1. write unit test in subpackage first
  2. add `world` integration test only for facade wiring
  3. avoid duplicate assertions across both layers

## Validation Commands

From `voxelcraft.ai`:

```bash
go test ./internal/sim/world ./internal/sim/multiworld ./cmd/server
go test ./...
scripts/release_gate.sh --with-agent
```
