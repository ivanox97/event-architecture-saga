package configs

import (
	"os"
)

// Database Configuration
const (
	// DefaultDatabaseURL is for local development only
	// In production, always use DATABASE_URL environment variable
	DefaultDatabaseURL = "postgres://event_saga:event_saga_pass@localhost:5433/event_saga_db?sslmode=disable"
	DatabaseURLEnvKey  = "DATABASE_URL"
)

// Service Ports
const (
	PortSagaOrchestrator       = "8080"
	PortWalletService          = "8081"
	PortExternalPaymentService = "8082"
	PortMetricsService         = "8083"
)

// Event Topics
const (
	TopicPayments = "events.payments.v1"
	TopicDLQ      = "events.dlq.v1"
)

// Service Names
const (
	ServiceNameSagaOrchestrator       = "saga-orchestrator"
	ServiceNameWalletService          = "wallet-service"
	ServiceNameExternalPaymentService = "external-payment-service"
	ServiceNameMetricsService         = "metrics-service"
)

// GetDatabaseURL returns the database URL from environment or default value
func GetDatabaseURL() string {
	if value := os.Getenv(DatabaseURLEnvKey); value != "" {
		return value
	}
	return DefaultDatabaseURL
}
