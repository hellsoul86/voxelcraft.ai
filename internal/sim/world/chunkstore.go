package world

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
)

type ChunkKey struct {
	CX int
	CZ int
}

type Chunk struct {
	CX, CZ int
	Blocks []uint16 // len = 16*16 (pure 2D world)

	dirty bool
	hash  [32]byte
}

func (c *Chunk) index(x, z int) int {
	// x fastest, then z
	return x + z*16
}

func (c *Chunk) Get(x, z int) uint16 {
	return c.Blocks[c.index(x, z)]
}

func (c *Chunk) Set(x, z int, b uint16) {
	i := c.index(x, z)
	if c.Blocks[i] == b {
		return
	}
	c.Blocks[i] = b
	c.dirty = true
}

func (c *Chunk) Digest() [32]byte {
	if c.dirty || c.hash == ([32]byte{}) {
		// Hash the raw uint16 slice deterministically.
		h := sha256.New()
		var tmp [2]byte
		for _, v := range c.Blocks {
			binary.LittleEndian.PutUint16(tmp[:], v)
			h.Write(tmp[:])
		}
		copy(c.hash[:], h.Sum(nil))
		c.dirty = false
	}
	return c.hash
}

type WorldGen struct {
	Seed      int64
	BoundaryR int // blocks

	// Worldgen tuning.
	BiomeRegionSize                 int
	SpawnClearRadius                int
	OreClusterProbScalePermille     int
	TerrainClusterProbScalePermille int
	SprinkleStonePermille           int
	SprinkleDirtPermille            int
	SprinkleLogPermille             int

	// Palette ids for core blocks.
	Air        uint16
	Dirt       uint16
	Grass      uint16
	Sand       uint16
	Stone      uint16
	Gravel     uint16
	Log        uint16
	CoalOre    uint16
	IronOre    uint16
	CopperOre  uint16
	CrystalOre uint16
}

type ChunkStore struct {
	gen WorldGen
	// Accessed only from the world loop goroutine.
	chunks map[ChunkKey]*Chunk
}

func NewChunkStore(gen WorldGen) *ChunkStore {
	return &ChunkStore{
		gen:    gen,
		chunks: map[ChunkKey]*Chunk{},
	}
}

func (s *ChunkStore) inBounds(pos Vec3i) bool {
	if pos.Y != 0 {
		return false
	}
	if s.gen.BoundaryR > 0 {
		if pos.X < -s.gen.BoundaryR || pos.X > s.gen.BoundaryR || pos.Z < -s.gen.BoundaryR || pos.Z > s.gen.BoundaryR {
			return false
		}
	}
	return true
}

func (s *ChunkStore) LoadedChunkKeys() []ChunkKey {
	keys := make([]ChunkKey, 0, len(s.chunks))
	for k := range s.chunks {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].CX != keys[j].CX {
			return keys[i].CX < keys[j].CX
		}
		return keys[i].CZ < keys[j].CZ
	})
	return keys
}

func (s *ChunkStore) GetBlock(pos Vec3i) uint16 {
	if pos.Y != 0 {
		return s.gen.Air
	}
	if !s.inBounds(pos) {
		return s.gen.Air
	}

	cx := floorDiv(pos.X, 16)
	cz := floorDiv(pos.Z, 16)
	lx := mod(pos.X, 16)
	lz := mod(pos.Z, 16)
	ch := s.getOrGenChunk(cx, cz)
	return ch.Get(lx, lz)
}

func (s *ChunkStore) SetBlock(pos Vec3i, b uint16) {
	if pos.Y != 0 {
		return
	}
	if !s.inBounds(pos) {
		return
	}
	cx := floorDiv(pos.X, 16)
	cz := floorDiv(pos.Z, 16)
	lx := mod(pos.X, 16)
	lz := mod(pos.Z, 16)
	ch := s.getOrGenChunk(cx, cz)
	ch.Set(lx, lz, b)
}

func (s *ChunkStore) getOrGenChunk(cx, cz int) *Chunk {
	k := ChunkKey{CX: cx, CZ: cz}
	if ch, ok := s.chunks[k]; ok {
		return ch
	}
	ch := &Chunk{
		CX:     cx,
		CZ:     cz,
		Blocks: make([]uint16, 16*16),
	}
	s.generateChunk(ch)
	ch.dirty = true
	_ = ch.Digest() // initialize digest
	s.chunks[k] = ch
	return ch
}

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

func floorDiv(a, b int) int {
	// b > 0
	q := a / b
	r := a % b
	if r < 0 {
		q--
	}
	return q
}

func mod(a, b int) int {
	// b > 0
	m := a % b
	if m < 0 {
		m += b
	}
	return m
}

func mix64(z uint64) uint64 {
	z += 0x9e3779b97f4a7c15
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func hash2(seed int64, x, z int) uint64 {
	ux := uint64(uint32(int32(x)))
	uz := uint64(uint32(int32(z)))
	v := uint64(seed) ^ (ux * 0x9e3779b97f4a7c15) ^ (uz * 0xbf58476d1ce4e5b9)
	return mix64(v)
}

func hash3(seed int64, x, y, z int) uint64 {
	ux := uint64(uint32(int32(x)))
	uy := uint64(uint32(int32(y)))
	uz := uint64(uint32(int32(z)))
	v := uint64(seed) ^ (ux * 0x9e3779b97f4a7c15) ^ (uy * 0xc2b2ae3d27d4eb4f) ^ (uz * 0xbf58476d1ce4e5b9)
	return mix64(v)
}

func biomeFrom(noise uint64) string {
	// 3-way split.
	switch noise % 3 {
	case 0:
		return "PLAINS"
	case 1:
		return "FOREST"
	default:
		return "DESERT"
	}
}

func biomeAt(seed int64, x, z, regionSize int) string {
	if regionSize <= 0 {
		regionSize = 1
	}
	rx := floorDiv(x, regionSize)
	rz := floorDiv(z, regionSize)
	return biomeFrom(hash2(seed, rx, rz))
}

func withinSpawnClear(x, z, radius int) bool {
	if radius <= 0 {
		return false
	}
	r := int64(radius)
	dx := int64(x)
	dz := int64(z)
	return dx*dx+dz*dz <= r*r
}

func clampPermille(v int) int {
	if v < 0 {
		return 0
	}
	if v > 1000 {
		return 1000
	}
	return v
}

func scalePermille(base uint64, scalePermille int) uint64 {
	if scalePermille <= 0 {
		scalePermille = 1000
	}
	// Nearest integer rounding: (base*scale + 500)/1000
	scaled := (base*uint64(scalePermille) + 500) / 1000
	if scaled > 1000 {
		return 1000
	}
	return scaled
}

func inCluster(seed int64, x, z, grid, radius int, probPermille uint64) bool {
	if grid <= 0 || radius <= 0 || probPermille == 0 {
		return false
	}
	gx := floorDiv(x, grid)
	gz := floorDiv(z, grid)
	r2 := radius * radius

	for dz := -1; dz <= 1; dz++ {
		for dx := -1; dx <= 1; dx++ {
			cgx := gx + dx
			cgz := gz + dz
			h := hash2(seed, cgx, cgz)
			if h%1000 >= probPermille {
				continue
			}

			// Deterministically place a center inside this grid cell.
			ox := int((h >> 10) % uint64(grid))
			oz := int((h >> 20) % uint64(grid))
			cx := cgx*grid + ox
			cz := cgz*grid + oz

			ddx := x - cx
			ddz := z - cz
			if ddx*ddx+ddz*ddz <= r2 {
				return true
			}
		}
	}
	return false
}
