# VoxelCraft Agent Protocol v0.9

目标：让 agent 获取结构化观测（OBS）并输出高层动作（ACT），避免陷入像素感知与逐格摆块细节。

## 1. Transport

- WebSocket endpoint: `/v1/ws`
- Encoding: JSON text frames
- Protocol version: `0.9`
- 连接后 **5 秒内必须发送 HELLO**，否则服务器断开连接

## 2. Session Lifecycle

1. Client -> Server: `HELLO`
2. Server -> Client: `WELCOME`
3. Server -> Client: `CATALOG`（可分片，MVP 单片）
4. Tick loop:
   - Server -> Client: `OBS`（每 tick）
   - Client -> Server: `ACT`（可为空）

重连（MVP）：
- Client 可在 `HELLO.auth.token` 中带上上一次 `WELCOME.resume_token`，服务端会尽力恢复同一个 `agent_id`（连接级功能，不影响世界确定性）。

## 3. HELLO

```json
{
  "type":"HELLO",
  "protocol_version":"0.9",
  "agent_name":"AliceBot",
  "capabilities":{
    "delta_voxels":true,
    "max_queue":8
  },
  "auth":{"token":"OPTIONAL"}
}
```

## 4. WELCOME

```json
{
  "type":"WELCOME",
  "protocol_version":"0.9",
  "agent_id":"A17",
  "resume_token":"resume_world_1_...",
  "world_params":{
    "tick_rate_hz":5,
    "chunk_size":[16,16,64],
    "height":64,
    "obs_radius":7,
    "day_ticks":6000,
    "seed":1337
  },
  "catalogs":{
    "block_palette":{"digest":"...","count":28},
    "item_palette":{"digest":"...","count":40},
    "recipes_digest":"...",
    "blueprints_digest":"...",
    "law_templates_digest":"...",
    "events_digest":"..."
  }
}
```

## 5. CATALOG

```json
{
  "type":"CATALOG",
  "protocol_version":"0.9",
  "name":"block_palette",
  "digest":"...",
  "part":1,
  "total_parts":1,
  "data":["AIR","DIRT","GRASS","SAND","STONE", "..."]
}
```

补充（MVP 实现）：
- 服务器会额外下发 `name="tuning"` 的 catalog，用于告知运行参数（例如 `snapshot_every_ticks`、`director_every_ticks`、`rate_limits` 等），方便 agent 做冷却/调度推理。

## 6. OBS

关键点：
- `OBS.tick` 是权威 tick
- agent 以它为基准回 `ACT.tick`（通常等于最后收到的 OBS.tick）

见 `schemas/obs.schema.json`。

补充（MVP 实现）：
- `local_rules.role` 会给出当前所处地块语义角色：`WILD|OWNER|MEMBER|VISITOR`
- `local_rules.owner` 可能是 agent id 或 org id（如 `ORG000001`）
- `local_rules.maintenance_due_tick` / `local_rules.maintenance_stage` 用于领地维护费与保护降级提示
- `fun_score`（可选）提供多维 Fun Score 面板：`novelty/creation/social/influence/narrative/risk_rescue`
- `events` 中可能出现：`FUN`（fun 变动明细）、`FINE`（罚款）、`ACCESS_PASS`（门票扣除）
- `entities`（MVP）可能包含具备额外 tags 的功能块：
  - `CONVEYOR`：`tags=["dir:+X|-X|+Z|-Z"]`
  - `SWITCH`：`tags=["state:on|off"]`
  - `SENSOR`：`tags=["state:on|off"]`（当前默认规则：附近掉落物或相邻容器有可用库存时为 on）

## 7. ACT

规则：
- `ACT.tick` 表示该 ACT 响应的最后一次 OBS.tick
- 服务器只接受 `ACT.tick` 在 `[current_tick-2, current_tick]` 范围内的动作，否则 `E_STALE`
- 两级动作：`instants`（即时）与 `tasks`（持续）
- 并发限制：每 agent 同时最多 1 个 Movement Task + 1 个 Work Task

