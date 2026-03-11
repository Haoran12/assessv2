package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"assessv2/backend/internal/config"
	"assessv2/backend/internal/database"
	"assessv2/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg := config.Load()

	username := flag.String("username", "root", "target username")
	password := flag.String("password", cfg.DefaultPassword, "new password")
	mustChange := flag.Bool("must-change", false, "force user to change password on next login")
	flag.Parse()

	db, err := database.NewSQLite(cfg.Database)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}

	now := time.Now().Unix()
	result := db.Model(&model.User{}).
		Where("username = ? AND deleted_at IS NULL", *username).
		Updates(map[string]any{
			"password_hash":        string(hashBytes),
			"must_change_password": *mustChange,
			"updated_at":           now,
		})
	if result.Error != nil {
		log.Fatalf("failed to update password: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		log.Fatalf("user %q not found", *username)
	}

	fmt.Printf("password updated for user=%s, must_change_password=%v\n", *username, *mustChange)
}
