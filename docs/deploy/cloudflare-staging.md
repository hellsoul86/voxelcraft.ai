# Cloudflare Staging Deployment (Containers + Durable Objects + D1 + R2)

This document describes the staging deployment path for `voxelcraft.ai` using Cloudflare.

## Architecture

- **Cloudflare Worker**: public HTTP/WS entrypoint.
- **Durable Object (Container-backed)**: `WorldCoordinator` routes requests by `world_id` to container instances.
- **Cloudflare Containers**: run the Go server (`cmd/server`) from `Dockerfile.cloudflare`.
- **D1**: stores request metadata (`world_heads`) for quick state visibility.
- **R2**: stores the latest world head JSON (`worlds/<world_id>/head.json`).

> Note: this is staging-first infrastructure. It gives durable coordination + persistent metadata/object storage,
> while the game server runtime itself still keeps local runtime data inside the container filesystem.

## GitHub Actions workflow

Workflow file: `.github/workflows/deploy-cloudflare-staging.yml`

Trigger:
- `push` to `staging` (selected paths)
- manual `workflow_dispatch`

Pipeline steps:
1. Run `scripts/release_gate.sh --skip-race`
2. Install Cloudflare deployment dependencies (`cloudflare/package.json`)
3. Render `cloudflare/wrangler.generated.toml` from placeholders
4. Apply D1 schema (`cloudflare/d1/schema.sql`)
5. Deploy Worker + Container (`wrangler deploy --env staging`)

## Required GitHub Actions config

Repository-level:
- Secret: `CLOUDFLARE_API_TOKEN`
- Variable: `CLOUDFLARE_ACCOUNT_ID`

Environment-level (`staging`):
- Variable: `CLOUDFLARE_D1_DATABASE_ID`
- Variable: `CLOUDFLARE_R2_BUCKET`

The deploy workflow is bound to `environment: staging`.

## Staging resource naming

Suggested names (prefix `voxelcraft-ai-*`):
- Worker: `voxelcraft-ai-staging`
- D1 database: `voxelcraft-ai-staging`
- R2 bucket: `voxelcraft-ai-staging-state`
- Custom domain: `staging-api.voxelcraft.ai`

Custom domain is configured in `cloudflare/wrangler.toml` via:

```toml
[[env.staging.routes]]
pattern = "staging-api.voxelcraft.ai"
custom_domain = true
```

Cloudflare will create DNS records/certificates automatically when the deploy token has the required zone permissions.

## Optional manual bootstrap commands

```bash
# from cloudflare/ directory
npx wrangler d1 create voxelcraft-ai-staging
npx wrangler r2 bucket create voxelcraft-ai-staging-state
```

Then set the returned D1 `database_id` and R2 bucket name as environment variables in GitHub Actions (`staging`).

## Release flow

- Commit to `staging` and validate the deployment at `staging-api.voxelcraft.ai`.
- After verification, merge `staging` into `main`.
- Push on `main` triggers automatic production deployment (`api.voxelcraft.ai`).

## Runtime diagnostics endpoints

- `GET /healthz` (from Go server)
- `GET /_cf/persistence/healthz` (Worker checks D1 + R2)
- `GET /_cf/persistence/head?world_id=world_1` (latest head from D1/R2)
