package services

import "time"

// StockRaw represents a standardized stock record from any data source.
type StockRaw struct {
	Symbol    string  `json:"symbol"`
	Name      string  `json:"name"`
	NameCN    string  `json:"name_cn,omitempty"`
	Exchange  string  `json:"exchange"`
	Industry  string  `json:"industry,omitempty"`
	Sector    string  `json:"sector,omitempty"`
	MarketCap float64 `json:"market_cap,omitempty"`
	Currency  string  `json:"currency"`
	LotSize   int     `json:"lot_size"`
	IsActive  bool    `json:"is_active"`
}

// DataSource defines the interface that every stock data provider must implement.
type DataSource interface {
	// FetchStockList returns all stocks for a given market code.
	FetchStockList(marketCode string) ([]StockRaw, error)

	// HealthCheck returns nil if the data source is reachable, or an error otherwise.
	HealthCheck() error
}

// MarketInfo holds mapping metadata for a market code.
type MarketInfo struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Country  string `json:"country"`
	Currency string `json:"currency"`
	Timezone string `json:"timezone"`
}

// DefaultMarkets returns the list of supported markets.
func DefaultMarkets() []MarketInfo {
	return []MarketInfo{
		{Code: "SSE", Name: "Shanghai Stock Exchange", Country: "CN", Currency: "CNY", Timezone: "Asia/Shanghai"},
		{Code: "SZSE", Name: "Shenzhen Stock Exchange", Country: "CN", Currency: "CNY", Timezone: "Asia/Shanghai"},
		{Code: "BSE", Name: "Beijing Stock Exchange", Country: "CN", Currency: "CNY", Timezone: "Asia/Shanghai"},
		{Code: "HKEX", Name: "Hong Kong Stock Exchange", Country: "HK", Currency: "HKD", Timezone: "Asia/Hong_Kong"},
		{Code: "NYSE", Name: "New York Stock Exchange", Country: "US", Currency: "USD", Timezone: "America/New_York"},
		{Code: "NASDAQ", Name: "NASDAQ", Country: "US", Currency: "USD", Timezone: "America/New_York"},
		{Code: "AMEX", Name: "NYSE American", Country: "US", Currency: "USD", Timezone: "America/New_York"},
		{Code: "TSE", Name: "Tokyo Stock Exchange", Country: "JP", Currency: "JPY", Timezone: "Asia/Tokyo"},
		{Code: "LSE", Name: "London Stock Exchange", Country: "GB", Currency: "GBP", Timezone: "Europe/London"},
		{Code: "Euronext", Name: "Euronext", Country: "EU", Currency: "EUR", Timezone: "Europe/Paris"},
		{Code: "Xetra", Name: "Deutsche Börse Xetra", Country: "DE", Currency: "EUR", Timezone: "Europe/Berlin"},
		{Code: "ASX", Name: "Australian Securities Exchange", Country: "AU", Currency: "AUD", Timezone: "Australia/Sydney"},
		{Code: "TSX", Name: "Toronto Stock Exchange", Country: "CA", Currency: "CAD", Timezone: "America/Toronto"},
		{Code: "KRX", Name: "Korea Exchange", Country: "KR", Currency: "KRW", Timezone: "Asia/Seoul"},
	}
}

// IsChineseMarket returns true if the market code belongs to a Chinese exchange.
func IsChineseMarket(code string) bool {
	switch code {
	case "SSE", "SZSE", "BSE":
		return true
	default:
		return false
	}
}

// IsOverseasMarket returns true if the market code belongs to an overseas exchange.
func IsOverseasMarket(code string) bool {
	switch code {
	case "HKEX", "NYSE", "NASDAQ", "AMEX", "TSE", "LSE", "Euronext", "Xetra", "ASX", "TSX", "KRX":
		return true
	default:
		return false
	}
}

// MarketCodeToCurrency returns the default currency for a market code.
func MarketCodeToCurrency(code string) string {
	markets := DefaultMarkets()
	for _, m := range markets {
		if m.Code == code {
			return m.Currency
		}
	}
	return "USD"
}

// MarketCodeToCountry returns the country code for a market code.
func MarketCodeToCountry(code string) string {
	markets := DefaultMarkets()
	for _, m := range markets {
		if m.Code == code {
			return m.Country
		}
	}
	return "UNKNOWN"
}

// RetryConfig holds retry parameters.
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	Timeout    time.Duration
}

// DefaultRetryConfig returns standard retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  2 * time.Second,
		Timeout:    30 * time.Second,
	}
}