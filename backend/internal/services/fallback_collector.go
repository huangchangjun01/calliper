package services

import (
	"log"
	"time"
)

// FallbackCollector implements MarketDataCollector with primary and fallback data sources.
// When the primary source fails, it automatically falls back to the secondary source.
type FallbackCollector struct {
	primary   MarketDataCollector
	fallback  MarketDataCollector
	marketCode string
}

// NewFallbackCollector creates a collector that tries primary first, then fallback.
func NewFallbackCollector(primary, fallback MarketDataCollector) *FallbackCollector {
	return &FallbackCollector{
		primary:    primary,
		fallback:   fallback,
		marketCode: primary.GetMarketCode(),
	}
}

// GetMarketCode returns the market code.
func (c *FallbackCollector) GetMarketCode() string {
	return c.marketCode
}

// FetchRealTimeData tries the primary source first.
// If it fails, returns empty data, or returns fewer than half of the requested symbols,
// falls back to the secondary source and merges the results.
func (c *FallbackCollector) FetchRealTimeData(symbols []string) ([]MarketData, error) {
	// Try primary source
	data, err := c.primary.FetchRealTimeData(symbols)

	// Determine if fallback is needed
	needFallback := false
	if err != nil {
		log.Printf("[Fallback] Primary source (%T) failed: %v, switching to fallback (%T)",
			c.primary, err, c.fallback)
		needFallback = true
	} else if len(data) == 0 {
		log.Printf("[Fallback] Primary source (%T) returned empty data, switching to fallback (%T)",
			c.primary, c.fallback)
		needFallback = true
	} else if len(symbols) > 0 && len(data) < len(symbols) {
		// Primary returned partial data — try fallback for missing symbols
		log.Printf("[Fallback] Primary source (%T) returned only %d/%d quotes, trying fallback for better coverage",
			c.primary, len(data), len(symbols))

		// Build set of symbols already covered by primary
		covered := make(map[string]bool, len(data))
		for _, md := range data {
			covered[md.Symbol] = true
		}

		// Try fallback to fill in missing symbols
		fbData, fbErr := c.fallback.FetchRealTimeData(symbols)
		if fbErr != nil {
			log.Printf("[Fallback] Fallback source (%T) failed: %v, using partial primary data", c.fallback, fbErr)
		} else {
			// Merge: add fallback results for symbols not already covered by primary
			added := 0
			for _, md := range fbData {
				if !covered[md.Symbol] {
					data = append(data, md)
					added++
				}
			}
			log.Printf("[Fallback] Merged %d additional quotes from fallback (%T), total: %d", added, c.fallback, len(data))
		}
		return data, nil
	}

	if needFallback {
		// Try fallback source
		data, err = c.fallback.FetchRealTimeData(symbols)
		if err != nil {
			log.Printf("[Fallback] Fallback source (%T) also failed: %v", c.fallback, err)
			return nil, err
		}
		log.Printf("[Fallback] Fallback source (%T) returned %d quotes", c.fallback, len(data))
	}

	return data, nil
}

// FetchHistoricalData tries the primary source first, then falls back.
func (c *FallbackCollector) FetchHistoricalData(symbol string, start, end time.Time, interval string) ([]MarketData, error) {
	// Try primary source
	data, err := c.primary.FetchHistoricalData(symbol, start, end, interval)
	if err == nil && len(data) > 0 {
		return data, nil
	}

	if err != nil {
		log.Printf("[Fallback] Primary historical (%T) failed for %s: %v, switching to fallback (%T)",
			c.primary, symbol, err, c.fallback)
	} else {
		log.Printf("[Fallback] Primary historical (%T) returned empty for %s, switching to fallback (%T)",
			c.primary, symbol, c.fallback)
	}

	// Try fallback source
	data, err = c.fallback.FetchHistoricalData(symbol, start, end, interval)
	if err != nil {
		log.Printf("[Fallback] Fallback historical (%T) also failed for %s: %v", c.fallback, symbol, err)
		return nil, err
	}

	log.Printf("[Fallback] Fallback historical (%T) returned %d records for %s", c.fallback, len(data), symbol)
	return data, nil
}