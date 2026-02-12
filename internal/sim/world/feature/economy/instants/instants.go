package instants

import "strings"

func ValidateOfferTradeInput(allowTrade bool, to string) (ok bool, code string, msg string) {
	if !allowTrade {
		return false, "E_NO_PERMISSION", "trade disabled in this world"
	}
	if strings.TrimSpace(to) == "" {
		return false, "E_BAD_REQUEST", "missing to"
	}
	return true, "", ""
}

func ValidateTradeLifecycleInput(allowTrade bool, tradeID string) (ok bool, code string, msg string) {
	if !allowTrade {
		return false, "E_NO_PERMISSION", "trade disabled in this world"
	}
	if strings.TrimSpace(tradeID) == "" {
		return false, "E_BAD_REQUEST", "missing trade_id"
	}
	return true, "", ""
}

func ValidateTradeOfferPairs(offer map[string]int, offerErr error, req map[string]int, reqErr error) (ok bool, code string, msg string) {
	if offerErr != nil || len(offer) == 0 {
		return false, "E_BAD_REQUEST", "bad offer"
	}
	if reqErr != nil || len(req) == 0 {
		return false, "E_BAD_REQUEST", "bad request"
	}
	return true, "", ""
}
