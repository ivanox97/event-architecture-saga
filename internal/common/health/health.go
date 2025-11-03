package health

import "context"

type HealthChecker interface {
	Check(ctx context.Context) HealthStatus
}

type HealthStatus struct {
	Status string `json:"status"`
}

type MockHealthChecker struct{}

func NewMockHealthChecker() *MockHealthChecker {
	return &MockHealthChecker{}
}

// Check performs a health check
func (mh *MockHealthChecker) Check(ctx context.Context) HealthStatus {
	return HealthStatus{
		Status: "healthy",
	}
}
