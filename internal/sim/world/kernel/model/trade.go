package model

type Trade struct {
	TradeID     string
	From        string
	To          string
	Offer       map[string]int
	Request     map[string]int
	CreatedTick uint64
}

