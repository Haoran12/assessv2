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
)

func main() {
	action := flag.String("action", "up", "migration action: up | down | status")
	steps := flag.Int("steps", 1, "rollback steps when action=down")
	seed := flag.Bool("seed", true, "seed baseline roles/settings/root user after up")
	flag.Parse()

	cfg := config.Load()
	db, err := database.NewSQLite(cfg.Database)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	manager, err := migration.NewManager(db, cfg.MigrationsDir)
	if err != nil {
		log.Fatalf("failed to initialize migration manager: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	switch strings.ToLower(strings.TrimSpace(*action)) {
	case "up":
		applied, err := manager.Up(ctx)
		if err != nil {
			log.Fatalf("failed to apply migrations: %v", err)
		}
		fmt.Printf("applied migrations: %d\n", applied)

		if *seed {
			if err := database.SeedBaselineData(db, cfg.DefaultPassword); err != nil {
				log.Fatalf("failed to seed baseline data: %v", err)
			}
			fmt.Println("baseline seed completed")
		}
	case "down":
		reverted, err := manager.Down(ctx, *steps)
		if err != nil {
			log.Fatalf("failed to rollback migrations: %v", err)
		}
		fmt.Printf("rolled back migrations: %d\n", reverted)
	case "status":
		statusRows, err := manager.Status(ctx)
		if err != nil {
			log.Fatalf("failed to query migration status: %v", err)
		}
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
	default:
		log.Fatalf("unsupported action=%q, expected up|down|status", *action)
	}
}
