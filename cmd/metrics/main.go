package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"event-saga/internal/application/metrics"
	"event-saga/internal/common/configs"
	"event-saga/internal/common/logger"
	commonmetrics "event-saga/internal/common/metrics"
	"event-saga/internal/domain/events"
	"event-saga/internal/infrastructure/dlq"
	"event-saga/internal/infrastructure/errors"
	"event-saga/internal/infrastructure/eventbus"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	port := configs.PortMetricsService
	dbURL := configs.GetDatabaseURL()

	l := logger.NewMockLogger()
	m := commonmetrics.NewMockCollector()

	// Initialize database for DB Errors
	db, err := initPostgreSQL(dbURL)
	if err != nil {
		l.Error("Failed to initialize database", logger.Field{Key: "error", Value: err})
		os.Exit(1)
	}
	defer db.Close()

	// Initialize DB Errors service
	dbErrors := errors.NewDBErrors(db)

	// Initialize DLQ
	dlqService := dlq.NewDLQSimulator()
	defer dlqService.Close()

	eventBus, err := eventbus.NewEventBus()
	if err != nil {
		l.Error("Failed to initialize event bus", logger.Field{Key: "error", Value: err})
		os.Exit(1)
	}
	defer eventBus.Close()

	// Initialize Metrics Service with DB Errors
	metricsService := metrics.NewService(eventBus, dlqService, dbErrors, m, l)

	router := setupRouter(metricsService, l)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startEventConsumers(ctx, metricsService, eventBus, dlqService, l)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	l.Info("Starting metrics service", logger.Field{Key: "port", Value: port})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.Error("Server failed", logger.Field{Key: "error", Value: err})
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	l.Info("Shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		l.Error("Server forced to shutdown", logger.Field{Key: "error", Value: err})
	}
}

func setupRouter(metricsService *metrics.Service, l logger.Logger) *gin.Engine {
	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	return router
}

func startEventConsumers(ctx context.Context, metricsService *metrics.Service, eventBus eventbus.EventBus, dlqService dlq.DLQ, l logger.Logger) {
	// Subscribe to all payment events
	eventBus.SubscribeWithGroupID(ctx, configs.TopicPayments, configs.ServiceNameMetricsService, func(ctx context.Context, event events.Event) error {
		// Handle different event types based on event.Type()
		switch event.Type() {
		// Wallet payment events
		case "WalletPaymentCompleted":
			return metricsService.HandleWalletPaymentCompleted(ctx, event)
		case "WalletPaymentFailed":
			return metricsService.HandleWalletPaymentFailed(ctx, event)
		case "WalletPaymentRequested":
			return metricsService.HandleWalletPaymentRequested(ctx, event)
		// External payment events
		case "ExternalPaymentCompleted":
			return metricsService.HandleExternalPaymentCompleted(ctx, event)
		case "ExternalPaymentFailed":
			return metricsService.HandleExternalPaymentFailed(ctx, event)
		case "ExternalPaymentRequested":
			return metricsService.HandleExternalPaymentRequested(ctx, event)
		}
		return nil
	})

	// Subscribe to DLQ events
	if dlqService != nil {
		dlqService.Subscribe(ctx, func(ctx context.Context, dlqEvent dlq.DLQEvent) error {
			return metricsService.HandleDLQEvent(ctx, dlqEvent)
		})
		l.Info("DLQ consumer started")
	}

	l.Info("Event consumers started")
}

func initPostgreSQL(connString string) (*sql.DB, error) {
	db, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
