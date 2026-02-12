package store

import (
	"crypto/sha256"
	"encoding/binary"
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

	BiomeRegionSize                 int
	SpawnClearRadius                int
	OreClusterProbScalePermille     int
	TerrainClusterProbScalePermille int
	SprinkleStonePermille           int
	SprinkleDirtPermille            int
	SprinkleLogPermille             int

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
	Gen    WorldGen
	Chunks map[ChunkKey]*Chunk
}

func NewChunkStore(gen WorldGen) *ChunkStore {
	return &ChunkStore{
		Gen:    gen,
		Chunks: map[ChunkKey]*Chunk{},
	}
}
