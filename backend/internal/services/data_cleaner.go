package services

import (
	"log"
	"math"
	"time"
)

// DataCleaner provides a pipeline for cleaning and validating market data.
type DataCleaner struct {
	// MaxPriceChangePercent is the threshold for anomaly detection (default 20%).
	MaxPriceChangePercent float64
}

// NewDataCleaner creates a new DataCleaner with default settings.
func NewDataCleaner() *DataCleaner {
	return &DataCleaner{
		MaxPriceChangePercent: 20.0,
	}
}

// CleanMarketData runs the full cleaning pipeline: dedup, fill missing, detect anomalies, adjust factor.
func (dc *DataCleaner) CleanMarketData(data []MarketData) []MarketData {
	if len(data) == 0 {
		return data
	}

	data = dc.Deduplicate(data)
	data = dc.FillMissing(data)
	data = dc.DetectAnomalies(data)
	data = dc.AdjustFactor(data)

	return data
}

// Deduplicate removes duplicate entries by symbol+timestamp, keeping the latest.
func (dc *DataCleaner) Deduplicate(data []MarketData) []MarketData {
	seen := make(map[string]int, len(data))
	result := make([]MarketData, 0, len(data))

	for _, md := range data {
		key := md.Symbol + "_" + md.Timestamp.Format(time.RFC3339Nano)
		if prevIdx, exists := seen[key]; exists {
			// Replace previous entry with the newer one
			result[prevIdx] = md
		} else {
			seen[key] = len(result)
			result = append(result, md)
		}
	}

	if len(result) < len(data) {
		log.Printf("[DataCleaner] Deduplicated: removed %d duplicate entries", len(data)-len(result))
	}
	return result
}

// FillMissing fills missing values using forward-fill (previous value).
// Fields with zero/invalid values are filled from the previous valid record for the same symbol.
func (dc *DataCleaner) FillMissing(data []MarketData) []MarketData {
	lastValid := make(map[string]*MarketData)
	filledCount := 0

	for i := range data {
		symbol := data[i].Symbol
		prev, exists := lastValid[symbol]

		if data[i].Price == 0 && exists && prev.Price != 0 {
			data[i].Price = prev.Price
			filledCount++
		}
		if data[i].Open == 0 && exists && prev.Open != 0 {
			data[i].Open = prev.Open
			filledCount++
		}
		if data[i].High == 0 && exists && prev.High != 0 {
			data[i].High = prev.High
			filledCount++
		}
		if data[i].Low == 0 && exists && prev.Low != 0 {
			data[i].Low = prev.Low
			filledCount++
		}
		if data[i].PreClose == 0 && exists && prev.PreClose != 0 {
			data[i].PreClose = prev.PreClose
			filledCount++
		}
		if data[i].Volume == 0 && exists && prev.Volume != 0 {
			data[i].Volume = prev.Volume
			filledCount++
		}
		if data[i].Amount == 0 && exists && prev.Amount != 0 {
			data[i].Amount = prev.Amount
			filledCount++
		}

		// Update last valid record
		lastValid[symbol] = &data[i]
	}

	if filledCount > 0 {
		log.Printf("[DataCleaner] FillMissing: filled %d missing values", filledCount)
	}
	return data
}

// DetectAnomalies detects anomalies where price change exceeds MaxPriceChangePercent.
// Anomalous entries are logged but returned as-is for downstream handling.
func (dc *DataCleaner) DetectAnomalies(data []MarketData) []MarketData {
	anomalyCount := 0

	for i := range data {
		md := &data[i]
		if md.PreClose == 0 {
			continue
		}

		changePercent := (md.Price - md.PreClose) / md.PreClose * 100

		if math.Abs(changePercent) > dc.MaxPriceChangePercent {
			anomalyCount++
			log.Printf("[DataCleaner] ANOMALY detected: symbol=%s price=%.2f preClose=%.2f change=%.2f%%",
				md.Symbol, md.Price, md.PreClose, changePercent)
		}
	}

	if anomalyCount > 0 {
		log.Printf("[DataCleaner] DetectAnomalies: found %d anomalous entries", anomalyCount)
	}
	return data
}

// AdjustFactor applies forward adjustment (前复权) to prices.
// For mock data, this is a no-op but the pipeline structure is in place.
func (dc *DataCleaner) AdjustFactor(data []MarketData) []MarketData {
	// In production, this would apply adjustment factors from a corporate actions database.
	// For mock data, prices are already adjusted.
	return data
}