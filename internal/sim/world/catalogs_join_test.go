package world

import (
	"reflect"
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestJoin_CatalogsIncludeFullSet(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 1}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "bot", DeltaVoxels: false, Out: nil, Resp: resp})
	jr := <-resp

	if got, want := len(jr.Catalogs), 7; got != want {
		t.Fatalf("catalog count: got %d want %d", got, want)
	}

	names := make([]string, 0, len(jr.Catalogs))
	for _, c := range jr.Catalogs {
		names = append(names, c.Name)
		if c.Type != protocol.TypeCatalog {
			t.Fatalf("catalog %q type=%q want %q", c.Name, c.Type, protocol.TypeCatalog)
		}
		if c.ProtocolVersion != protocol.Version {
			t.Fatalf("catalog %q protocol_version=%q want %q", c.Name, c.ProtocolVersion, protocol.Version)
		}
		if c.Part != 1 || c.TotalParts != 1 {
			t.Fatalf("catalog %q part=%d total=%d want 1/1", c.Name, c.Part, c.TotalParts)
		}
		if c.Digest == "" {
			t.Fatalf("catalog %q missing digest", c.Name)
		}
		if c.Data == nil {
			t.Fatalf("catalog %q missing data", c.Name)
		}
	}
	wantNames := []string{"block_palette", "item_palette", "tuning", "recipes", "blueprints", "law_templates", "events"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("catalog names/order: got %v want %v", names, wantNames)
	}

	// Validate expected digests.
	if jr.Catalogs[0].Digest != w.catalogs.Blocks.PaletteDigest {
		t.Fatalf("block_palette digest mismatch")
	}
	if jr.Catalogs[1].Digest != w.catalogs.Items.PaletteDigest {
		t.Fatalf("item_palette digest mismatch")
	}
	if jr.Catalogs[3].Digest != w.catalogs.Recipes.Digest {
		t.Fatalf("recipes digest mismatch")
	}
	if jr.Catalogs[4].Digest != w.catalogs.Blueprints.Digest {
		t.Fatalf("blueprints digest mismatch")
	}
	if jr.Catalogs[5].Digest != w.catalogs.Laws.Digest {
		t.Fatalf("law_templates digest mismatch")
	}
	if jr.Catalogs[6].Digest != w.catalogs.Events.Digest {
		t.Fatalf("events digest mismatch")
	}

	// Data shapes + determinism ordering.
	if pal, ok := jr.Catalogs[0].Data.([]string); !ok || len(pal) == 0 {
		t.Fatalf("block_palette data type/len unexpected: %T", jr.Catalogs[0].Data)
	}
	if pal, ok := jr.Catalogs[1].Data.([]string); !ok || len(pal) == 0 {
		t.Fatalf("item_palette data type/len unexpected: %T", jr.Catalogs[1].Data)
	}
	if tc, ok := jr.Catalogs[2].Data.(tuningCatalog); !ok || tc.SnapshotEveryTicks <= 0 {
		t.Fatalf("tuning data type/fields unexpected: %T", jr.Catalogs[2].Data)
	}
	if defs, ok := jr.Catalogs[3].Data.([]catalogs.RecipeDef); !ok || len(defs) == 0 {
		t.Fatalf("recipes data type/len unexpected: %T", jr.Catalogs[3].Data)
	} else {
		for i := 1; i < len(defs); i++ {
			if defs[i-1].RecipeID > defs[i].RecipeID {
				t.Fatalf("recipes not sorted by recipe_id")
			}
		}
	}
	if defs, ok := jr.Catalogs[4].Data.([]catalogs.BlueprintDef); !ok || len(defs) == 0 {
		t.Fatalf("blueprints data type/len unexpected: %T", jr.Catalogs[4].Data)
	} else {
		for i := 1; i < len(defs); i++ {
			if defs[i-1].ID > defs[i].ID {
				t.Fatalf("blueprints not sorted by id")
			}
		}
	}
	if lc, ok := jr.Catalogs[5].Data.(lawTemplatesCatalogData); !ok || len(lc.Templates) == 0 {
		t.Fatalf("law_templates data type/len unexpected: %T", jr.Catalogs[5].Data)
	} else {
		for i := 1; i < len(lc.Templates); i++ {
			if lc.Templates[i-1].ID > lc.Templates[i].ID {
				t.Fatalf("law_templates not sorted by id")
			}
		}
	}
	if defs, ok := jr.Catalogs[6].Data.([]catalogs.EventTemplate); !ok || len(defs) == 0 {
		t.Fatalf("events data type/len unexpected: %T", jr.Catalogs[6].Data)
	} else {
		for i := 1; i < len(defs); i++ {
			if defs[i-1].ID > defs[i].ID {
				t.Fatalf("events not sorted by id")
			}
		}
	}
}

