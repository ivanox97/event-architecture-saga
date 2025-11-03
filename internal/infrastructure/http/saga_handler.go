package http

import (
	"net/http"

	"event-saga/internal/application/saga"

	"github.com/gin-gonic/gin"
)

type SagaHandler struct {
	orchestrator *saga.Orchestrator
}

func NewSagaHandler(o *saga.Orchestrator) *SagaHandler {
	return &SagaHandler{
		orchestrator: o,
	}
}

func (h *SagaHandler) CreateWalletPayment(c *gin.Context) {
	var req saga.CreateWalletPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be greater than 0"})
		return
	}

	resp, err := h.orchestrator.CreateWalletPayment(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *SagaHandler) CreateExternalPayment(c *gin.Context) {
	var req saga.CreateExternalPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be greater than 0"})
		return
	}

	if req.CardToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "card_token is required"})
		return
	}

	resp, err := h.orchestrator.CreateExternalPayment(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *SagaHandler) GetPaymentStatus(c *gin.Context) {
	paymentID := c.Param("id")
	if paymentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment_id is required"})
		return
	}

	status, err := h.orchestrator.GetPaymentStatus(c.Request.Context(), paymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}
