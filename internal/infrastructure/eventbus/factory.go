package eventbus

import (
	"os"
	"strings"
)

// NewEventBus creates a new EventBus instance
// Uses KAFKA_BROKERS environment variable if set, otherwise defaults to localhost:19092
// Multiple brokers can be specified as comma-separated: "broker1:9092,broker2:9092"
func NewEventBus() (EventBus, error) {
	brokersEnv := os.Getenv("KAFKA_BROKERS")
	var brokers []string

	if brokersEnv != "" {
		brokers = strings.Split(brokersEnv, ",")
		// Trim whitespace from each broker
		for i := range brokers {
			brokers[i] = strings.TrimSpace(brokers[i])
		}
	}

	// Create EventBus with specified or default brokers
	return newEventBusImpl(brokers)
}
