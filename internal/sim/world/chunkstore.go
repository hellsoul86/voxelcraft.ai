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

			// Pure 2D generation: each (x,z) cell picks a single block.
			roll := hash2(s.gen.Seed, wx, wz) % 1000
			b := s.gen.Air
			switch {
			case roll < 10:
				b = s.gen.CrystalOre
			case roll < 30:
				b = s.gen.IronOre
			case roll < 60:
				b = s.gen.CopperOre
			case roll < 100:
				b = s.gen.CoalOre
			case roll < 180:
				b = s.gen.Stone
			case roll < 240:
				b = s.gen.Log
			case roll < 300:
				if biomeFrom(hash2(s.gen.Seed, wx, wz)) == "DESERT" {
					b = s.gen.Sand
				} else {
					b = s.gen.Dirt
				}
			case roll < 330:
				b = s.gen.Sand
			case roll < 350:
				b = s.gen.Gravel
			default:
				b = s.gen.Air
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
