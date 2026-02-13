import { Container, getContainer } from "@cloudflare/containers";

const WORLD_ID_RE = /^[a-zA-Z0-9_-]{1,64}$/;
const D1_SCHEMA_SQL = `
CREATE TABLE IF NOT EXISTS world_heads (
  world_id TEXT PRIMARY KEY,
  last_path TEXT NOT NULL,
  last_status INTEGER NOT NULL,
  request_count INTEGER NOT NULL DEFAULT 0,
  last_request_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_world_heads_updated_at ON world_heads(updated_at DESC);
`;

let schemaReadyPromise;

export class WorldCoordinator extends Container {
  defaultPort = 8080;
  sleepAfter = "10m";
}

function coerceWorldId(raw, fallback = "world_1") {
  const candidate = (raw || "").trim();
  if (!candidate) return fallback;
  if (!WORLD_ID_RE.test(candidate)) return fallback;
  return candidate;
}

function resolveWorldId(request, env) {
  const url = new URL(request.url);
  const fallback = coerceWorldId(env.DEFAULT_WORLD_ID, "world_1");
  return coerceWorldId(
    url.searchParams.get("world_id") ||
      url.searchParams.get("world") ||
      request.headers.get("x-voxelcraft-world"),
    fallback,
  );
}

async function ensureSchema(env) {
  if (!schemaReadyPromise) {
    schemaReadyPromise = env.VOXEL_D1.exec(D1_SCHEMA_SQL).catch((err) => {
      schemaReadyPromise = undefined;
      throw err;
    });
  }
  await schemaReadyPromise;
}

async function persistWorldHead(env, { worldId, path, status, when }) {
  await ensureSchema(env);

  await env.VOXEL_D1.prepare(
    `
      INSERT INTO world_heads (
        world_id,
        last_path,
        last_status,
        request_count,
        last_request_at,
        updated_at
      )
      VALUES (?1, ?2, ?3, 1, ?4, ?4)
      ON CONFLICT(world_id) DO UPDATE SET
        last_path = excluded.last_path,
        last_status = excluded.last_status,
        request_count = world_heads.request_count + 1,
        last_request_at = excluded.last_request_at,
        updated_at = excluded.updated_at
    `,
  )
    .bind(worldId, path, status, when)
    .run();

  const payload = {
    world_id: worldId,
    last_path: path,
    last_status: status,
    last_request_at: when,
  };

  await env.VOXEL_R2.put(`worlds/${worldId}/head.json`, JSON.stringify(payload), {
    httpMetadata: { contentType: "application/json" },
  });
}

async function readWorldHead(env, worldId) {
  await ensureSchema(env);

  const row = await env.VOXEL_D1.prepare(
    `
      SELECT
        world_id,
        last_path,
        last_status,
        request_count,
        last_request_at,
        updated_at
      FROM world_heads
      WHERE world_id = ?1
    `,
  )
    .bind(worldId)
    .first();

  const object = await env.VOXEL_R2.get(`worlds/${worldId}/head.json`);
  let r2 = null;
  if (object) {
    try {
      r2 = await object.json();
    } catch {
      r2 = { parse_error: true };
    }
  }

  return { row, r2 };
}

async function persistenceHealth(env) {
  await ensureSchema(env);
  await env.VOXEL_D1.prepare("SELECT 1 AS ok").first();
  await env.VOXEL_R2.head("_healthcheck/probe.json");

  return {
    ok: true,
    storage: {
      d1: "ok",
      r2: "ok",
    },
  };
}

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    if (url.pathname === "/_cf/persistence/healthz") {
      const payload = await persistenceHealth(env);
      return Response.json(payload);
    }

    if (url.pathname === "/_cf/persistence/head") {
      const worldId = resolveWorldId(request, env);
      const payload = await readWorldHead(env, worldId);
      return Response.json({ ok: true, world_id: worldId, ...payload });
    }

    const worldId = resolveWorldId(request, env);
    const coordinator = getContainer(env.VOXEL_WORLD, worldId);
    const response = await coordinator.fetch(request);

    ctx.waitUntil(
      persistWorldHead(env, {
        worldId,
        path: `${url.pathname}${url.search}`,
        status: response.status,
        when: new Date().toISOString(),
      }).catch((err) => {
        console.error("persist world head failed", err);
      }),
    );

    return response;
  },
};
