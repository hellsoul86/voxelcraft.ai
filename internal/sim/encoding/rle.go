package encoding

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

// EncodeRLE encodes a sequence of palette ids into base64(varint pairs).
// The pairs are (block_id, run_len) repeated.
func EncodeRLE(ids []uint16) string {
	var buf bytes.Buffer
	var tmp [binary.MaxVarintLen64]byte

	i := 0
	for i < len(ids) {
		b := ids[i]
		run := 1
		for j := i + 1; j < len(ids) && ids[j] == b && run < 1<<31; j++ {
			run++
		}

		n := binary.PutUvarint(tmp[:], uint64(b))
		buf.Write(tmp[:n])
		n = binary.PutUvarint(tmp[:], uint64(run))
		buf.Write(tmp[:n])

		i += run
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func DecodeRLE(b64 string) ([]uint16, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	var out []uint16
	for i := 0; i < len(raw); {
		b, n := binary.Uvarint(raw[i:])
		if n <= 0 {
			return nil, fmt.Errorf("bad varint at %d", i)
		}
		i += n
		run, n := binary.Uvarint(raw[i:])
		if n <= 0 {
			return nil, fmt.Errorf("bad varint at %d", i)
		}
		i += n
		if b > 0xFFFF {
			return nil, fmt.Errorf("block id too large: %d", b)
		}
		for k := 0; k < int(run); k++ {
			out = append(out, uint16(b))
		}
	}
	return out, nil
}
