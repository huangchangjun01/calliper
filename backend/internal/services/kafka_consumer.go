package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/quant-trading/backend/internal/models"
)

// KafkaConsumer handles consuming market data from Kafka topics and writing to TimescaleDB.
type KafkaConsumer struct {
	tsdb       *gorm.DB
	brokers    []string
	maxRetries int
	batchSize  int
}

// KafkaConsumerConfig holds configuration for Kafka consumers.
type KafkaConsumerConfig struct {
	Brokers    []string
	MaxRetries int
	BatchSize  int
}

// NewKafkaConsumer creates a new Kafka consumer.
func NewKafkaConsumer(tsdb *gorm.DB, cfg KafkaConsumerConfig) *KafkaConsumer {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}

	return &KafkaConsumer{
		tsdb:       tsdb,
		brokers:    cfg.Brokers,
		maxRetries: cfg.MaxRetries,
		batchSize:  cfg.BatchSize,
	}
}

// StartConsuming starts consuming from both "market-data" and "tick-data" topics.
func (kc *KafkaConsumer) StartConsuming(ctx context.Context) {
	go kc.consumeMarketData(ctx)
	go kc.consumeTickData(ctx)
	log.Println("[KafkaConsumer] Started consuming market-data and tick-data topics")
}

// consumeMarketData consumes from "market-data" topic and writes to stock_prices_1min and stock_prices_daily.
func (kc *KafkaConsumer) consumeMarketData(ctx context.Context) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     kc.brokers,
		Topic:       "market-data",
		GroupID:     "market-data-consumer",
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafka.LastOffset,
	})
	defer reader.Close()

	batch := make([]MarketData, 0, kc.batchSize)

	for {
		select {
		case <-ctx.Done():
			// Flush remaining batch before exit
			if len(batch) > 0 {
				kc.writeBatchWithRetry(ctx, batch)
			}
			log.Println("[KafkaConsumer] Market data consumer stopped")
			return
		default:
		}

		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				continue
			}
			log.Printf("[KafkaConsumer] Error reading market-data: %v", err)
			time.Sleep(time.Second)
			continue
		}

		var md MarketData
		if err := json.Unmarshal(msg.Value, &md); err != nil {
			log.Printf("[KafkaConsumer] Failed to unmarshal market data: %v", err)
			continue
		}

		batch = append(batch, md)

		if len(batch) >= kc.batchSize {
			kc.writeBatchWithRetry(ctx, batch)
			batch = batch[:0]
		}
	}
}

// consumeTickData consumes from "tick-data" topic and writes to stock_prices_tick.
func (kc *KafkaConsumer) consumeTickData(ctx context.Context) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     kc.brokers,
		Topic:       "tick-data",
		GroupID:     "tick-data-consumer",
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafka.LastOffset,
	})
	defer reader.Close()

	batch := make([]MarketData, 0, kc.batchSize)

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				kc.writeTickBatchWithRetry(ctx, batch)
			}
			log.Println("[KafkaConsumer] Tick data consumer stopped")
			return
		default:
		}

		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				continue
			}
			log.Printf("[KafkaConsumer] Error reading tick-data: %v", err)
			time.Sleep(time.Second)
			continue
		}

		var md MarketData
		if err := json.Unmarshal(msg.Value, &md); err != nil {
			log.Printf("[KafkaConsumer] Failed to unmarshal tick data: %v", err)
			continue
		}

		batch = append(batch, md)

		if len(batch) >= kc.batchSize {
			kc.writeTickBatchWithRetry(ctx, batch)
			batch = batch[:0]
		}
	}
}

// writeBatchWithRetry writes market data batch to TimescaleDB with retry logic.
func (kc *KafkaConsumer) writeBatchWithRetry(ctx context.Context, batch []MarketData) {
	for attempt := 0; attempt < kc.maxRetries; attempt++ {
		if err := kc.writeMarketBatch(ctx, batch); err != nil {
			log.Printf("[KafkaConsumer] Write attempt %d failed: %v", attempt+1, err)
			if attempt < kc.maxRetries-1 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}
		log.Printf("[KafkaConsumer] Wrote %d market data records to TimescaleDB", len(batch))
		return
	}
	log.Printf("[KafkaConsumer] Failed to write %d market data records after %d retries", len(batch), kc.maxRetries)
}

