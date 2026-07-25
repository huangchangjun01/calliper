package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaProducer handles publishing market data to Kafka topics.
type KafkaProducer struct {
	marketWriter *kafka.Writer
	tickWriter   *kafka.Writer
}

// KafkaProducerConfig holds configuration for Kafka producers.
type KafkaProducerConfig struct {
	Brokers []string
}

// NewKafkaProducer creates a new Kafka producer.
func NewKafkaProducer(cfg KafkaProducerConfig) *KafkaProducer {
	marketWriter := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        "market-data",
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    100,
		BatchTimeout: 100 * time.Millisecond,
		Async:        false,
	}

	tickWriter := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        "tick-data",
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    100,
		BatchTimeout: 50 * time.Millisecond,
		Async:        false,
	}

	return &KafkaProducer{
		marketWriter: marketWriter,
		tickWriter:   tickWriter,
	}
}

// PublishMarketData sends market data to the "market-data" topic.
func (p *KafkaProducer) PublishMarketData(ctx context.Context, data []MarketData) error {
	if len(data) == 0 {
		return nil
	}

	// No-op mode: writer is nil, just log and return
	if p.marketWriter == nil {
		log.Printf("[KafkaProducer] NOOP mode: would publish %d market data messages", len(data))
		return nil
	}

	messages := make([]kafka.Message, 0, len(data))
	for _, md := range data {
		payload, err := json.Marshal(md)
		if err != nil {
			log.Printf("[KafkaProducer] Failed to marshal market data for %s: %v", md.Symbol, err)
			continue
		}
		messages = append(messages, kafka.Message{
			Key:   []byte(md.Symbol),
			Value: payload,
		})
	}

	if len(messages) == 0 {
		return nil
	}

	err := p.marketWriter.WriteMessages(ctx, messages...)
	if err != nil {
		return fmt.Errorf("failed to publish market data: %w", err)
	}

	log.Printf("[KafkaProducer] Published %d market data messages", len(messages))
	return nil
}

// PublishTickData sends tick data to the "tick-data" topic.
func (p *KafkaProducer) PublishTickData(ctx context.Context, data []MarketData) error {
	if len(data) == 0 {
		return nil
	}

	// No-op mode: writer is nil, just log and return
	if p.tickWriter == nil {
		log.Printf("[KafkaProducer] NOOP mode: would publish %d tick data messages", len(data))
		return nil
	}

	messages := make([]kafka.Message, 0, len(data))
	for _, md := range data {
		payload, err := json.Marshal(md)
		if err != nil {
			log.Printf("[KafkaProducer] Failed to marshal tick data for %s: %v", md.Symbol, err)
			continue
		}
		messages = append(messages, kafka.Message{
			Key:   []byte(md.Symbol),
			Value: payload,
		})
	}

	if len(messages) == 0 {
		return nil
	}

	err := p.tickWriter.WriteMessages(ctx, messages...)
	if err != nil {
		return fmt.Errorf("failed to publish tick data: %w", err)
	}

	log.Printf("[KafkaProducer] Published %d tick data messages", len(messages))
	return nil
}

// Close closes both Kafka writers.
func (p *KafkaProducer) Close() error {
	if p.marketWriter != nil {
		if err := p.marketWriter.Close(); err != nil {
			log.Printf("[KafkaProducer] Error closing market writer: %v", err)
		}
	}
	if p.tickWriter != nil {
		if err := p.tickWriter.Close(); err != nil {
			log.Printf("[KafkaProducer] Error closing tick writer: %v", err)
		}
	}
	return nil
}

// NewKafkaProducerNoop creates a no-op Kafka producer for when Kafka is unavailable.
func NewKafkaProducerNoop() *KafkaProducer {
	return &KafkaProducer{}
}

// PublishMarketData (noop) logs data instead of sending to Kafka.
func (p *KafkaProducer) PublishMarketDataNoop(ctx context.Context, data []MarketData) {
	log.Printf("[KafkaProducer] NOOP mode: would publish %d market data messages", len(data))
}

// PublishTickData (noop) logs data instead of sending to Kafka.
func (p *KafkaProducer) PublishTickDataNoop(ctx context.Context, data []MarketData) {
	log.Printf("[KafkaProducer] NOOP mode: would publish %d tick data messages", len(data))
}