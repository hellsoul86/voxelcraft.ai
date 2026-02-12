package trade

import (
	"fmt"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type Trade = modelpkg.Trade

func TradeID(n uint64) string {
	return fmt.Sprintf("TR%06d", n)
}
