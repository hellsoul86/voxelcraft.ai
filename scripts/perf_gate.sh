#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

WORLD_STEP_MAX_NS="${VC_BENCH_MAX_WORLD_STEP_NS:-500000}"
SNAP_EXPORT_MAX_NS="${VC_BENCH_MAX_SNAPSHOT_EXPORT_NS:-500000}"
SNAP_IMPORT_MAX_NS="${VC_BENCH_MAX_SNAPSHOT_IMPORT_NS:-500000}"
EVENT_CURSOR_MAX_NS="${VC_BENCH_MAX_EVENT_CURSOR_NS:-20000}"

TMP_OUT="$(mktemp)"
trap 'rm -f "${TMP_OUT}"' EXIT

go test ./internal/sim/world \
  -run '^$' \
  -bench '^BenchmarkPerf(WorldStep|SnapshotExport|SnapshotImport|EventCursorQuery)$' \
  -benchmem \
  -benchtime=20x \
  -count=1 | tee "${TMP_OUT}"

extract_ns() {
  local bench_name="$1"
  awk -v name="${bench_name}" '
    index($1, name) == 1 {
      v = $3
      sub("ns/op", "", v)
      print v
    }
  ' "${TMP_OUT}" | tail -n1
}

assert_le() {
  local metric="$1"
  local value="$2"
  local limit="$3"
  if [[ -z "${value}" ]]; then
    echo "[perf] missing metric: ${metric}" >&2
    exit 1
  fi
  awk -v v="${value}" -v m="${limit}" 'BEGIN { exit !(v <= m) }' || {
    echo "[perf] FAIL metric=${metric} value=${value}ns limit=${limit}ns" >&2
    exit 1
  }
  echo "[perf] ok metric=${metric} value=${value}ns limit=${limit}ns"
}

assert_le "BenchmarkPerfWorldStep" "$(extract_ns BenchmarkPerfWorldStep)" "${WORLD_STEP_MAX_NS}"
assert_le "BenchmarkPerfSnapshotExport" "$(extract_ns BenchmarkPerfSnapshotExport)" "${SNAP_EXPORT_MAX_NS}"
assert_le "BenchmarkPerfSnapshotImport" "$(extract_ns BenchmarkPerfSnapshotImport)" "${SNAP_IMPORT_MAX_NS}"
assert_le "BenchmarkPerfEventCursorQuery" "$(extract_ns BenchmarkPerfEventCursorQuery)" "${EVENT_CURSOR_MAX_NS}"

