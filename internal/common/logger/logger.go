package logger

import "fmt"

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// Logger defines the interface for logging
type Logger interface {
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
}

type MockLogger struct{}

func NewMockLogger() *MockLogger {
	return &MockLogger{}
}

// Info logs an info message
func (ml *MockLogger) Info(msg string, fields ...Field) {
	fmt.Print("[INFO] " + msg)
	if len(fields) > 0 {
		fmt.Print(" [")
		for i, f := range fields {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%s=%v", f.Key, f.Value)
		}
		fmt.Print("]")
	}
	fmt.Println()
}

// Warn logs a warning message
func (ml *MockLogger) Warn(msg string, fields ...Field) {
	fmt.Print("[WARN] " + msg)
	if len(fields) > 0 {
		fmt.Print(" [")
		for i, f := range fields {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%s=%v", f.Key, f.Value)
		}
		fmt.Print("]")
	}
	fmt.Println()
}

// Error logs an error message
func (ml *MockLogger) Error(msg string, fields ...Field) {
	fmt.Print("[ERROR] " + msg)
	if len(fields) > 0 {
		fmt.Print(" [")
		for i, f := range fields {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%s=%v", f.Key, f.Value)
		}
		fmt.Print("]")
	}
	fmt.Println()
}
