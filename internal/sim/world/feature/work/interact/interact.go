package interact

type BoardPost struct {
	PostID string
	Author string
	Title  string
	Body   string
	Tick   uint64
}

func ValidateBoardOpen(blockName string, distance int) (bool, string, string) {
	if blockName != "BULLETIN_BOARD" {
		return false, "E_INVALID_TARGET", "board not found"
	}
	if distance > 3 {
		return false, "E_BLOCKED", "too far"
	}
	return true, "", ""
}

func ValidateSignOpen(blockName string, distance int) (bool, string, string) {
	if blockName != "SIGN" {
		return false, "E_INVALID_TARGET", "sign not found"
	}
	if distance > 3 {
		return false, "E_BLOCKED", "too far"
	}
	return true, "", ""
}

func BuildBoardPosts(posts []BoardPost, max int) []map[string]any {
	if max <= 0 {
		max = 20
	}
	start := 0
	if len(posts) > max {
		start = len(posts) - max
	}
	out := make([]map[string]any, 0, len(posts)-start)
	for i := start; i < len(posts); i++ {
		p := posts[i]
		out = append(out, map[string]any{
			"post_id": p.PostID,
			"author":  p.Author,
			"title":   p.Title,
			"body":    p.Body,
			"tick":    p.Tick,
		})
	}
	return out
}

func ValidateTransferNoop(srcID string, dstID string) (bool, string, string) {
	if srcID == "SELF" && dstID == "SELF" {
		return false, "E_BAD_REQUEST", "no-op transfer"
	}
	return true, "", ""
}

func ValidateContainerDistance(found bool, distance int, which string) (bool, string, string) {
	if !found {
		return false, "E_INVALID_TARGET", which + " container not found"
	}
	if distance > 3 {
		return false, "E_BLOCKED", "too far from " + which
	}
	return true, "", ""
}
