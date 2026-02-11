package economy

import "fmt"

type Trade struct {
	TradeID     string
	From        string
	To          string
	Offer       map[string]int
	Request     map[string]int
	CreatedTick uint64
}

func TradeID(n uint64) string {
	return fmt.Sprintf("TR%06d", n)
}
