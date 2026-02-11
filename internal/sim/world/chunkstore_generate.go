package world

func (s *ChunkStore) generateChunk(ch *Chunk) {
	for z := 0; z < 16; z++ {
		for x := 0; x < 16; x++ {
			wx := ch.CX*16 + x
			wz := ch.CZ*16 + z

			b := s.gen.Air

			// Guarantee an open "spawn clearing" around the origin so agents can
			// build and navigate reliably without needing to mine the spawn area.
			if !withinSpawnClear(wx, wz, s.gen.SpawnClearRadius) {
				biome := biomeAt(s.gen.Seed, wx, wz, s.gen.BiomeRegionSize)

				// Precedence order: rare ores > common ores > biome terrain.
				switch {
				case inCluster(s.gen.Seed+101, wx, wz, 192, 2, scalePermille(200, s.gen.OreClusterProbScalePermille)): // ~0.008%
					b = s.gen.CrystalOre
				case inCluster(s.gen.Seed+102, wx, wz, 128, 3, scalePermille(450, s.gen.OreClusterProbScalePermille)): // ~0.15%
					b = s.gen.IronOre
				case inCluster(s.gen.Seed+103, wx, wz, 128, 3, scalePermille(450, s.gen.OreClusterProbScalePermille)): // ~0.15%
					b = s.gen.CopperOre
				case inCluster(s.gen.Seed+104, wx, wz, 64, 4, scalePermille(650, s.gen.OreClusterProbScalePermille)): // ~0.7%
					b = s.gen.CoalOre
				default:
					// Biome-flavored terrain clutter (kept low so the world stays navigable).
					switch biome {
					case "FOREST":
						switch {
						case inCluster(s.gen.Seed+201, wx, wz, 48, 4, scalePermille(450, s.gen.TerrainClusterProbScalePermille)):
							b = s.gen.Log
						case inCluster(s.gen.Seed+202, wx, wz, 32, 4, scalePermille(500, s.gen.TerrainClusterProbScalePermille)):
							b = s.gen.Stone
						case inCluster(s.gen.Seed+203, wx, wz, 48, 3, scalePermille(350, s.gen.TerrainClusterProbScalePermille)):
							b = s.gen.Dirt
						case inCluster(s.gen.Seed+204, wx, wz, 96, 2, scalePermille(180, s.gen.TerrainClusterProbScalePermille)):
							b = s.gen.Gravel
						default:
							b = s.gen.Air
						}
					case "DESERT":
						switch {
						case inCluster(s.gen.Seed+301, wx, wz, 48, 3, scalePermille(550, s.gen.TerrainClusterProbScalePermille)):
							b = s.gen.Sand
						case inCluster(s.gen.Seed+302, wx, wz, 32, 4, scalePermille(450, s.gen.TerrainClusterProbScalePermille)):
							b = s.gen.Stone
						case inCluster(s.gen.Seed+303, wx, wz, 96, 2, scalePermille(200, s.gen.TerrainClusterProbScalePermille)):
							b = s.gen.Gravel
						default:
							b = s.gen.Air
						}
					default: // PLAINS
						switch {
						case inCluster(s.gen.Seed+401, wx, wz, 48, 3, scalePermille(400, s.gen.TerrainClusterProbScalePermille)):
							b = s.gen.Dirt
						case inCluster(s.gen.Seed+402, wx, wz, 32, 4, scalePermille(500, s.gen.TerrainClusterProbScalePermille)):
							b = s.gen.Stone
						case inCluster(s.gen.Seed+403, wx, wz, 96, 2, scalePermille(180, s.gen.TerrainClusterProbScalePermille)):
							b = s.gen.Gravel
						default:
							b = s.gen.Air
						}
					}

					// Always sprinkle a small amount of terrain blocks so the world isn't
					// an endless void of AIR when clusters don't land nearby.
					if b == s.gen.Air {
						roll := hash2(s.gen.Seed+999, wx, wz) % 1000
						switch {
						case roll < uint64(clampPermille(s.gen.SprinkleStonePermille)):
							b = s.gen.Stone
						case roll < uint64(clampPermille(s.gen.SprinkleStonePermille))+uint64(clampPermille(s.gen.SprinkleDirtPermille)):
							if biome == "DESERT" {
								b = s.gen.Sand
							} else {
								b = s.gen.Dirt
							}
						case roll < uint64(clampPermille(s.gen.SprinkleStonePermille))+uint64(clampPermille(s.gen.SprinkleDirtPermille))+uint64(clampPermille(s.gen.SprinkleLogPermille)) && biome == "FOREST":
							b = s.gen.Log
						}
					}
				}
			}

			ch.Blocks[ch.index(x, z)] = b
		}
	}
}
