package config

import "strings"

func ClampChunkRadius(v int) int {
	if v < 1 {
		return 1
	}
	if v > 32 {
		return 32
	}
	return v
}

func ClampMaxChunks(v int) int {
	if v < 1 {
		return 1
	}
	if v > 16384 {
		return 16384
	}
	return v
}

func ClampVoxelRadius(v int) int {
	if v < 0 {
		return 0
	}
	if v > 8 {
		return 8
	}
	return v
}

func ClampVoxelMaxChunks(v int) int {
	if v < 1 {
		return 1
	}
	if v > 2048 {
		return 2048
	}
	return v
}

func NormalizeFocusAgentID(v string) string {
	return strings.TrimSpace(v)
}
