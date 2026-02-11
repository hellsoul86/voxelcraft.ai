package observer

import "strings"

func ResolveBoardID(boardID, targetID string) string {
	b := strings.TrimSpace(boardID)
	t := strings.TrimSpace(targetID)
	if t != "" {
		b = t
	}
	return b
}

func ValidatePostInput(boardID, title, body string) (ok bool, code string, message string) {
	if boardID == "" || title == "" || body == "" {
		return false, "E_BAD_REQUEST", "missing board_id/target_id/title/body"
	}
	if len(title) > 80 || len(body) > 2000 {
		return false, "E_BAD_REQUEST", "post too large"
	}
	return true, "", ""
}

func ValidateSearchInput(boardID, query string) (ok bool, code string, message string) {
	if boardID == "" || query == "" {
		return false, "E_BAD_REQUEST", "missing board_id/target_id/text"
	}
	if len(query) > 120 {
		return false, "E_BAD_REQUEST", "query too large"
	}
	return true, "", ""
}
