package instants

import "strings"

func ValidateSayInput(text string) (ok bool, code string, message string) {
	if strings.TrimSpace(text) == "" {
		return false, "E_BAD_REQUEST", "missing text"
	}
	return true, "", ""
}

func ValidateWhisperInput(to, text string) (ok bool, code string, message string) {
	if strings.TrimSpace(to) == "" || strings.TrimSpace(text) == "" {
		return false, "E_BAD_REQUEST", "missing to/text"
	}
	return true, "", ""
}

func ValidateSaveMemoryInput(key string) (ok bool, code string, message string) {
	if strings.TrimSpace(key) == "" {
		return false, "E_BAD_REQUEST", "missing key"
	}
	return true, "", ""
}
