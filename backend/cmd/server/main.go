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
	"assessv2/backend/internal/migration"
)

func main() {
	cfg := config.Load()

	businessDB, err := database.NewSQLite(cfg.Database)
	if err != nil {
		log.Fatalf("failed to initialize business database: %v", err)
	}

	accountsDBConfig := cfg.Database
	accountsDBConfig.Path = cfg.AccountsDatabasePath
	accountsDB, err := database.NewSQLite(accountsDBConfig)
	if err != nil {
		log.Fatalf("failed to initialize accounts database: %v", err)
	}

	businessMigrationManager, err := migration.NewManager(businessDB, cfg.BusinessMigrationsDir)
	if err != nil {
		log.Fatalf("failed to initialize business migration manager: %v", err)
	}
	applied, err := businessMigrationManager.Up(context.Background())
	if err != nil {
		log.Fatalf("failed to apply business database migrations: %v", err)
	}
	log.Printf("business database migrations applied=%d", applied)

	accountsMigrationManager, err := migration.NewManager(accountsDB, cfg.AccountsMigrationsDir)
	if err != nil {
		log.Fatalf("failed to initialize accounts migration manager: %v", err)
	}
	applied, err = accountsMigrationManager.Up(context.Background())
	if err != nil {
		log.Fatalf("failed to apply accounts database migrations: %v", err)
	}
	log.Printf("accounts database migrations applied=%d", applied)

	if err := database.SeedAssessmentData(businessDB); err != nil {
		log.Fatalf("failed to seed assessment baseline data: %v", err)
	}
	if err := database.SeedAccountsData(accountsDB, cfg.DefaultPassword); err != nil {
		log.Fatalf("failed to seed accounts baseline data: %v", err)
	}

	engine := router.NewWithDatabases(cfg, businessDB, accountsDB)

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