见 `schemas/act.schema.json`。

### 7.1 MVP 已实现动作

Instants:
- `SAY(channel,text)`
- `WHISPER(to,text)`
- `EAT(item_id,count?)`（食物回血+回饥饿+回体力）
- `OFFER_TRADE(to, offer, request)`
- `ACCEPT_TRADE(trade_id)`
- `DECLINE_TRADE(trade_id)`
- `POST_BOARD(board_id,title,body)`
- `SEARCH_BOARD(board_id,text,limit?)`
- `SET_SIGN(target_id,text)`（target_id 形如 `SIGN@x,y,z`）
- `TOGGLE_SWITCH(target_id)`（target_id 形如 `SWITCH@x,y,z`）
- `SET_PERMISSIONS(land_id, policy)` (claim flags)
- `ADD_MEMBER(land_id, member_id)` / `REMOVE_MEMBER(land_id, member_id)` (claim members)
- `CREATE_ORG(org_kind, org_name)` -> `org_id` (kinds: `GUILD|CITY`)
- `JOIN_ORG(org_id)` / `LEAVE_ORG()`
- `ORG_DEPOSIT(org_id,item_id,count)` / `ORG_WITHDRAW(org_id,item_id,count)` (withdraw requires org admin)
- `DEED_LAND(land_id, new_owner)` (new_owner can be agent_id or org_id)
- `PROPOSE_LAW(land_id, template_id, params, title?)` (MVP: land owner or member; supported templates: `MARKET_TAX`, `CURFEW_NO_BUILD`, `FINE_BREAK_PER_BLOCK`, `ACCESS_PASS_CORE`)
- `VOTE(law_id, choice)` (MVP: land owner or member; `choice` in `YES|NO|ABSTAIN`)
- `SAVE_MEMORY(key,value,ttl_ticks)`
- `LOAD_MEMORY(prefix,limit)`

Tasks:
- `MOVE_TO(target,tolerance)`
- `FOLLOW(target_id,distance?)`
- `STOP()`
- `MINE(block_pos)`
- `GATHER(target_id)`（拾取掉落物，target_id 为 item entity id）
- `PLACE(block_pos,item_id)` (single-block placement)
- `OPEN(target_id)` (open container/terminal)
- `TRANSFER(src_container,dst_container,item_id,count)` (`SELF` is allowed)
- `CRAFT(recipe_id,count)`
- `SMELT(item_id,count)`
- `BUILD_BLUEPRINT(blueprint_id,anchor,rotation)`（MVP：若背包材料不足，会尝试从 `anchor` 32 格内、同一领地内的 `CHEST/CONTRACT_TERMINAL` 自动补齐）
- `CLAIM_LAND(anchor,radius)`

Contracts (Instants):
- `POST_CONTRACT(terminal_id, contract_kind, requirements, reward, deposit?, duration_ticks?|deadline_tick, blueprint_id?, anchor?, rotation?)`
- `ACCEPT_CONTRACT(terminal_id, contract_id)`
- `SUBMIT_CONTRACT(terminal_id, contract_id)`
- `CLAIM_OWED(terminal_id)`

## 8. Error Codes (MVP)

- `E_NO_PERMISSION`
- `E_NO_RESOURCE`
- `E_INVALID_TARGET`
- `E_BLOCKED`
- `E_RATE_LIMIT`
- `E_CONFLICT`
- `E_UNSAFE`
- `E_STALE`
- `E_BAD_REQUEST`
- `E_INTERNAL`

## 9. Rate Limits (defaults)

在 `configs/tuning.yaml` 中可调。

当触发 `E_RATE_LIMIT` 时，服务端会在 `ACTION_RESULT` event 中附带冷却信息：
- `cooldown_ticks`: 距离下一次窗口重置还剩多少 ticks
- `cooldown_until_tick`: 下一次窗口重置的 tick（client 可据此推算等待时间）
