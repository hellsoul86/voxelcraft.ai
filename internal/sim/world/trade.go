package world

import (
	"fmt"
)

type Trade struct {
	TradeID     string
	From        string
	To          string
	Offer       map[string]int
	Request     map[string]int
	CreatedTick uint64
}

func (w *World) newTradeID() string {
	n := w.nextTradeNum.Add(1)
	return fmt.Sprintf("TR%06d", n)
}
