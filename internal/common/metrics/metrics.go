package metrics

import "sync"

// Collector defines the interface for metrics collection
type Collector interface {
	IncrementCounter(name string)
	GetCounter(name string) int64
}

type MockCollector struct {
	counters map[string]int64
	mu       sync.RWMutex
}

func NewMockCollector() *MockCollector {
	return &MockCollector{
		counters: make(map[string]int64),
	}
}

func (mc *MockCollector) IncrementCounter(name string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.counters[name]++
}

func (mc *MockCollector) GetCounter(name string) int64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.counters[name]
}
