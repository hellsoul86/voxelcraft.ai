package world

import digestfeaturepkg "voxelcraft.ai/internal/sim/world/feature/persistence/digest"

func (w *World) stateDigest(nowTick uint64) string {
	keys := w.chunks.LoadedChunkKeys()
	return digestfeaturepkg.StateDigest(digestfeaturepkg.StateInput{
		NowTick: nowTick,
		Seed:    w.cfg.Seed,

		Weather:          w.weather,
		WeatherUntilTick: w.weatherUntilTick,

		ActiveEventID:     w.activeEventID,
		ActiveEventStart:  w.activeEventStart,
		ActiveEventEnds:   w.activeEventEnds,
		ActiveEventCenter: w.activeEventCenter,
		ActiveEventRadius: w.activeEventRadius,

		ChunkKeys: keys,
		ChunkDigest: func(k ChunkKey) [32]byte {
			ch := w.chunks.chunks[k]
			if ch == nil {
				return [32]byte{}
			}
			return ch.Digest()
		},

		Claims:     w.claims,
		Laws:       w.laws,
		Orgs:       w.orgs,
		Containers: w.containers,
		Items:      w.items,
		Signs:      w.signs,
		Conveyors:  w.conveyors,
		Switches:   w.switches,
		Contracts:  w.contracts,
		Trades:     w.trades,
		Boards:     w.boards,
		Structures: w.structures,
		Agents:     w.agents,
	})
}
