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
}
