package mock

import (
	"context"
	"fmt"
	"time"
)

type PaymentRequest struct {
	PaymentID string
	Amount    float64
	Currency  string
	CardToken string
}

type GatewayResponse struct {
	GatewayPaymentID string
	Status           string
	TransactionID    string
}

type ExternalGateway interface {
	ProcessPayment(ctx context.Context, req PaymentRequest) (*GatewayResponse, error)
}

type MockExternalGateway struct {
	successRate     float64
	avgLatency      time.Duration
	timeoutRate     float64
	timeoutDuration time.Duration
}

func NewMockExternalGateway() *MockExternalGateway {
	return &MockExternalGateway{
		successRate:     0.9, // 90% success rate
		avgLatency:      100 * time.Millisecond,
		timeoutRate:     0.0, // No timeouts by default
		timeoutDuration: 30 * time.Second,
	}
}

func (mg *MockExternalGateway) SetTimeoutRate(rate float64) {
	mg.timeoutRate = rate
}

func (mg *MockExternalGateway) ProcessPayment(ctx context.Context, req PaymentRequest) (*GatewayResponse, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Check for timeout simulation based on timeout rate
	// This allows testing timeout scenarios by setting timeoutRate > 0
	if mg.timeoutRate > 0 {
		// Simple timeout simulation: use payment ID hash to determine if timeout
		hash := 0
		for _, c := range req.PaymentID {
			hash += int(c)
		}
		if hash%100 < int(mg.timeoutRate*100) {
			// Simulate timeout: wait longer than context timeout
			// The context will timeout first (30s), causing context.DeadlineExceeded
			select {
			case <-ctx.Done():
				return nil, ctx.Err() // This will be context.DeadlineExceeded
			case <-time.After(mg.timeoutDuration + 5*time.Second):
				// Fallback if context doesn't timeout (shouldn't happen)
				return nil, fmt.Errorf("gateway timeout: no response after %v", mg.timeoutDuration)
			}
		}
	}

	// Simulate latency
	select {
	case <-time.After(mg.avgLatency):
		// Continue
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Simulate success/failure based on success rate
	// For MVP, always succeed (unless timeout)
	return &GatewayResponse{
		GatewayPaymentID: fmt.Sprintf("gateway_%s", req.PaymentID),
		Status:           "SUCCESS",
		TransactionID:    fmt.Sprintf("txn_%d", time.Now().Unix()),
	}, nil
}
