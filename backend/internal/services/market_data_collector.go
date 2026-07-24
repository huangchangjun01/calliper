package services

import (
	"time"

	"github.com/shopspring/decimal"
)

// MarketData represents a single market data snapshot for a stock.
type MarketData struct {
	Symbol         string            `json:"symbol"`
	Name           string            `json:"name"`
	Price          decimal.Decimal   `json:"price"`
	Open           decimal.Decimal   `json:"open"`
	High           decimal.Decimal   `json:"high"`
	Low            decimal.Decimal   `json:"low"`
	PreClose       decimal.Decimal   `json:"pre_close"`
	Volume         int64             `json:"volume"`
	Amount         decimal.Decimal   `json:"amount"`
	Change         decimal.Decimal   `json:"change"`
	ChangePercent  float64           `json:"change_percent"`
	TurnoverRate   float64           `json:"turnover_rate"`
	PE             float64           `json:"pe"`
	PB             float64           `json:"pb"`
	TotalMarketCap decimal.Decimal   `json:"total_market_cap"`
	FloatMarketCap decimal.Decimal   `json:"float_market_cap"`
	BidPrices      []decimal.Decimal `json:"bid_prices"`
	BidVolumes     []int64           `json:"bid_volumes"`
	AskPrices      []decimal.Decimal `json:"ask_prices"`
	AskVolumes     []int64           `json:"ask_volumes"`
	Timestamp      time.Time         `json:"timestamp"`
	MarketCode     string            `json:"market_code"`
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