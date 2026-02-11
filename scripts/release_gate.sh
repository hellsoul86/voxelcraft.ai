#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AGENT_DIR_DEFAULT="$(cd "${ROOT_DIR}/.." && pwd)/voxelcraft.agent"

WITH_AGENT=0
SKIP_RACE=0
AGENT_DIR="${AGENT_DIR_DEFAULT}"
AGENT_SCENARIO="multiworld_mine_trade_govern"
AGENT_COUNT=50
AGENT_DURATION=60

usage() {
  cat <<'USAGE'
Usage: scripts/release_gate.sh [options]

Options:
  --with-agent           Run voxelcraft.agent e2e + swarm after Go tests.
  --skip-race            Skip go test -race stage.
  --agent-dir <path>     Path to voxelcraft.agent repo (default: ../voxelcraft.agent).
  --scenario <name>      Agent scenario (default: multiworld_mine_trade_govern).
  --count <n>            Swarm agent count (default: 50).
  --duration <seconds>   Swarm duration in seconds (default: 60).
  -h, --help             Show this help text.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-agent)
      WITH_AGENT=1
      shift
      ;;
    --skip-race)
      SKIP_RACE=1
      shift
      ;;
    --agent-dir)
      AGENT_DIR="$2"
      shift 2
      ;;
    --scenario)
      AGENT_SCENARIO="$2"
      shift 2
      ;;
    --count)
      AGENT_COUNT="$2"
      shift 2
      ;;
    --duration)
      AGENT_DURATION="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage
      exit 2
      ;;
  esac
done

cd "${ROOT_DIR}"

echo "[gate] stage=core-tests"
go test ./internal/sim/world ./internal/sim/multiworld ./cmd/server

if [[ "${SKIP_RACE}" -eq 0 ]]; then
  echo "[gate] stage=race-tests"
  go test -race ./internal/sim/world ./internal/sim/multiworld ./cmd/server ./internal/openclaw/mcp ./internal/protocol ./internal/persistence/indexdb ./internal/sim/encoding
else
  echo "[gate] stage=race-tests skipped"
fi

echo "[gate] stage=full-go-tests"
go test ./...

if [[ "${WITH_AGENT}" -eq 1 ]]; then
  if [[ ! -d "${AGENT_DIR}" ]]; then
    echo "[gate] missing agent repo: ${AGENT_DIR}" >&2
    exit 1
  fi
  echo "[gate] stage=agent-e2e dir=${AGENT_DIR} scenario=${AGENT_SCENARIO}"
  (
    cd "${AGENT_DIR}"
    pnpm run e2e -- --scenario "${AGENT_SCENARIO}"
  )
  echo "[gate] stage=agent-swarm dir=${AGENT_DIR} scenario=${AGENT_SCENARIO} count=${AGENT_COUNT} duration=${AGENT_DURATION}"
  (
    cd "${AGENT_DIR}"
    pnpm run e2e:swarm -- --count "${AGENT_COUNT}" --duration_sec "${AGENT_DURATION}" --scenario "${AGENT_SCENARIO}"
  )
else
  echo "[gate] stage=agent-tests skipped"
fi

echo "[gate] done"

