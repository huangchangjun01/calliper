package services

import (
	"log"
	"sync"
)

// QuoteSubscriptionManager tracks per-symbol WebSocket subscriber counts.
// It is used to determine collection priority: symbols with active subscribers
// should be collected at full frequency, while symbols with no subscribers
// can use a lower frequency (graceful degradation).
type QuoteSubscriptionManager struct {
	mu          sync.RWMutex
	subscribers map[string]int // symbol -> subscriber count
}

// NewQuoteSubscriptionManager creates a new QuoteSubscriptionManager.
func NewQuoteSubscriptionManager() *QuoteSubscriptionManager {
	return &QuoteSubscriptionManager{
		subscribers: make(map[string]int),
	}
}

// Subscribe increments the subscriber count for the given symbol.
// Returns the new subscriber count.
func (m *QuoteSubscriptionManager) Subscribe(symbol string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.subscribers[symbol]++
	count := m.subscribers[symbol]
	log.Printf("[Subscription] Symbol %s now has %d subscriber(s)", symbol, count)
	return count
}

// Unsubscribe decrements the subscriber count for the given symbol.
// If the count reaches zero, the symbol is removed from tracking.
// Returns the new subscriber count (0 if removed).
func (m *QuoteSubscriptionManager) Unsubscribe(symbol string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if count, ok := m.subscribers[symbol]; ok {
		if count <= 1 {
			delete(m.subscribers, symbol)
			log.Printf("[Subscription] Symbol %s has no more subscribers, removed from tracking", symbol)
			return 0
		}
		m.subscribers[symbol] = count - 1
		newCount := count - 1
		log.Printf("[Subscription] Symbol %s now has %d subscriber(s)", symbol, newCount)
		return newCount
	}
	return 0
}

// GetSubscriberCount returns the number of subscribers for the given symbol.
func (m *QuoteSubscriptionManager) GetSubscriberCount(symbol string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.subscribers[symbol]
}

// HasSubscribers returns true if the symbol has at least one active subscriber.
func (m *QuoteSubscriptionManager) HasSubscribers(symbol string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.subscribers[symbol] > 0
}

// GetAllSymbols returns a list of all symbols that currently have subscribers.
func (m *QuoteSubscriptionManager) GetAllSymbols() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	symbols := make([]string, 0, len(m.subscribers))
	for s := range m.subscribers {
		symbols = append(symbols, s)
	}
	return symbols
}

// GetSubscriberCounts returns a copy of all subscriber counts.
func (m *QuoteSubscriptionManager) GetSubscriberCounts() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int, len(m.subscribers))
	for k, v := range m.subscribers {
		result[k] = v
	}
	return result
}