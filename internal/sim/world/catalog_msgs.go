package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func (w *World) recipesCatalogMsg() protocol.CatalogMsg {
	ids := make([]string, 0, len(w.catalogs.Recipes.ByID))
	for id := range w.catalogs.Recipes.ByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	defs := make([]catalogs.RecipeDef, 0, len(ids))
	for _, id := range ids {
		defs = append(defs, w.catalogs.Recipes.ByID[id])
	}
	return protocol.CatalogMsg{
		Type:            protocol.TypeCatalog,
		ProtocolVersion: protocol.Version,
		Name:            "recipes",
		Digest:          w.catalogs.Recipes.Digest,
		Part:            1,
		TotalParts:      1,
		Data:            defs,
	}
}

func (w *World) blueprintsCatalogMsg() protocol.CatalogMsg {
	ids := make([]string, 0, len(w.catalogs.Blueprints.ByID))
	for id := range w.catalogs.Blueprints.ByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	defs := make([]catalogs.BlueprintDef, 0, len(ids))
	for _, id := range ids {
		defs = append(defs, w.catalogs.Blueprints.ByID[id])
	}
	return protocol.CatalogMsg{
		Type:            protocol.TypeCatalog,
		ProtocolVersion: protocol.Version,
		Name:            "blueprints",
		Digest:          w.catalogs.Blueprints.Digest,
		Part:            1,
		TotalParts:      1,
		Data:            defs,
	}
}

type lawTemplatesCatalogData struct {
	Templates []catalogs.LawTemplate `json:"templates"`
}

func (w *World) lawTemplatesCatalogMsg() protocol.CatalogMsg {
	tpls := make([]catalogs.LawTemplate, 0, len(w.catalogs.Laws.Templates))
	tpls = append(tpls, w.catalogs.Laws.Templates...)
	sort.Slice(tpls, func(i, j int) bool { return tpls[i].ID < tpls[j].ID })
	return protocol.CatalogMsg{
		Type:            protocol.TypeCatalog,
		ProtocolVersion: protocol.Version,
		Name:            "law_templates",
		Digest:          w.catalogs.Laws.Digest,
		Part:            1,
		TotalParts:      1,
		Data:            lawTemplatesCatalogData{Templates: tpls},
	}
}

func (w *World) eventsCatalogMsg() protocol.CatalogMsg {
	ids := make([]string, 0, len(w.catalogs.Events.ByID))
	for id := range w.catalogs.Events.ByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	defs := make([]catalogs.EventTemplate, 0, len(ids))
	for _, id := range ids {
		defs = append(defs, w.catalogs.Events.ByID[id])
	}
	return protocol.CatalogMsg{
		Type:            protocol.TypeCatalog,
		ProtocolVersion: protocol.Version,
		Name:            "events",
		Digest:          w.catalogs.Events.Digest,
		Part:            1,
		TotalParts:      1,
		Data:            defs,
	}
}

