package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/candidate/wallet-transfer/internal/domain"
	"github.com/candidate/wallet-transfer/internal/service"
	"github.com/candidate/wallet-transfer/internal/utils"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *service.TransferService
}

func New(svc *service.TransferService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1")

	wallets := v1.Group("/wallets")
	{
		wallets.POST("", h.CreateWallet)
		wallets.GET("/:id", h.GetWallet)
		wallets.GET("/:id/ledger", h.GetLedger)
	}

	transfers := v1.Group("/transfers")
	{
		transfers.POST("", h.CreateTransfer)
		transfers.GET("/:id", h.GetTransfer)
	}
}

func (h *Handler) CreateWallet(c *gin.Context) {
	var req service.CreateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
		return
	}

	wallet, err := h.svc.CreateWallet(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"wallet": wallet})
}

func (h *Handler) GetWallet(c *gin.Context) {
	id, err := utils.ParseUint(c, "id")
	if err != nil {
		return
	}

	wallet, err := h.svc.GetWallet(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrWalletNotFound) {
			c.JSON(http.StatusNotFound, utils.ErrorResponse("wallet not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"wallet": wallet})
}

func (h *Handler) GetLedger(c *gin.Context) {
	id, err := utils.ParseUint(c, "id")
	if err != nil {
		return
	}

	limit := utils.QueryInt(c, "limit", 20)
	var cursor uint64
	if cursorStr := c.Query("cursor"); cursorStr != "" {
		if parsed, err := strconv.ParseUint(cursorStr, 10, 64); err == nil {
			cursor = parsed
		}
	}
	
	if limit > 100 {
		limit = 100
	}

	entries, err := h.svc.GetLedger(c.Request.Context(), id, limit, cursor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
		return
	}
	
	var nextCursor uint64
	if len(entries) > 0 {
		nextCursor = entries[len(entries)-1].ID
	}
	
	c.JSON(http.StatusOK, gin.H{"entries": entries, "limit": limit, "next_cursor": nextCursor})
}

func (h *Handler) CreateTransfer(c *gin.Context) {
	var req service.TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
		return
	}

	resp, err := h.svc.Transfer(c.Request.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInsufficientFunds):
			c.JSON(http.StatusUnprocessableEntity, utils.ErrorResponse(err.Error()))
		case errors.Is(err, domain.ErrSameWallet),
			errors.Is(err, domain.ErrInvalidAmount),
			errors.Is(err, domain.ErrCurrencyMismatch):
			c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
		case errors.Is(err, domain.ErrWalletNotFound):
			c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
		}
		return
	}

	status := http.StatusCreated
	c.JSON(status, resp)
}

func (h *Handler) GetTransfer(c *gin.Context) {
	id, err := utils.ParseUint(c, "id")
	if err != nil {
		return
	}

	txn, err := h.svc.GetTransaction(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrTransactionNotFound) {
			c.JSON(http.StatusNotFound, utils.ErrorResponse("transaction not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"transaction": txn})
}
