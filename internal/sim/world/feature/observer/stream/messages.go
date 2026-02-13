package stream

import (
	"encoding/json"

	"voxelcraft.ai/internal/observerproto"
	"voxelcraft.ai/internal/sim/world/io/obscodec"
)

func EncodeSurfacePAL16Y8(blocks []uint16, ys []byte) string {
	return obscodec.EncodePAL16Y8(blocks, ys)
}

func EncodeVoxelsPAL16U16LE(blocks []uint16) string {
	return obscodec.EncodePAL16U16LE(blocks)
}

func BuildChunkSurfaceMsg(cx, cz int, data string) ([]byte, error) {
	msg := observerproto.ChunkSurfaceMsg{
		Type:            "CHUNK_SURFACE",
		ProtocolVersion: observerproto.Version,
		CX:              cx,
		CZ:              cz,
		Encoding:        "PAL16_Y8",
		Data:            data,
	}
	return json.Marshal(msg)
}

func BuildChunkPatchMsg(cx, cz int, cells []observerproto.ChunkPatchCell) ([]byte, error) {
	msg := observerproto.ChunkPatchMsg{
		Type:            "CHUNK_PATCH",
		ProtocolVersion: observerproto.Version,
		CX:              cx,
		CZ:              cz,
		Cells:           cells,
	}
	return json.Marshal(msg)
}

func BuildChunkEvictMsg(cx, cz int) ([]byte, error) {
	msg := observerproto.ChunkEvictMsg{
		Type:            "CHUNK_EVICT",
		ProtocolVersion: observerproto.Version,
		CX:              cx,
		CZ:              cz,
	}
	return json.Marshal(msg)
}

func BuildChunkVoxelsMsg(cx, cz int, data string) ([]byte, error) {
	msg := observerproto.ChunkVoxelsMsg{
		Type:            "CHUNK_VOXELS",
		ProtocolVersion: observerproto.Version,
		CX:              cx,
		CZ:              cz,
		Encoding:        "PAL16_U16LE_YZX",
		Data:            data,
	}
	return json.Marshal(msg)
}

func BuildChunkVoxelPatchMsg(cx, cz int, cells []observerproto.ChunkVoxelPatchCell) ([]byte, error) {
	msg := observerproto.ChunkVoxelPatchMsg{
		Type:            "CHUNK_VOXEL_PATCH",
		ProtocolVersion: observerproto.Version,
		CX:              cx,
		CZ:              cz,
		Cells:           cells,
	}
	return json.Marshal(msg)
}

func BuildChunkVoxelsEvictMsg(cx, cz int) ([]byte, error) {
	msg := observerproto.ChunkVoxelsEvictMsg{
		Type:            "CHUNK_VOXELS_EVICT",
		ProtocolVersion: observerproto.Version,
		CX:              cx,
		CZ:              cz,
	}
	return json.Marshal(msg)
}
