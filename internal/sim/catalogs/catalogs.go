package catalogs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Catalogs struct {
	Blocks BlockCatalog
	Items  ItemCatalog

	Recipes    RecipeCatalog
	Blueprints BlueprintCatalog
	Laws       LawCatalog
	Events     EventCatalog
}

type BlockCatalog struct {
	Palette       []string
	Index         map[string]uint16
	Defs          map[string]BlockDef
	PaletteDigest string
	DefsDigest    string
}

type BlockDef struct {
	ID        string `json:"id"`
	Solid     bool   `json:"solid"`
	Breakable bool   `json:"breakable"`
	DropsItem string `json:"drops_item,omitempty"`
}

type ItemCatalog struct {
	Palette       []string
	Index         map[string]uint16
	Defs          map[string]ItemDef
	PaletteDigest string
	DefsDigest    string
}

type ItemDef struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"` // "BLOCK","TOOL","MATERIAL","FOOD","MECH"
	PlaceAs  string `json:"place_as,omitempty"`
	EdibleHP int    `json:"edible_hp,omitempty"`
}

type RecipeCatalog struct {
	ByID   map[string]RecipeDef
	Digest string
}

type RecipeDef struct {
	RecipeID  string      `json:"recipe_id"`
	Station   string      `json:"station"`
	Inputs    []ItemCount `json:"inputs"`
	Outputs   []ItemCount `json:"outputs"`
	Tier      int         `json:"tier"`
	TimeTicks int         `json:"time_ticks"`
}

type ItemCount struct {
	Item  string `json:"item"`
	Count int    `json:"count"`
}

type BlueprintCatalog struct {
	ByID   map[string]BlueprintDef
	Digest string
}

type BlueprintDef struct {
	ID      string      `json:"id"`
	Author  string      `json:"author"`
	Version string      `json:"version"`
	AABB    [2][3]int   `json:"aabb"`
	Blocks  []BPBlock   `json:"blocks"`
	Cost    []ItemCount `json:"cost"`
}

type BPBlock struct {
	Pos   [3]int `json:"pos"`
	Block string `json:"block"`
}

type LawCatalog struct {
	Templates []LawTemplate `json:"templates"`
	ByID      map[string]LawTemplate
	Digest    string
}

type LawTemplate struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Params      map[string]string `json:"params"`
}

type EventCatalog struct {
	ByID   map[string]EventTemplate
	Digest string
}

