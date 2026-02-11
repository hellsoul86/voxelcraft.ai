package digestcodec

func BoolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}

func Clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
