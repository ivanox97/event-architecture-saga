package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"event-saga/internal/application/saga"
	"event-saga/internal/common/configs"
	"event-saga/internal/common/logger"
	"event-saga/internal/domain/events"
	"event-saga/internal/infrastructure/eventbus"
	"event-saga/internal/infrastructure/eventstore"
	httphandler "event-saga/internal/infrastructure/http"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// Load configuration
	dbURL := configs.GetDatabaseURL()
	port := configs.PortSagaOrchestrator

	// Initialize logger
	l := logger.NewMockLogger()

	// Initialize database
	db, err := initPostgreSQL(dbURL)
	if err != nil {
		l.Error("Failed to initialize database", logger.Field{Key: "error", Value: err})
		os.Exit(1)
	}
	defer db.Close()

	// Initialize Event Store
	eventStore, err := eventstore.NewPostgresEventStore(dbURL)
	if err != nil {
		l.Error("Failed to initialize event store", logger.Field{Key: "error", Value: err})
		os.Exit(1)
	}
	defer eventStore.Close()

	// Initialize Event Bus
	eventBus, err := eventbus.NewEventBus()
	if err != nil {
		l.Error("Failed to initialize event bus", logger.Field{Key: "error", Value: err})
		os.Exit(1)
	}
	defer eventBus.Close()

	// Initialize Orchestrator (no saga repository - using Event Sourcing)
	orchestrator := saga.NewOrchestrator(eventStore, eventBus, l)

	// Initialize HTTP Handlers
	sagaHandler := httphandler.NewSagaHandler(orchestrator)

	// Setup HTTP router
	router := setupRouter(sagaHandler, l)

	// Start event consumers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startEventConsumers(ctx, orchestrator, eventBus, l)

	// Start HTTP server
	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	l.Info("Starting saga orchestrator", logger.Field{Key: "port", Value: port})

	// Graceful shutdown
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.Error("Server failed", logger.Field{Key: "error", Value: err})
		}
	}()

	// Wait for interrupt signal
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

func setupRouter(sagaHandler *httphandler.SagaHandler, l logger.Logger) *gin.Engine {
	router := gin.Default()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// API routes
	v1 := router.Group("/api/payments")
	{
		v1.POST("/wallet", sagaHandler.CreateWalletPayment)
		v1.POST("/creditcard", sagaHandler.CreateExternalPayment)
	}

	// Status endpoint
	router.GET("/api/v1/payments/:id", sagaHandler.GetPaymentStatus)

	return router
}

func startEventConsumers(ctx context.Context, orchestrator *saga.Orchestrator, eventBus eventbus.EventBus, l logger.Logger) {
	// Subscribe to payment events with service-specific consumer group ID
	// Note: Orchestrator only processes RESPONSE events (FundsDebited, PaymentGatewayResponse, etc.)
	// It ignores WalletPaymentRequested and ExternalPaymentRequested because it publishes those itself
	eventBus.SubscribeWithGroupID(ctx, configs.TopicPayments, configs.ServiceNameSagaOrchestrator, func(ctx context.Context, event events.Event) error {
		return orchestrator.ProcessEvent(ctx, event)
	})

	l.Info("Event consumers started")
}
