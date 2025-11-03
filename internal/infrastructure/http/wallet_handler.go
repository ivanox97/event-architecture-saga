package http

import (
	"net/http"

	"event-saga/internal/application/wallet"

	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	walletService *wallet.Service
}

func NewWalletHandler(ws *wallet.Service) *WalletHandler {
	return &WalletHandler{
		walletService: ws,
	}
}

func (h *WalletHandler) GetWallet(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	w, err := h.walletService.RebuildWalletState(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":           userID,
		"balance":           w.Balance(),
		"available_balance": w.AvailableBalance(),
	})
}

func (h *WalletHandler) ProcessRefund(c *gin.Context) {
	var req wallet.ProcessRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be greater than 0"})
		return
	}

	if req.PaymentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment_id is required"})
		return
	}

	if err := h.walletService.ProcessRefund(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Refund processed successfully",
		"payment_id": req.PaymentID,
		"amount":     req.Amount,
	})
}

func (h *WalletHandler) AddFunds(c *gin.Context) {
	var req wallet.AddFundsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be greater than 0"})
		return
	}

	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	w, err := h.walletService.RebuildWalletState(c.Request.Context(), req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	previousBalance := w.Balance()

	if err := h.walletService.AddFunds(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Rebuild to get new balance
	w, _ = h.walletService.RebuildWalletState(c.Request.Context(), req.UserID)

	c.JSON(http.StatusOK, gin.H{
		"message":          "Funds added successfully",
		"user_id":          req.UserID,
		"amount":           req.Amount,
		"previous_balance": previousBalance,
		"new_balance":      w.Balance(),
	})
}
