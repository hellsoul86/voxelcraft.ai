package obscodec

import "encoding/base64"

func EncodePAL16Y8(blocks []uint16, ys []byte) string {
	n := len(blocks)
	if len(ys) < n {
		n = len(ys)
	}
	buf := make([]byte, n*3)
	for i := 0; i < n; i++ {
		off := i * 3
		b := blocks[i]
		buf[off] = byte(b)
		buf[off+1] = byte(b >> 8)
		buf[off+2] = ys[i]
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func EncodePAL16U16LE(blocks []uint16) string {
	buf := make([]byte, len(blocks)*2)
	for i, v := range blocks {
		off := i * 2
		buf[off] = byte(v)
		buf[off+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}
