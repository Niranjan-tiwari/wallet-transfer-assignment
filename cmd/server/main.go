package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/candidate/wallet-transfer/internal/config"
	"github.com/candidate/wallet-transfer/internal/db"
	"github.com/candidate/wallet-transfer/internal/handler"
	"github.com/candidate/wallet-transfer/internal/repository"
	"github.com/candidate/wallet-transfer/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	database, err := db.New(cfg.DSN())
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer database.Close()
	log.Println("connected to MySQL")

	redisClient, err := db.NewRedisClient(cfg.RedisAddr)
	if err != nil {
		log.Printf("warning: failed to connect to Redis at %s: %v. Running without Redis fast-path cache.", cfg.RedisAddr, err)
	} else {
		defer redisClient.Close()
		log.Println("connected to Redis")
	}

	walletRepo := repository.NewWalletRepository(database)
	txnRepo := repository.NewTransactionRepository(database)
	ledgerRepo := repository.NewLedgerRepository()

	svc := service.NewTransferService(database, redisClient, walletRepo, txnRepo, ledgerRepo)
	h := handler.New(svc)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	h.RegisterRoutes(r)
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server listening on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("server exited")
}
