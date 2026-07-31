package services

import (
	"net"
	"net/http"
	"time"
)

// newPooledHTTPClient creates an HTTP client with connection pool limits
// to prevent unbounded goroutine and connection growth during periodic polling.
func newPooledHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 4,
			MaxConnsPerHost:     8,
			IdleConnTimeout:     60 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
			DisableCompression:  true, // Sina responses are small text, no need to decompress
		},
	}
}

// MarketData represents a single market data snapshot for a stock.
// Uses float64 instead of decimal.Decimal to avoid big.Int heap allocations
// and reduce GC pressure in memory-constrained environments.
type MarketData struct {
	Symbol         string    `json:"symbol"`
	Name           string    `json:"name"`
	Price          float64   `json:"price"`
	Open           float64   `json:"open"`
	High           float64   `json:"high"`
	Low            float64   `json:"low"`
	PreClose       float64   `json:"pre_close"`
	Volume         int64     `json:"volume"`
	Amount         float64   `json:"amount"`
	Change         float64   `json:"change"`
	ChangePercent  float64   `json:"change_percent"`
	TurnoverRate   float64   `json:"turnover_rate"`
	PE             float64   `json:"pe"`
	PB             float64   `json:"pb"`
	TotalMarketCap float64   `json:"total_market_cap"`
	FloatMarketCap float64   `json:"float_market_cap"`
	BidPrices      []float64 `json:"bid_prices"`
	BidVolumes     []int64   `json:"bid_volumes"`
	AskPrices      []float64 `json:"ask_prices"`
	AskVolumes     []int64   `json:"ask_volumes"`
	Timestamp      time.Time `json:"timestamp"`
	MarketCode     string    `json:"market_code"`
}

// MarketDataCollector defines the interface for fetching market data from a specific source.
type MarketDataCollector interface {
	// FetchRealTimeData fetches real-time market data for the given symbols.
	FetchRealTimeData(symbols []string) ([]MarketData, error)

	// FetchHistoricalData fetches historical market data for a single symbol.
	FetchHistoricalData(symbol string, start, end time.Time, interval string) ([]MarketData, error)

	// GetMarketCode returns the market code identifier (e.g., "CN", "US").
	GetMarketCode() string
}