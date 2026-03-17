package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"assessv2/backend/internal/config"
	"assessv2/backend/internal/database"
	"assessv2/backend/internal/migration"
	"gorm.io/gorm"
)

func main() {
	action := flag.String("action", "up", "migration action: up | down | status")
	target := flag.String("target", "all", "migration target: business | accounts | all")
	steps := flag.Int("steps", 1, "rollback steps when action=down")
	seed := flag.Bool("seed", true, "seed baseline roles/settings/root user after up")
	flag.Parse()

	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	normalizedTarget := strings.ToLower(strings.TrimSpace(*target))
	if normalizedTarget != "business" && normalizedTarget != "accounts" && normalizedTarget != "all" {
		log.Fatalf("unsupported target=%q, expected business|accounts|all", *target)
	}
	shouldRunBusiness := normalizedTarget == "business" || normalizedTarget == "all"
	shouldRunAccounts := normalizedTarget == "accounts" || normalizedTarget == "all"

	switch strings.ToLower(strings.TrimSpace(*action)) {
	case "up", "down", "status":
		// covered above
	default:
		log.Fatalf("unsupported action=%q, expected up|down|status", *action)
	}

	businessDB, err := mustOpenDB(cfg.Database, shouldRunBusiness)
	if err != nil {
		log.Fatalf("failed to open business database: %v", err)
	}
	accountsDBConfig := cfg.Database
	accountsDBConfig.Path = cfg.AccountsDatabasePath
	accountsDB, err := mustOpenDB(accountsDBConfig, shouldRunAccounts)
	if err != nil {
		log.Fatalf("failed to open accounts database: %v", err)
	}

	switch strings.ToLower(strings.TrimSpace(*action)) {
	case "up":
		if shouldRunBusiness {
			manager, err := migration.NewManager(businessDB, cfg.BusinessMigrationsDir)
			if err != nil {
				log.Fatalf("failed to initialize business migration manager: %v", err)
			}
			applied, err := manager.Up(ctx)
			if err != nil {
				log.Fatalf("failed to apply business migrations: %v", err)
			}
			fmt.Printf("business applied migrations: %d\n", applied)
		}
		if shouldRunAccounts {
			manager, err := migration.NewManager(accountsDB, cfg.AccountsMigrationsDir)
			if err != nil {
				log.Fatalf("failed to initialize accounts migration manager: %v", err)
			}
			applied, err := manager.Up(ctx)
			if err != nil {
				log.Fatalf("failed to apply accounts migrations: %v", err)
			}
			fmt.Printf("accounts applied migrations: %d\n", applied)
		}

		if *seed {
			if shouldRunBusiness {
				if err := database.SeedAssessmentData(businessDB); err != nil {
					log.Fatalf("failed to seed business baseline data: %v", err)
				}
			}
			if shouldRunAccounts {
				if err := database.SeedAccountsData(accountsDB, cfg.DefaultPassword); err != nil {
					log.Fatalf("failed to seed accounts baseline data: %v", err)
				}
			}
			fmt.Println("baseline seed completed")
		}
	case "down":
		if shouldRunBusiness {
			manager, err := migration.NewManager(businessDB, cfg.BusinessMigrationsDir)
			if err != nil {
				log.Fatalf("failed to initialize business migration manager: %v", err)
			}
			reverted, err := manager.Down(ctx, *steps)
			if err != nil {
				log.Fatalf("failed to rollback business migrations: %v", err)
			}
			fmt.Printf("business rolled back migrations: %d\n", reverted)
		}
		if shouldRunAccounts {
			manager, err := migration.NewManager(accountsDB, cfg.AccountsMigrationsDir)
			if err != nil {
				log.Fatalf("failed to initialize accounts migration manager: %v", err)
			}
			reverted, err := manager.Down(ctx, *steps)
			if err != nil {
				log.Fatalf("failed to rollback accounts migrations: %v", err)
			}
			fmt.Printf("accounts rolled back migrations: %d\n", reverted)
		}
	case "status":
		if shouldRunBusiness {
			manager, err := migration.NewManager(businessDB, cfg.BusinessMigrationsDir)
			if err != nil {
				log.Fatalf("failed to initialize business migration manager: %v", err)
			}
			statusRows, err := manager.Status(ctx)
			if err != nil {
				log.Fatalf("failed to query business migration status: %v", err)
			}
			printMigrationStatus("business", statusRows)
		}
		if shouldRunAccounts {
			manager, err := migration.NewManager(accountsDB, cfg.AccountsMigrationsDir)
			if err != nil {
				log.Fatalf("failed to initialize accounts migration manager: %v", err)
			}
			statusRows, err := manager.Status(ctx)
			if err != nil {
				log.Fatalf("failed to query accounts migration status: %v", err)
			}
			printMigrationStatus("accounts", statusRows)
		}
	}

}

func mustOpenDB(cfg config.DatabaseConfig, required bool) (*gorm.DB, error) {
	if !required {
		return nil, nil
	}
	db, err := database.NewSQLite(cfg)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func printMigrationStatus(label string, statusRows []migration.Status) {
	fmt.Printf("[%s] migration status\n", label)
	if len(statusRows) == 0 {
		fmt.Println("no migration files found")
		return
	}
	for _, row := range statusRows {
		state := "pending"
		if row.Applied {
			state = fmt.Sprintf("applied@%s", time.Unix(row.AppliedAt, 0).Format(time.RFC3339))
		}
		fmt.Printf("%04d %-30s %s\n", row.Version, row.Name, state)
	}
}
