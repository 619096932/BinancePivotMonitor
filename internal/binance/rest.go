package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type RESTClient struct {
	BaseURL string
	HTTP    *http.Client
}

func NewRESTClient(baseURL string) *RESTClient {
	return &RESTClient{
		BaseURL: baseURL,
		HTTP: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type exchangeInfoResp struct {
	Symbols []struct {
		Symbol       string `json:"symbol"`
		Status       string `json:"status"`
		ContractType string `json:"contractType"`
		QuoteAsset   string `json:"quoteAsset"`
	} `json:"symbols"`
}

func (c *RESTClient) ExchangeInfoUSDTPERP(ctx context.Context) ([]string, error) {
	url := c.BaseURL + "/fapi/v1/exchangeInfo"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("exchangeInfo status=%d body=%s", resp.StatusCode, string(b))
	}

	var out exchangeInfoResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	symbols := make([]string, 0, len(out.Symbols))
	for _, s := range out.Symbols {
		if s.Status != "TRADING" {
			continue
		}
		if s.ContractType != "PERPETUAL" {
			continue
		}
		if s.QuoteAsset != "USDT" {
			continue
		}
		symbols = append(symbols, s.Symbol)
	}
	return symbols, nil
}

func (c *RESTClient) PrevKline(ctx context.Context, symbol, interval string) (high, low, close float64, err error) {
	url := fmt.Sprintf("%s/fapi/v1/klines?symbol=%s&interval=%s&limit=2", c.BaseURL, symbol, interval)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, 0, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return 0, 0, 0, fmt.Errorf("klines %s %s status=%d body=%s", symbol, interval, resp.StatusCode, string(b))
	}

	var raw [][]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return 0, 0, 0, err
	}
	if len(raw) < 2 {
		return 0, 0, 0, fmt.Errorf("klines %s %s: not enough data", symbol, interval)
	}

	k := raw[len(raw)-2]
	if len(k) < 5 {
		return 0, 0, 0, fmt.Errorf("klines %s %s: invalid kline", symbol, interval)
	}

	highStr, ok := k[2].(string)
	if !ok {
		return 0, 0, 0, fmt.Errorf("klines %s %s: high not string", symbol, interval)
	}
	lowStr, ok := k[3].(string)
	if !ok {
		return 0, 0, 0, fmt.Errorf("klines %s %s: low not string", symbol, interval)
	}
	closeStr, ok := k[4].(string)
	if !ok {
		return 0, 0, 0, fmt.Errorf("klines %s %s: close not string", symbol, interval)
	}

	high, err = strconv.ParseFloat(highStr, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	low, err = strconv.ParseFloat(lowStr, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	close, err = strconv.ParseFloat(closeStr, 64)
	if err != nil {
		return 0, 0, 0, err
	}

	return high, low, close, nil
}
