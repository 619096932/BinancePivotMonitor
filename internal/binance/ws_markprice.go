package binance

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

const FStreamWSBaseURL = "wss://fstream.binance.com/ws"

type MarkPriceEvent struct {
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	MarkPrice string `json:"p"`
}

func (e *MarkPriceEvent) UnmarshalJSON(data []byte) error {
	var aux struct {
		EventTime json.RawMessage `json:"E"`
		Symbol    string          `json:"s"`
		MarkPrice json.RawMessage `json:"p"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	e.Symbol = aux.Symbol

	if len(aux.EventTime) > 0 {
		var n int64
		if err := json.Unmarshal(aux.EventTime, &n); err == nil {
			e.EventTime = n
		} else {
			var s string
			if err2 := json.Unmarshal(aux.EventTime, &s); err2 == nil {
				if v, err3 := strconv.ParseInt(s, 10, 64); err3 == nil {
					e.EventTime = v
				}
			}
		}
	}

	if len(aux.MarkPrice) > 0 {
		var s string
		if err := json.Unmarshal(aux.MarkPrice, &s); err == nil {
			e.MarkPrice = s
		} else {
			var f float64
			if err2 := json.Unmarshal(aux.MarkPrice, &f); err2 == nil {
				e.MarkPrice = strconv.FormatFloat(f, 'f', -1, 64)
			}
		}
	}

	return nil
}

func DialMarkPriceArr1s(ctx context.Context) (*websocket.Conn, *http.Response, error) {
	d := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 10 * time.Second,
	}
	url := FStreamWSBaseURL + "/!markPrice@arr@1s"
	return d.DialContext(ctx, url, nil)
}
