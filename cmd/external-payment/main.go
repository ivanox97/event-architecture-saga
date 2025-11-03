package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"event-saga/internal/application/externalpayment"
	"event-saga/internal/common/configs"
	"event-saga/internal/common/logger"
	"event-saga/internal/domain/events"
	"event-saga/internal/infrastructure/dlq"
	"event-saga/internal/infrastructure/eventbus"
	eventstore "event-saga/internal/infrastructure/eventstore"
	"event-saga/internal/infrastructure/mock"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	port := configs.PortExternalPaymentService

	l := logger.NewMockLogger()

	eventStore, err := eventstore.NewPostgresEventStore(configs.GetDatabaseURL())
	if err != nil {
		l.Error("Failed to initialize event store", logger.Field{Key: "error", Value: err})
		os.Exit(1)
	}
	defer eventStore.Close()

	eventBus, err := eventbus.NewEventBus()
	if err != nil {
		l.Error("Failed to initialize event bus", logger.Field{Key: "error", Value: err})
		os.Exit(1)
	}
	defer eventBus.Close()

	dlqService := dlq.NewDLQSimulator()
	defer dlqService.Close()

	gateway := mock.NewMockExternalGateway()

	externalService := externalpayment.NewService(eventStore, eventBus, dlqService, gateway, l)

	router := setupRouter(l)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startEventConsumers(ctx, externalService, eventBus, l)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	l.Info("Starting external payment service", logger.Field{Key: "port", Value: port})

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

func setupRouter(l logger.Logger) *gin.Engine {
	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	return router
}

func startEventConsumers(ctx context.Context, externalService *externalpayment.Service, eventBus eventbus.EventBus, l logger.Logger) {
	eventBus.SubscribeWithGroupID(ctx, configs.TopicPayments, configs.ServiceNameExternalPaymentService, func(ctx context.Context, event events.Event) error {
		// Only process ExternalPaymentRequested events
		if event.Type() == "ExternalPaymentRequested" {
			return externalService.HandleExternalPaymentRequested(ctx, event)
		}
		return nil
	})

	l.Info("Event consumers started")
}
