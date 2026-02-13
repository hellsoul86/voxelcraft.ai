# Cloudflare Production Deployment (Containers + Durable Objects + D1 + R2)

This document describes the production deployment path for `voxelcraft.ai` using Cloudflare.

## Architecture

- **Cloudflare Worker**: public HTTP/WS entrypoint.
- **Durable Object (Container-backed)**: `WorldCoordinator` routes requests by `shard_id` (legacy `world_id` alias still accepted) to container instances.
- **Cloudflare Containers**: run the Go server (`cmd/server`) from `Dockerfile.cloudflare`.
- Runtime hardening defaults in Cloudflare env: admin HTTP and pprof endpoints disabled (`VC_ENABLE_ADMIN_HTTP=false`, `VC_ENABLE_PPROF_HTTP=false`).
- **D1**: stores request metadata (`world_heads`) and Cloud index tables (replacing local sqlite index in Cloudflare runtime).
- **R2**: stores the latest world head JSON (`worlds/<world_id>/head.json`).
- **Container->R2 mirror (S3 API)**: server snapshots/events/audit files are uploaded from container runtime to R2 asynchronously.

## GitHub Actions workflow

Workflow file: `.github/workflows/deploy-cloudflare-production.yml`

Trigger:
- `push` to `main` (selected paths, automatic production deployment)
- manual `workflow_dispatch`

Pipeline steps:
1. Run `scripts/release_gate.sh --skip-race`
2. Install Cloudflare deployment dependencies (`cloudflare/package.json`)
3. Render `cloudflare/wrangler.generated.toml` from placeholders
4. Apply D1 schema (`cloudflare/d1/schema.sql`)
5. Deploy Worker + Container (`wrangler deploy --env production`)

## Required GitHub Actions config

Repository-level:
- Secret: `CLOUDFLARE_API_TOKEN`
- Variable: `CLOUDFLARE_ACCOUNT_ID`

Environment-level (`production`):
- Variable: `CLOUDFLARE_D1_DATABASE_ID`
- Variable: `CLOUDFLARE_R2_BUCKET`
- Secret: `VC_R2_ACCESS_KEY_ID`
- Secret: `VC_R2_SECRET_ACCESS_KEY`

The deploy workflow is bound to `environment: production`.

## Production resource naming

Suggested names:
- Worker: `voxelcraft-ai-production`
- D1 database: `voxelcraft-ai-production`
- R2 bucket: `voxelcraft-ai-production-state`
- Custom domain: `api.voxelcraft.ai`

Custom domain is configured in `cloudflare/wrangler.toml` via:

```toml
[[env.production.routes]]
pattern = "api.voxelcraft.ai"
custom_domain = true
```

For `VC_R2_ACCESS_KEY_ID` / `VC_R2_SECRET_ACCESS_KEY`, create an R2 API token pair in Cloudflare (S3-compatible credentials) with read/write access to the production bucket, then store those values as `production` environment secrets in GitHub Actions.

`VC_INDEX_D1_TOKEN` is derived automatically in workflow from existing `CLOUDFLARE_API_TOKEN` and written as Worker secret (no extra GitHub secret required).

## Release flow

- Commit to `staging` for pre-release verification.
- Validate `staging-api.voxelcraft.ai`.
- Merge `staging` into `main`.
- Push on `main` automatically deploys production.

## Runtime diagnostics endpoints

- `GET /healthz`
- `GET /_cf/persistence/healthz`
- `GET /_cf/persistence/head?shard_id=world_1` (`world_id` still accepted as legacy alias)
- `GET /_cf/indexdb/healthz?world_id=OVERWORLD`
