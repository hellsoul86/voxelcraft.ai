package store

import genpkg "voxelcraft.ai/internal/sim/world/terrain/gen"

func (s *ChunkStore) GenerateChunk(ch *Chunk) {
	for z := 0; z < 16; z++ {
		for x := 0; x < 16; x++ {
			wx := ch.CX*16 + x
			wz := ch.CZ*16 + z

			b := s.Gen.Air
			if !genpkg.WithinSpawnClear(wx, wz, s.Gen.SpawnClearRadius) {
				biome := genpkg.BiomeAt(s.Gen.Seed, wx, wz, s.Gen.BiomeRegionSize)
				switch {
				case genpkg.InCluster(s.Gen.Seed+101, wx, wz, 192, 2, genpkg.ScalePermille(200, s.Gen.OreClusterProbScalePermille)):
					b = s.Gen.CrystalOre
				case genpkg.InCluster(s.Gen.Seed+102, wx, wz, 128, 3, genpkg.ScalePermille(450, s.Gen.OreClusterProbScalePermille)):
					b = s.Gen.IronOre
				case genpkg.InCluster(s.Gen.Seed+103, wx, wz, 128, 3, genpkg.ScalePermille(450, s.Gen.OreClusterProbScalePermille)):
					b = s.Gen.CopperOre
				case genpkg.InCluster(s.Gen.Seed+104, wx, wz, 64, 4, genpkg.ScalePermille(650, s.Gen.OreClusterProbScalePermille)):
					b = s.Gen.CoalOre
				default:
					switch biome {
					case "FOREST":
						switch {
						case genpkg.InCluster(s.Gen.Seed+201, wx, wz, 48, 4, genpkg.ScalePermille(450, s.Gen.TerrainClusterProbScalePermille)):
							b = s.Gen.Log
						case genpkg.InCluster(s.Gen.Seed+202, wx, wz, 32, 4, genpkg.ScalePermille(500, s.Gen.TerrainClusterProbScalePermille)):
							b = s.Gen.Stone
						case genpkg.InCluster(s.Gen.Seed+203, wx, wz, 48, 3, genpkg.ScalePermille(350, s.Gen.TerrainClusterProbScalePermille)):
							b = s.Gen.Dirt
						case genpkg.InCluster(s.Gen.Seed+204, wx, wz, 96, 2, genpkg.ScalePermille(180, s.Gen.TerrainClusterProbScalePermille)):
							b = s.Gen.Gravel
						default:
							b = s.Gen.Air
						}
					case "DESERT":
						switch {
						case genpkg.InCluster(s.Gen.Seed+301, wx, wz, 48, 3, genpkg.ScalePermille(550, s.Gen.TerrainClusterProbScalePermille)):
							b = s.Gen.Sand
						case genpkg.InCluster(s.Gen.Seed+302, wx, wz, 32, 4, genpkg.ScalePermille(450, s.Gen.TerrainClusterProbScalePermille)):
							b = s.Gen.Stone
						case genpkg.InCluster(s.Gen.Seed+303, wx, wz, 96, 2, genpkg.ScalePermille(200, s.Gen.TerrainClusterProbScalePermille)):
							b = s.Gen.Gravel
						default:
							b = s.Gen.Air
						}
					default:
						switch {
						case genpkg.InCluster(s.Gen.Seed+401, wx, wz, 48, 3, genpkg.ScalePermille(400, s.Gen.TerrainClusterProbScalePermille)):
							b = s.Gen.Dirt
						case genpkg.InCluster(s.Gen.Seed+402, wx, wz, 32, 4, genpkg.ScalePermille(500, s.Gen.TerrainClusterProbScalePermille)):
							b = s.Gen.Stone
						case genpkg.InCluster(s.Gen.Seed+403, wx, wz, 96, 2, genpkg.ScalePermille(180, s.Gen.TerrainClusterProbScalePermille)):
							b = s.Gen.Gravel
						default:
							b = s.Gen.Air
						}
					}
					if b == s.Gen.Air {
						roll := genpkg.Hash2(s.Gen.Seed+999, wx, wz) % 1000
						switch {
						case roll < uint64(genpkg.ClampPermille(s.Gen.SprinkleStonePermille)):
							b = s.Gen.Stone
						case roll < uint64(genpkg.ClampPermille(s.Gen.SprinkleStonePermille))+uint64(genpkg.ClampPermille(s.Gen.SprinkleDirtPermille)):
							if biome == "DESERT" {
								b = s.Gen.Sand
							} else {
								b = s.Gen.Dirt
							}
						case roll < uint64(genpkg.ClampPermille(s.Gen.SprinkleStonePermille))+uint64(genpkg.ClampPermille(s.Gen.SprinkleDirtPermille))+uint64(genpkg.ClampPermille(s.Gen.SprinkleLogPermille)) && biome == "FOREST":
							b = s.Gen.Log
						}
					}
				}
			}
			ch.Blocks[x+z*16] = b
		}
	}
}