type EventTemplate struct {
	ID          string         `json:"id"`
	Category    string         `json:"category"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	BaseWeight  float64        `json:"base_weight"`
	Params      map[string]any `json:"params,omitempty"`
}

func Load(configDir string) (*Catalogs, error) {
	var c Catalogs

	if err := loadBlocks(filepath.Join(configDir, "blocks.json"), &c.Blocks); err != nil {
		return nil, err
	}
	if err := loadItems(filepath.Join(configDir, "items.json"), &c.Items); err != nil {
		return nil, err
	}
	if err := loadRecipes(filepath.Join(configDir, "recipes.json"), &c.Recipes); err != nil {
		return nil, err
	}
	if err := loadBlueprints(filepath.Join(configDir, "blueprints"), &c.Blueprints); err != nil {
		return nil, err
	}
	if err := loadLaws(filepath.Join(configDir, "law_templates.json"), &c.Laws); err != nil {
		return nil, err
	}
	if err := loadEvents(filepath.Join(configDir, "events"), &c.Events); err != nil {
		return nil, err
	}

	return &c, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func loadBlocks(path string, out *BlockCatalog) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out.DefsDigest = sha256Hex(raw)

	var defs []BlockDef
	if err := json.Unmarshal(raw, &defs); err != nil {
		return fmt.Errorf("blocks.json: %w", err)
	}
	out.Defs = map[string]BlockDef{}
	for _, d := range defs {
		if d.ID == "" {
			return fmt.Errorf("blocks.json: empty id")
		}
		out.Defs[d.ID] = d
	}

	ids := make([]string, 0, len(out.Defs))
	for id := range out.Defs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Ensure AIR exists and is palette id 0.
	if _, ok := out.Defs["AIR"]; !ok {
		return fmt.Errorf("blocks.json: missing AIR")
	}
	ids = append([]string{"AIR"}, filterOut(ids, "AIR")...)

	out.Palette = ids
	out.Index = make(map[string]uint16, len(ids))
	for i, id := range ids {
		out.Index[id] = uint16(i)
	}
	palJSON, _ := json.Marshal(ids)
	out.PaletteDigest = sha256Hex(palJSON)
	return nil
}

func loadItems(path string, out *ItemCatalog) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out.DefsDigest = sha256Hex(raw)

	var defs []ItemDef
	if err := json.Unmarshal(raw, &defs); err != nil {
		return fmt.Errorf("items.json: %w", err)
	}
	out.Defs = map[string]ItemDef{}
	for _, d := range defs {
		if d.ID == "" {
			return fmt.Errorf("items.json: empty id")
		}
		out.Defs[d.ID] = d
	}

	ids := make([]string, 0, len(out.Defs))
	for id := range out.Defs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out.Palette = ids
	out.Index = make(map[string]uint16, len(ids))
	for i, id := range ids {
		out.Index[id] = uint16(i)
	}
	palJSON, _ := json.Marshal(ids)
	out.PaletteDigest = sha256Hex(palJSON)
	return nil
}

func loadRecipes(path string, out *RecipeCatalog) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out.Digest = sha256Hex(raw)

	var defs []RecipeDef
	if err := json.Unmarshal(raw, &defs); err != nil {
		return fmt.Errorf("recipes.json: %w", err)
	}
	out.ByID = map[string]RecipeDef{}
	for _, r := range defs {
		if r.RecipeID == "" {
			return fmt.Errorf("recipes.json: empty recipe_id")
		}
		out.ByID[r.RecipeID] = r
	}
	return nil
}

func loadBlueprints(dir string, out *BlueprintCatalog) error {
	out.ByID = map[string]BlueprintDef{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		// For MVP, allow no blueprints directory.
		if os.IsNotExist(err) {
			out.Digest = sha256Hex(nil)
			return nil
		}
		return err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".json") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)

	var concat bytes.Buffer
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		concat.Write(b)
		concat.WriteByte('\n')

		var bp BlueprintDef
		if err := json.Unmarshal(b, &bp); err != nil {
			return fmt.Errorf("blueprint %s: %w", filepath.Base(p), err)
		}
		if bp.ID == "" {
			return fmt.Errorf("blueprint %s: missing id", filepath.Base(p))
		}
		out.ByID[bp.ID] = bp
	}
	out.Digest = sha256Hex(concat.Bytes())
	return nil
}

func loadLaws(path string, out *LawCatalog) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		// Allow missing for now.
		if os.IsNotExist(err) {
			out.Digest = sha256Hex(nil)
			out.ByID = map[string]LawTemplate{}
			return nil
		}
		return err
	}
	out.Digest = sha256Hex(raw)
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("law_templates.json: %w", err)
	}
	out.ByID = map[string]LawTemplate{}
	for _, t := range out.Templates {
		out.ByID[t.ID] = t
	}
	return nil
}

func loadEvents(dir string, out *EventCatalog) error {
	out.ByID = map[string]EventTemplate{}

	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".json") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			out.Digest = sha256Hex(nil)
			return nil
		}
		// WalkDir returns nil even if root missing; handle separately.
		if _, statErr := os.Stat(dir); statErr != nil && os.IsNotExist(statErr) {
			out.Digest = sha256Hex(nil)
			return nil
		}
		return err
	}
	sort.Strings(files)

	var concat bytes.Buffer
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		concat.Write(b)
		concat.WriteByte('\n')

		var ev EventTemplate
		if err := json.Unmarshal(b, &ev); err != nil {
			return fmt.Errorf("event %s: %w", filepath.Base(p), err)
		}
		if ev.ID == "" {
			return fmt.Errorf("event %s: missing id", filepath.Base(p))
		}
		out.ByID[ev.ID] = ev
	}
	out.Digest = sha256Hex(concat.Bytes())
	return nil
}

func filterOut(in []string, remove string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == remove {
			continue
		}
		out = append(out, s)
	}
	return out
}
