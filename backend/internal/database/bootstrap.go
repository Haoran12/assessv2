package database

import (
	"encoding/json"
	"errors"
	"fmt"

	"assessv2/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	defaultRootUsername = "root"
	defaultRootRealName = "系统Root管理员"
)

func AutoMigrateAndSeed(db *gorm.DB, defaultPassword string) error {
	if err := db.AutoMigrate(
		&model.SystemSetting{},
		&model.User{},
		&model.Role{},
		&model.UserRole{},
		&model.UserOrganization{},
		&model.AuditLog{},
	); err != nil {
		return fmt.Errorf("failed to run migration: %w", err)
	}

	if err := seedSystemRoles(db); err != nil {
		return err
	}
	if err := seedDefaultRootUser(db, defaultPassword); err != nil {
		return err
	}
	return nil
}

func seedSystemRoles(db *gorm.DB) error {
	type roleSeed struct {
		RoleCode    string
		RoleName    string
		Description string
		Permissions []string
		IsSystem    bool
	}

	seeds := []roleSeed{
		{
			RoleCode:    "root",
			RoleName:    "Root管理员",
			Description: "系统最高权限，可管理所有功能",
			Permissions: []string{"*"},
			IsSystem:    true,
		},
		{
			RoleCode:    "viewer",
			RoleName:    "查看者",
			Description: "只能查看考核数据，无修改权限",
			Permissions: []string{"assessment:view", "score:view", "report:view"},
			IsSystem:    true,
		},
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, item := range seeds {
			permissionsJSON, err := json.Marshal(item.Permissions)
			if err != nil {
				return fmt.Errorf("failed to marshal permissions for role %s: %w", item.RoleCode, err)
			}

			var role model.Role
			err = tx.Where("role_code = ?", item.RoleCode).First(&role).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				role = model.Role{
					RoleCode:    item.RoleCode,
					RoleName:    item.RoleName,
					Description: item.Description,
					Permissions: string(permissionsJSON),
					IsSystem:    item.IsSystem,
				}
				if createErr := tx.Create(&role).Error; createErr != nil {
					return fmt.Errorf("failed to create role %s: %w", item.RoleCode, createErr)
				}
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to query role %s: %w", item.RoleCode, err)
			}

			role.RoleName = item.RoleName
			role.Description = item.Description
			role.Permissions = string(permissionsJSON)
			role.IsSystem = item.IsSystem
			if saveErr := tx.Save(&role).Error; saveErr != nil {
				return fmt.Errorf("failed to update role %s: %w", item.RoleCode, saveErr)
			}
		}
		return nil
	})
}

func seedDefaultRootUser(db *gorm.DB, defaultPassword string) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to generate default root password hash: %w", err)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var rootRole model.Role
		if err := tx.Where("role_code = ?", "root").First(&rootRole).Error; err != nil {
			return fmt.Errorf("failed to load root role: %w", err)
		}

		var user model.User
		err := tx.Where("username = ?", defaultRootUsername).Where("deleted_at IS NULL").First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = model.User{
				Username:           defaultRootUsername,
				PasswordHash:       string(passwordHash),
				RealName:           defaultRootRealName,
				Status:             "active",
				MustChangePassword: true,
			}
			if createErr := tx.Create(&user).Error; createErr != nil {
				return fmt.Errorf("failed to create root user: %w", createErr)
			}
		} else if err != nil {
			return fmt.Errorf("failed to query root user: %w", err)
		}

		var userRole model.UserRole
		err = tx.Where("user_id = ? AND role_id = ?", user.ID, rootRole.ID).First(&userRole).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			userRole = model.UserRole{
				UserID:    user.ID,
				RoleID:    rootRole.ID,
				IsPrimary: true,
			}
			if createErr := tx.Create(&userRole).Error; createErr != nil {
				return fmt.Errorf("failed to attach root role: %w", createErr)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to query root role mapping: %w", err)
		}

		if !userRole.IsPrimary {
			userRole.IsPrimary = true
			if saveErr := tx.Save(&userRole).Error; saveErr != nil {
				return fmt.Errorf("failed to set root role as primary: %w", saveErr)
			}
		}
		return nil
	})
}
