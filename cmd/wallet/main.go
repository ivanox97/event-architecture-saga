package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"event-saga/internal/application/wallet"
	"event-saga/internal/common/configs"
	"event-saga/internal/common/logger"
	"event-saga/internal/domain/events"
	"event-saga/internal/infrastructure/eventbus"
	"event-saga/internal/infrastructure/eventstore"
	httphandler "event-saga/internal/infrastructure/http"

	"github.com/gin-gonic/gin"
)

func main() {
	dbURL := configs.GetDatabaseURL()
	port := configs.PortWalletService

	l := logger.NewMockLogger()

	eventStore, err := eventstore.NewPostgresEventStore(dbURL)
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

	walletService := wallet.NewService(eventStore, eventBus, l)

	walletHandler := httphandler.NewWalletHandler(walletService)

	router := setupRouter(walletHandler, l)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startEventConsumers(ctx, walletService, eventBus, l)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	l.Info("Starting wallet service", logger.Field{Key: "port", Value: port})

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

func setupRouter(walletHandler *httphandler.WalletHandler, l logger.Logger) *gin.Engine {
	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	router.GET("/internal/wallet/:user_id", walletHandler.GetWallet)
	router.POST("/internal/wallet/refund", walletHandler.ProcessRefund)

	return router
}

func startEventConsumers(ctx context.Context, walletService *wallet.Service, eventBus eventbus.EventBus, l logger.Logger) {
	// Use service-specific consumer group ID for wallet service
	eventBus.SubscribeWithGroupID(ctx, configs.TopicPayments, configs.ServiceNameWalletService, func(ctx context.Context, event events.Event) error {
		// Only process WalletPaymentRequested events
		if event.Type() == "WalletPaymentRequested" {
			return walletService.HandleWalletPaymentRequested(ctx, event)
		}
		return nil
	})

	l.Info("Event consumers started")
}
