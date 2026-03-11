package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"assessv2/backend/internal/api/router"
	"assessv2/backend/internal/config"
	"assessv2/backend/internal/database"
)

func main() {
	cfg := config.Load()

	db, err := database.NewSQLite(cfg.Database)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}

	if err := database.AutoMigrateAndSeed(db, cfg.DefaultPassword); err != nil {
		log.Fatalf("failed to bootstrap database: %v", err)
	}

	engine := router.New(cfg, db)

	server := &http.Server{
		Addr:              cfg.Server.Address(),
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("backend server listening on %s", cfg.Server.Address())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		return
	}
	log.Println("server stopped")
}
