package targets

func ValidatePhysicalBoardTarget(targetType string, blockName string, distance int, postingAllowed bool) (ok bool, code string, message string) {
	if targetType != "BULLETIN_BOARD" {
		return false, "E_BAD_REQUEST", "invalid board target"
	}
	if blockName != "BULLETIN_BOARD" {
		return false, "E_INVALID_TARGET", "bulletin board not found"
	}
	if distance > 3 {
		return false, "E_BLOCKED", "too far"
	}
	if !postingAllowed {
		return false, "E_NO_PERMISSION", "posting not allowed here"
	}
	return true, "", ""
}

func ValidateSetSignTarget(targetType string, blockName string, distance int, textLen int) (ok bool, code string, message string) {
	if targetType != "SIGN" {
		return false, "E_BAD_REQUEST", "invalid sign target"
	}
	if blockName != "SIGN" {
		return false, "E_INVALID_TARGET", "sign not found"
	}
	if distance > 3 {
		return false, "E_BLOCKED", "too far"
	}
	if textLen > 200 {
		return false, "E_BAD_REQUEST", "text too large"
	}
	return true, "", ""
}
