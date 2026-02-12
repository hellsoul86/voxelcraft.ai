package lifecycle

import "fmt"

func NewAgentID(idNum uint64) string {
	return fmt.Sprintf("A%d", idNum)
}

func SpawnSeed(idNum uint64) (x int, z int) {
	seed := int(idNum) * 2
	return seed, -seed
}

func NewResumeToken(worldID string, nowUnixNano int64) string {
	return fmt.Sprintf("resume_%s_%d", worldID, nowUnixNano)
}