// writeMarketBatch writes a batch of market data to the appropriate TimescaleDB table.
func (kc *KafkaConsumer) writeMarketBatch(ctx context.Context, batch []MarketData) error {
	var minuteRecords []models.StockPriceMinute
	var dailyRecords []models.StockPriceDaily

	for _, md := range batch {
		price, _ := md.Price.Float64()
		open, _ := md.Open.Float64()
		high, _ := md.High.Float64()
		low, _ := md.Low.Float64()
		amount, _ := md.Amount.Float64()

		// Minute data
		minuteRecords = append(minuteRecords, models.StockPriceMinute{
			Time:    md.Timestamp,
			StockID: 0, // Will be resolved via symbol lookup in production
			Open:    open,
			High:    high,
			Low:     low,
			Close:   price,
			Volume:  md.Volume,
			Amount:  amount,
		})

		// Daily data (end-of-day aggregate)
		totalMarketCap, _ := md.TotalMarketCap.Float64()
		floatMarketCap, _ := md.FloatMarketCap.Float64()

		dailyRecords = append(dailyRecords, models.StockPriceDaily{
			Time:           md.Timestamp,
			StockID:        0,
			Open:           open,
			High:           high,
			Low:            low,
			Close:          price,
			Volume:         md.Volume,
			Amount:         amount,
			TurnoverRate:   md.TurnoverRate,
			PERatio:        md.PE,
			PBRatio:        md.PB,
			TotalMarketCap: totalMarketCap,
			FloatMarketCap: floatMarketCap,
		})
	}

	// Batch upsert minute data
	if len(minuteRecords) > 0 {
		if err := kc.tsdb.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "stock_id"}, {Name: "time"}},
			DoUpdates: clause.AssignmentColumns([]string{"open", "high", "low", "close", "volume", "amount"}),
		}).CreateInBatches(minuteRecords, 100).Error; err != nil {
			return fmt.Errorf("failed to write minute data: %w", err)
		}
	}

	// Batch upsert daily data
	if len(dailyRecords) > 0 {
		if err := kc.tsdb.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "stock_id"}, {Name: "time"}},
			DoUpdates: clause.AssignmentColumns([]string{"open", "high", "low", "close", "volume", "amount", "turnover_rate", "pe_ratio", "pb_ratio", "total_market_cap", "float_market_cap"}),
		}).CreateInBatches(dailyRecords, 100).Error; err != nil {
			return fmt.Errorf("failed to write daily data: %w", err)
		}
	}

	return nil
}

// writeTickBatchWithRetry writes tick data batch to TimescaleDB with retry logic.
func (kc *KafkaConsumer) writeTickBatchWithRetry(ctx context.Context, batch []MarketData) {
	for attempt := 0; attempt < kc.maxRetries; attempt++ {
		if err := kc.writeTickBatch(ctx, batch); err != nil {
			log.Printf("[KafkaConsumer] Tick write attempt %d failed: %v", attempt+1, err)
			if attempt < kc.maxRetries-1 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}
		log.Printf("[KafkaConsumer] Wrote %d tick data records to TimescaleDB", len(batch))
		return
	}
	log.Printf("[KafkaConsumer] Failed to write %d tick data records after %d retries", len(batch), kc.maxRetries)
}

// writeTickBatch writes a batch of tick data to the stock_prices_tick table.
func (kc *KafkaConsumer) writeTickBatch(ctx context.Context, batch []MarketData) error {
	var tickRecords []models.StockPriceTick

	for _, md := range batch {
		price, _ := md.Price.Float64()

		tickRecords = append(tickRecords, models.StockPriceTick{
			Time:    md.Timestamp,
			StockID: 0,
			Price:   price,
			Volume:  md.Volume,
		})
	}

	if len(tickRecords) > 0 {
		if err := kc.tsdb.WithContext(ctx).CreateInBatches(tickRecords, 100).Error; err != nil {
			return fmt.Errorf("failed to write tick data: %w", err)
		}
	}

	return nil
}