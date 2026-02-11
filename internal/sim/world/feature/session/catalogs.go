package session

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
)

func NormalizeAgentName(name string) string {
	v := strings.TrimSpace(name)
	if v == "" {
		return "agent"
	}
	return v
}

func OrderedCatalogs(
	blockPalette protocol.CatalogMsg,
	itemPalette protocol.CatalogMsg,
	tuning protocol.CatalogMsg,
	recipes protocol.CatalogMsg,
	blueprints protocol.CatalogMsg,
	laws protocol.CatalogMsg,
	events protocol.CatalogMsg,
) []protocol.CatalogMsg {
	return []protocol.CatalogMsg{
		blockPalette,
		itemPalette,
		tuning,
		recipes,
		blueprints,
		laws,
		events,
	}
}
