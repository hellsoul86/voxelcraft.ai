import { Container, getContainer } from "@cloudflare/containers";
import { env as workerEnv } from "cloudflare:workers";

const WORLD_ID_RE = /^[a-zA-Z0-9_-]{1,64}$/;
const INDEX_TOKEN_HEADER = "x-vc-index-token";

const D1_SCHEMA_STATEMENTS = [
  `
    CREATE TABLE IF NOT EXISTS world_heads (
      world_id TEXT PRIMARY KEY,
      last_path TEXT NOT NULL,
      last_status INTEGER NOT NULL,
      request_count INTEGER NOT NULL DEFAULT 0,
      last_request_at TEXT NOT NULL,
      updated_at TEXT NOT NULL
    )
  `,
  `
    CREATE INDEX IF NOT EXISTS idx_world_heads_updated_at
      ON world_heads(updated_at DESC)
  `,
];

let schemaReadyPromise;

export class WorldCoordinator extends Container {
  defaultPort = 8080;
  sleepAfter = "10m";

  // Pass runtime settings directly into the container runtime.
  envVars = {
    VC_R2_MIRROR: workerEnv.VC_R2_MIRROR ?? "false",
    VC_R2_ENDPOINT: workerEnv.VC_R2_ENDPOINT ?? "",
    VC_R2_BUCKET: workerEnv.VC_R2_BUCKET ?? "",
    VC_R2_PREFIX: workerEnv.VC_R2_PREFIX ?? "voxelcraft-ai",
    VC_R2_UPLOAD_WORKERS: workerEnv.VC_R2_UPLOAD_WORKERS ?? "2",
    VC_R2_ACCESS_KEY_ID: workerEnv.VC_R2_ACCESS_KEY_ID ?? "",
    VC_R2_SECRET_ACCESS_KEY: workerEnv.VC_R2_SECRET_ACCESS_KEY ?? "",

    VC_INDEX_BACKEND: workerEnv.VC_INDEX_BACKEND ?? "sqlite",
    VC_INDEX_D1_INGEST_URL: workerEnv.VC_INDEX_D1_INGEST_URL ?? "",
    VC_INDEX_D1_FLUSH_MS: workerEnv.VC_INDEX_D1_FLUSH_MS ?? "500",
    VC_INDEX_D1_BATCH_SIZE: workerEnv.VC_INDEX_D1_BATCH_SIZE ?? "128",
    VC_INDEX_D1_TOKEN: workerEnv.VC_INDEX_D1_TOKEN ?? "",
  };
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

function toInt(value, fallback = 0) {
  const n = Number(value);
  if (!Number.isFinite(n)) return fallback;
  return Math.trunc(n);
}

function jsonString(value) {
  try {
    return JSON.stringify(value ?? null);
  } catch {
    return "null";
  }
}

function vec3(value) {
  const a = Array.isArray(value) ? value : [];
  return [toInt(a[0], 0), toInt(a[1], 0), toInt(a[2], 0)];
}

function nowISO() {
  return new Date().toISOString();
}

function badRequest(message, status = 400) {
  return Response.json({ ok: false, error: message }, { status });
}

async function ensureSchema(env) {
  if (!schemaReadyPromise) {
    schemaReadyPromise = (async () => {
      for (const statement of D1_SCHEMA_STATEMENTS) {
        await env.VOXEL_D1.prepare(statement).run();
      }
    })().catch((err) => {
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

async function indexdbHealth(env, worldId) {
  await ensureSchema(env);
  await env.VOXEL_D1.prepare("SELECT 1 AS ok").first();

  const ticks = await env.VOXEL_D1.prepare(
    "SELECT COUNT(1) AS n FROM index_ticks WHERE world_id = ?1",
  )
    .bind(worldId)
    .first();
  const audits = await env.VOXEL_D1.prepare(
    "SELECT COUNT(1) AS n FROM index_audits WHERE world_id = ?1",
  )
    .bind(worldId)
    .first();
  const snapshots = await env.VOXEL_D1.prepare(
    "SELECT COUNT(1) AS n FROM index_snapshots WHERE world_id = ?1",
  )
    .bind(worldId)
    .first();

  return {
    ok: true,
    world_id: worldId,
    index: {
      ticks: toInt(ticks?.n, 0),
      audits: toInt(audits?.n, 0),
      snapshots: toInt(snapshots?.n, 0),
    },
  };
}

function isIndexIngestAuthorized(request, env) {
  const expected = (env.VC_INDEX_D1_TOKEN || "").trim();
  if (!expected) return false;
  const actual = (request.headers.get(INDEX_TOKEN_HEADER) || "").trim();
  return actual !== "" && actual === expected;
}

async function handleIndexIngest(request, env) {
  if (request.method !== "POST") {
    return badRequest("method not allowed", 405);
  }
  if (!isIndexIngestAuthorized(request, env)) {
    return badRequest("forbidden", 403);
  }

  const body = await request.json().catch(() => null);
  const events = Array.isArray(body?.events) ? body.events : null;
  if (!events || events.length === 0) {
    return badRequest("missing events");
  }
  if (events.length > 1024) {
    return badRequest("too many events", 413);
  }

  await ensureSchema(env);

  let applied = 0;
  for (const ev of events) {
    if (!ev || typeof ev !== "object") continue;
    const kind = String(ev.kind || "").trim();
    const worldId = coerceWorldId(ev.world_id, "world_1");
    const payload = ev.payload;
    if (!kind || !payload || typeof payload !== "object") continue;
    await applyIndexEvent(env, kind, worldId, payload);
    applied += 1;
  }

  return Response.json({ ok: true, applied });
}

async function applyIndexEvent(env, kind, worldId, payload) {
  switch (kind) {
    case "tick":
      await upsertTick(env, worldId, payload);
      return;
    case "audit":
      await upsertAudit(env, worldId, payload);
      return;
    case "snapshot":
      await upsertSnapshot(env, worldId, payload);
      return;
    case "snapshot_state":
      await upsertSnapshotState(env, worldId, payload);
      return;
    case "season":
      await upsertSeason(env, worldId, payload);
      return;
    case "catalog":
      await upsertCatalog(env, worldId, payload);
      return;
    default:
      return;
  }
}

async function upsertTick(env, worldId, payload) {
  const tick = toInt(payload.tick, 0);
  const joins = Array.isArray(payload.joins) ? payload.joins : [];
  const leaves = Array.isArray(payload.leaves) ? payload.leaves : [];
  const actions = Array.isArray(payload.actions) ? payload.actions : [];
  const when = nowISO();

  await env.VOXEL_D1.prepare(
    `
      INSERT INTO index_ticks (
        world_id,
        tick,
        digest,
        joins_json,
        leaves_json,
        actions_json,
        raw_json,
        joins_count,
        leaves_count,
        actions_count,
        updated_at
      )
      VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11)
      ON CONFLICT(world_id, tick) DO UPDATE SET
        digest = excluded.digest,
        joins_json = excluded.joins_json,
        leaves_json = excluded.leaves_json,
        actions_json = excluded.actions_json,
        raw_json = excluded.raw_json,
        joins_count = excluded.joins_count,
        leaves_count = excluded.leaves_count,
        actions_count = excluded.actions_count,
        updated_at = excluded.updated_at
    `,
  )
    .bind(
      worldId,
      tick,
      String(payload.digest || ""),
      jsonString(joins),
      jsonString(leaves),
      jsonString(actions),
      jsonString(payload),
      joins.length,
      leaves.length,
      actions.length,
      when,
    )
    .run();
}

async function upsertAudit(env, worldId, payload) {
  const pos = vec3(payload.pos);
  const when = nowISO();
  await env.VOXEL_D1.prepare(
    `
      INSERT INTO index_audits (
        world_id,
        tick,
        seq,
        actor,
        action,
        x,
        y,
        z,
        from_block,
        to_block,
        reason,
        raw_json,
        updated_at
      )
      VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13)
      ON CONFLICT(world_id, tick, seq) DO UPDATE SET
        actor = excluded.actor,
        action = excluded.action,
        x = excluded.x,
        y = excluded.y,
        z = excluded.z,
        from_block = excluded.from_block,
        to_block = excluded.to_block,
        reason = excluded.reason,
        raw_json = excluded.raw_json,
        updated_at = excluded.updated_at
    `,
  )
    .bind(
      worldId,
      toInt(payload.tick, 0),
      toInt(payload.seq, 0),
      String(payload.actor || ""),
      String(payload.action || ""),
      pos[0],
      pos[1],
      pos[2],
      toInt(payload.from, 0),
      toInt(payload.to, 0),
      String(payload.reason || ""),
      jsonString(payload.raw || payload),
      when,
    )
    .run();
}

async function upsertSnapshot(env, worldId, payload) {
  const when = nowISO();
  await env.VOXEL_D1.prepare(
    `
      INSERT INTO index_snapshots (
        world_id,
        tick,
        path,
        seed,
        height,
        chunks,
        agents,
        claims,
        containers,
        contracts,
        laws,
        orgs,
        updated_at
      )
      VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13)
      ON CONFLICT(world_id, tick) DO UPDATE SET
        path = excluded.path,
        seed = excluded.seed,
        height = excluded.height,
        chunks = excluded.chunks,
        agents = excluded.agents,
        claims = excluded.claims,
        containers = excluded.containers,
        contracts = excluded.contracts,
        laws = excluded.laws,
        orgs = excluded.orgs,
        updated_at = excluded.updated_at
    `,
  )
    .bind(
      worldId,
      toInt(payload.tick, 0),
      String(payload.path || ""),
      toInt(payload.seed, 0),
      toInt(payload.height, 0),
      toInt(payload.chunks, 0),
      toInt(payload.agents, 0),
      toInt(payload.claims, 0),
      toInt(payload.containers, 0),
      toInt(payload.contracts, 0),
      toInt(payload.laws, 0),
      toInt(payload.orgs, 0),
      when,
    )
    .run();
}

async function upsertSnapshotState(env, worldId, payload) {
  const tick = toInt(payload.tick, 0);
  const eventCenter = vec3(payload.active_event_center);
  const when = nowISO();

  await env.VOXEL_D1.prepare(
    `
      INSERT INTO index_snapshot_world (
        world_id,
        tick,
        weather,
        weather_until_tick,
        active_event_id,
        active_event_start_tick,
        active_event_ends_tick,
        active_event_center_x,
        active_event_center_y,
        active_event_center_z,
        active_event_radius,
        updated_at
      )
      VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12)
      ON CONFLICT(world_id, tick) DO UPDATE SET
        weather = excluded.weather,
        weather_until_tick = excluded.weather_until_tick,
        active_event_id = excluded.active_event_id,
        active_event_start_tick = excluded.active_event_start_tick,
        active_event_ends_tick = excluded.active_event_ends_tick,
        active_event_center_x = excluded.active_event_center_x,
        active_event_center_y = excluded.active_event_center_y,
        active_event_center_z = excluded.active_event_center_z,
        active_event_radius = excluded.active_event_radius,
        updated_at = excluded.updated_at
    `,
  )
    .bind(
      worldId,
      tick,
      String(payload.weather || ""),
      toInt(payload.weather_until_tick, 0),
      String(payload.active_event_id || ""),
      toInt(payload.active_event_start_tick, 0),
      toInt(payload.active_event_ends_tick, 0),
      eventCenter[0],
      eventCenter[1],
      eventCenter[2],
      toInt(payload.active_event_radius, 0),
      when,
    )
    .run();

  await env.VOXEL_D1.prepare(
    "DELETE FROM index_snapshot_agents WHERE world_id = ?1 AND tick = ?2",
  )
    .bind(worldId, tick)
    .run();
  await env.VOXEL_D1.prepare(
    "DELETE FROM index_snapshot_boards WHERE world_id = ?1 AND tick = ?2",
  )
    .bind(worldId, tick)
    .run();
  await env.VOXEL_D1.prepare(
    "DELETE FROM index_snapshot_board_posts WHERE world_id = ?1 AND tick = ?2",
  )
    .bind(worldId, tick)
    .run();
  await env.VOXEL_D1.prepare(
    "DELETE FROM index_snapshot_trades WHERE world_id = ?1 AND tick = ?2",
  )
    .bind(worldId, tick)
    .run();

  const agents = Array.isArray(payload.agents) ? payload.agents : [];
  for (const a of agents) {
    const pos = vec3(a?.pos);
    await env.VOXEL_D1.prepare(
      `
        INSERT INTO index_snapshot_agents (
          world_id,
          tick,
          agent_id,
          name,
          org_id,
          x,
          y,
          z,
          yaw,
          hp,
          hunger,
          stamina_milli,
          rep_trade,
          rep_build,
          rep_social,
          rep_law,
          fun_novelty,
          fun_creation,
          fun_social,
          fun_influence,
          fun_narrative,
          fun_risk_rescue,
          inventory_json,
          updated_at
        )
        VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15, ?16, ?17, ?18, ?19, ?20, ?21, ?22, ?23, ?24)
      `,
    )
      .bind(
        worldId,
        tick,
        String(a?.id || ""),
        String(a?.name || ""),
        String(a?.org_id || ""),
        pos[0],
        pos[1],
        pos[2],
        toInt(a?.yaw, 0),
        toInt(a?.hp, 0),
        toInt(a?.hunger, 0),
        toInt(a?.stamina_milli, 0),
        toInt(a?.rep_trade, 0),
        toInt(a?.rep_build, 0),
        toInt(a?.rep_social, 0),
        toInt(a?.rep_law, 0),
        toInt(a?.fun_novelty, 0),
        toInt(a?.fun_creation, 0),
        toInt(a?.fun_social, 0),
        toInt(a?.fun_influence, 0),
        toInt(a?.fun_narrative, 0),
        toInt(a?.fun_risk_rescue, 0),
        jsonString(a?.inventory || {}),
        when,
      )
      .run();
  }

  const boards = Array.isArray(payload.boards) ? payload.boards : [];
  for (const b of boards) {
    const boardId = String(b?.board_id || "");
    let kind = "GLOBAL";
    let x = null;
    let y = null;
    let z = null;
    if (boardId.startsWith("board@")) {
      kind = "LOCAL";
      const coords = boardId.slice("board@".length).split(",");
      if (coords.length === 3) {
        x = toInt(coords[0], null);
        y = toInt(coords[1], null);
        z = toInt(coords[2], null);
      }
    }

    const posts = Array.isArray(b?.posts) ? b.posts : [];
    await env.VOXEL_D1.prepare(
      `
        INSERT INTO index_snapshot_boards (
          world_id,
          tick,
          board_id,
          kind,
          x,
          y,
          z,
          posts_count,
          updated_at
        )
        VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)
      `,
    )
      .bind(worldId, tick, boardId, kind, x, y, z, posts.length, when)
      .run();

    for (const p of posts) {
      await env.VOXEL_D1.prepare(
        `
          INSERT INTO index_snapshot_board_posts (
            world_id,
            tick,
            board_id,
            post_id,
            author,
            title,
            body,
            post_tick,
            updated_at
          )
          VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)
        `,
      )
        .bind(
          worldId,
          tick,
          boardId,
          String(p?.post_id || ""),
          String(p?.author || ""),
          String(p?.title || ""),
          String(p?.body || ""),
          toInt(p?.tick, 0),
          when,
        )
        .run();
    }
  }

  const trades = Array.isArray(payload.trades) ? payload.trades : [];
  for (const t of trades) {
    await env.VOXEL_D1.prepare(
      `
        INSERT INTO index_snapshot_trades (
          world_id,
          tick,
          trade_id,
          from_agent,
          to_agent,
          created_tick,
          offer_json,
          request_json,
          updated_at
        )
        VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)
      `,
    )
      .bind(
        worldId,
        tick,
        String(t?.trade_id || ""),
        String(t?.from || ""),
        String(t?.to || ""),
        toInt(t?.created_tick, 0),
        jsonString(t?.offer || {}),
        jsonString(t?.request || {}),
        when,
      )
      .run();
  }
}

async function upsertSeason(env, worldId, payload) {
  await env.VOXEL_D1.prepare(
    `
      INSERT INTO index_seasons (
        world_id,
        season,
        end_tick,
        seed,
        snapshot_path,
        recorded_at
      )
      VALUES (?1, ?2, ?3, ?4, ?5, ?6)
      ON CONFLICT(world_id, season) DO UPDATE SET
        end_tick = excluded.end_tick,
        seed = excluded.seed,
        snapshot_path = excluded.snapshot_path,
        recorded_at = excluded.recorded_at
    `,
  )
    .bind(
      worldId,
      toInt(payload.season, 0),
      toInt(payload.end_tick, 0),
      toInt(payload.seed, 0),
      String(payload.path || ""),
      String(payload.recorded_at || nowISO()),
    )
    .run();
}

async function upsertCatalog(env, worldId, payload) {
  await env.VOXEL_D1.prepare(
    `
      INSERT INTO index_catalogs (
        world_id,
        name,
        digest,
        json,
        updated_at
      )
      VALUES (?1, ?2, ?3, ?4, ?5)
      ON CONFLICT(world_id, name) DO UPDATE SET
        digest = excluded.digest,
        json = excluded.json,
        updated_at = excluded.updated_at
    `,
  )
    .bind(
      worldId,
      String(payload.name || ""),
      String(payload.digest || ""),
      String(payload.json || "{}"),
      String(payload.updated_at || nowISO()),
    )
    .run();
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

    if (url.pathname === "/_cf/indexdb/healthz") {
      const worldId = resolveWorldId(request, env);
      const payload = await indexdbHealth(env, worldId);
      return Response.json(payload);
    }

    if (url.pathname === "/_cf/indexdb/ingest") {
      return handleIndexIngest(request, env);
    }

    const worldId = resolveWorldId(request, env);
    const coordinator = getContainer(env.VOXEL_WORLD, worldId);
    const response = await coordinator.fetch(request);

    ctx.waitUntil(
      persistWorldHead(env, {
        worldId,
        path: `${url.pathname}${url.search}`,
        status: response.status,
        when: nowISO(),
      }).catch((err) => {
        console.error("persist world head failed", err);
      }),
    );

    return response;
  },
};
