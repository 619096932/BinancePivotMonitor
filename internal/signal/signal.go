package signal

import "time"

type Signal struct {
	ID          string    `json:"id"`
	Symbol      string    `json:"symbol"`
	Period      string    `json:"period"`
	Level       string    `json:"level"`
	Price       float64   `json:"price"`
	Direction   string    `json:"direction"`
	TriggeredAt time.Time `json:"triggered_at"`
	Source      string    `json:"source"`
}
