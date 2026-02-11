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

## Module Map

Core runtime:
- `world.go`: world construction (`New`) only
- `types_world_runtime.go`: runtime state shape (`World`)
- `types_public.go`: public structs used by transport/persistence
- `runtime_api.go`: lightweight API/accessor methods

Action handling:
- `action_apply.go`: ACT/instant/task entry dispatch
- `instant_*.go`: instant behavior handlers
- `task_handlers.go`: task request handlers
- `action_types.go`: canonical action names and dispatch validation

System loops:
- `movement_system.go`, `movement_detour.go`
- `work_system.go`
- `work_ticks_mine_place.go`
- `work_ticks_interact.go`
- `work_ticks_craft_build.go`
- `conveyor_system.go`
- `environment_system.go`

Observation path:
- `obs_builder.go`: top-level OBS assembly
- `obs_events.go`: tasks/entities/events/fun/memory projection
- `obs_voxels.go`: voxel window + delta/RLE encoding

Determinism and persistence:
- `state_digest.go`: canonical digest serialization
- `snapshot_export.go`, `snapshot_import.go`
- `audit_helpers.go`: audit event writes

Shared helpers:
- `world_blocks.go`: terrain/block helpers
- `world_helpers_runtime.go`: runtime utility helpers
- `world_helpers_misc.go`: item parsing/tax/transfer helpers
- `work_progress.go`: progress projection for OBS

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
4. rerun package + full tests
5. rerun agent E2E and swarm gate

## Validation Commands

From `voxelcraft.ai`:

```bash
go test ./internal/sim/world ./internal/sim/multiworld ./cmd/server
go test ./...
scripts/release_gate.sh --with-agent
```

