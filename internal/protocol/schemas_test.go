package protocol_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func TestSchemas_ValidateSamples(t *testing.T) {
	compile := func(name string) *jsonschema.Schema {
		t.Helper()
		p := filepath.Join("..", "..", "schemas", name)
		s, err := jsonschema.Compile(p)
		if err != nil {
			t.Fatalf("compile %s: %v", name, err)
		}
		return s
	}

	validate := func(s *jsonschema.Schema, v any) {
		t.Helper()
		if err := s.Validate(v); err != nil {
			t.Fatalf("validate: %v", err)
		}
	}

	helloSchema := compile("hello.schema.json")
	welcomeSchema := compile("welcome.schema.json")
	obsSchema := compile("obs.schema.json")
	actSchema := compile("act.schema.json")
	hello11Schema := compile("hello_v1_1.schema.json")
	welcome11Schema := compile("welcome_v1_1.schema.json")
	obs11Schema := compile("obs_v1_1.schema.json")
	act11Schema := compile("act_v1_1.schema.json")
	ackSchema := compile("ack.schema.json")
	eventBatchSchema := compile("event_batch.schema.json")
	eventBatchReqSchema := compile("event_batch_req.schema.json")
	eventBatchMsgSchema := compile("event_batch_msg.schema.json")

	var hello any
	_ = json.Unmarshal([]byte(`{
	  "type":"HELLO",
	  "protocol_version":"1.0",
	  "agent_name":"bot1",
	  "capabilities":{"delta_voxels":true,"max_queue":8}
	}`), &hello)
	validate(helloSchema, hello)

	var welcome any
	_ = json.Unmarshal([]byte(`{
	  "type":"WELCOME",
	  "protocol_version":"1.0",
	  "agent_id":"A1",
	  "resume_token":"resume_world_1_123",
	  "world_params":{
	    "tick_rate_hz":5,
	    "chunk_size":[16,16,1],
	    "height":1,
	    "obs_radius":7,
	    "day_ticks":6000,
	    "seed":1337
	  },
	  "catalogs":{
	    "block_palette":{"digest":"deadbeef","count":28},
	    "item_palette":{"digest":"deadbeef","count":40},
	    "recipes_digest":"deadbeef",
	    "blueprints_digest":"deadbeef",
	    "law_templates_digest":"deadbeef",
	    "events_digest":"deadbeef"
	  }
	}`), &welcome)
	validate(welcomeSchema, welcome)

	var obs any
	_ = json.Unmarshal([]byte(`{
	  "type":"OBS",
	  "protocol_version":"1.0",
	  "tick":0,
	  "agent_id":"A1",
	  "world_id":"OVERWORLD",
	  "world_clock":0,
	  "world":{"time_of_day":0.0,"weather":"CLEAR","season_day":1,"biome":"PLAINS"},
	  "self":{"pos":[0,0,0],"yaw":0,"hp":20,"hunger":20,"stamina":1.0,"status":["NONE"]},
	  "inventory":[],
	  "equipment":{"main_hand":"NONE","armor":["NONE","NONE","NONE","NONE"]},
	  "local_rules":{"permissions":{"can_build":true,"can_break":true,"can_damage":false},"tax":{"market":0.0}},
	  "voxels":{"center":[0,0,0],"radius":7,"encoding":"RLE","data":"AA=="},
	  "entities":[],
	  "events":[],
	  "tasks":[]
	}`), &obs)
	validate(obsSchema, obs)

	var act any
	_ = json.Unmarshal([]byte(`{
	  "type":"ACT",
	  "protocol_version":"1.0",
	  "tick":0,
	  "agent_id":"A1",
	  "instants":[{"id":"I1","type":"SAY","channel":"LOCAL","text":"hi"}],
	  "tasks":[{"id":"K1","type":"MOVE_TO","target":[1,0,1],"tolerance":1.2}],
	  "cancel":[]
	}`), &act)
	validate(actSchema, act)

	var hello11 any
	_ = json.Unmarshal([]byte(`{
	  "type":"HELLO",
	  "protocol_version":"1.1",
	  "supported_versions":["1.1","1.0"],
	  "agent_name":"bot11",
	  "capabilities":{"delta_voxels":true,"max_queue":8},
	  "client_capabilities":{"ack_required":true,"event_cursor":true}
	}`), &hello11)
	validate(hello11Schema, hello11)

	var welcome11 any
	_ = json.Unmarshal([]byte(`{
	  "type":"WELCOME",
	  "protocol_version":"1.1",
	  "selected_version":"1.1",
	  "session_id":"sess_1",
	  "server_capabilities":{"ack":true,"event_batch":true,"idempotency":true},
	  "agent_id":"A1",
	  "resume_token":"resume_world_1_123",
	  "world_params":{"tick_rate_hz":5,"chunk_size":[16,16,1],"height":1,"obs_radius":7,"day_ticks":6000,"seed":1337},
	  "catalogs":{"block_palette":{"digest":"deadbeef","count":28},"item_palette":{"digest":"deadbeef","count":40},"recipes_digest":"deadbeef","blueprints_digest":"deadbeef","law_templates_digest":"deadbeef","events_digest":"deadbeef"}
	}`), &welcome11)
	validate(welcome11Schema, welcome11)

	var obs11 any
	_ = json.Unmarshal([]byte(`{
	  "type":"OBS",
	  "protocol_version":"1.1",
	  "tick":1,
	  "agent_id":"A1",
	  "obs_id":"A1:1:0",
	  "events_cursor":0,
	  "world":{"time_of_day":0.0,"weather":"CLEAR","season_day":1,"biome":"PLAINS"},
	  "self":{"pos":[0,0,0],"yaw":0,"hp":20,"hunger":20,"stamina":1.0,"status":["NONE"]},
	  "inventory":[],
	  "equipment":{"main_hand":"NONE","armor":["NONE","NONE","NONE","NONE"]},
	  "local_rules":{"permissions":{"can_build":true,"can_break":true,"can_damage":false},"tax":{"market":0.0}},
	  "voxels":{"center":[0,0,0],"radius":7,"encoding":"RLE","data":"AA=="},
	  "entities":[],
	  "events":[],
	  "tasks":[]
	}`), &obs11)
	validate(obs11Schema, obs11)

	var act11 any
	_ = json.Unmarshal([]byte(`{
	  "type":"ACT",
	  "protocol_version":"1.1",
	  "act_id":"ACT_1",
	  "based_on_obs_id":"A1:1:0",
	  "idempotency_key":"ACT_1",
	  "tick":1,
	  "agent_id":"A1",
	  "expected_world_id":"OVERWORLD",
	  "instants":[{"id":"I1","type":"SAY","channel":"LOCAL","text":"hi"}]
	}`), &act11)
	validate(act11Schema, act11)

	var ack any
	_ = json.Unmarshal([]byte(`{
	  "type":"ACK",
	  "protocol_version":"1.1",
	  "ack_for":"ACT_1",
	  "accepted":true,
	  "server_tick":1,
	  "world_id":"OVERWORLD"
	}`), &ack)
	validate(ackSchema, ack)

	var eventBatch any
	_ = json.Unmarshal([]byte(`{"events":[{"cursor":1,"event":{"type":"ACTION_RESULT","ok":true}}],"next_cursor":1}`), &eventBatch)
	validate(eventBatchSchema, eventBatch)

	var eventBatchReq any
	_ = json.Unmarshal([]byte(`{
	  "type":"EVENT_BATCH_REQ",
	  "protocol_version":"1.1",
	  "req_id":"req_1",
	  "since_cursor":0,
	  "limit":100
	}`), &eventBatchReq)
	validate(eventBatchReqSchema, eventBatchReq)

	var eventBatchMsg any
	_ = json.Unmarshal([]byte(`{
	  "type":"EVENT_BATCH",
	  "protocol_version":"1.1",
	  "req_id":"req_1",
	  "events":[{"cursor":1,"event":{"type":"ACTION_RESULT","ok":true}}],
	  "next_cursor":1,
	  "world_id":"OVERWORLD"
	}`), &eventBatchMsg)
	validate(eventBatchMsgSchema, eventBatchMsg)
}
