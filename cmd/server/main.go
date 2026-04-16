package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"recommendation-service/internal/cache"
	"recommendation-service/internal/config"
	"recommendation-service/internal/handler"
	"recommendation-service/internal/model"
	"recommendation-service/internal/repository"
	"recommendation-service/internal/service"
)

func main() {
	// Load config
	cfg := config.Load()

	// Connect PostgreSQL
	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Connect Redis
	redisClient := cache.NewRedisClient(cfg.RedisURL)
	defer redisClient.Close()

	// Wire up layers (Dependency Injection)
	repo := repository.New(db)
	cacheLayer := cache.New(redisClient)
	modelClient := model.NewClient()
	svc := service.New(repo, cacheLayer, modelClient)
	h := handler.New(svc)

	// Setup router
	router := h.Routes()

	// HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("server starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-done
	log.Println("server shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server stopped")
}
