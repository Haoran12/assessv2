package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"assessv2/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	defaultRootUsername = "root"
	defaultRootRealName = "System Root"
)

func SeedBaselineData(db *gorm.DB, defaultPassword string) error {
	if err := seedSystemSettings(db); err != nil {
		return err
	}
	if err := seedSystemRoles(db); err != nil {
		return err
	}
	if err := seedDefaultRootUser(db, defaultPassword); err != nil {
		return err
	}
	if err := seedDefaultPositionLevels(db); err != nil {
		return err
	}
	return nil
}

func seedSystemSettings(db *gorm.DB) error {
	now := time.Now().Unix()
	seeds := []model.SystemSetting{
		{
			SettingKey:   "backup.retention_days",
			SettingValue: "7",
			SettingType:  "number",
			Description:  "Backup retention days",
			IsSystem:     true,
			UpdatedAt:    now,
		},
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, item := range seeds {
			var existing model.SystemSetting
			err := tx.Where("setting_key = ?", item.SettingKey).First(&existing).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				if createErr := tx.Create(&item).Error; createErr != nil {
					return fmt.Errorf("failed to create system setting %s: %w", item.SettingKey, createErr)
				}
			case err != nil:
				return fmt.Errorf("failed to query system setting %s: %w", item.SettingKey, err)
			default:
				existing.SettingValue = item.SettingValue
				existing.SettingType = item.SettingType
				existing.Description = item.Description
				existing.IsSystem = item.IsSystem
				existing.UpdatedAt = now
				if saveErr := tx.Save(&existing).Error; saveErr != nil {
					return fmt.Errorf("failed to update system setting %s: %w", item.SettingKey, saveErr)
				}
			}
		}
		return nil
	})
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
			RoleName:    "Root Admin",
			Description: "System super administrator",
			Permissions: []string{"*"},
			IsSystem:    true,
		},
		{
			RoleCode:    "viewer",
			RoleName:    "Viewer",
			Description: "Read-only user",
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

func seedDefaultPositionLevels(db *gorm.DB) error {
	type positionLevelSeed struct {
		LevelCode       string
		LevelName       string
		Description     string
		IsSystem        bool
		IsForAssessment bool
		SortOrder       int
	}

	seeds := []positionLevelSeed{
		{
			LevelCode:       "group_leader",
			LevelName:       "集团高层",
			Description:     "集团层级高管",
			IsSystem:        true,
			IsForAssessment: true,
			SortOrder:       1,
		},
		{
			LevelCode:       "company_leader",
			LevelName:       "企业高层",
			Description:     "权属企业高管",
			IsSystem:        true,
			IsForAssessment: true,
			SortOrder:       2,
		},
		{
			LevelCode:       "manager_main",
			LevelName:       "正职管理人员",
			Description:     "部门正职管理人员",
			IsSystem:        true,
			IsForAssessment: true,
			SortOrder:       3,
		},
		{
			LevelCode:       "manager_deputy",
			LevelName:       "副职管理人员",
			Description:     "部门副职管理人员",
			IsSystem:        true,
			IsForAssessment: true,
			SortOrder:       4,
		},
		{
			LevelCode:       "staff",
			LevelName:       "一般人员",
			Description:     "普通员工",
			IsSystem:        true,
			IsForAssessment: true,
			SortOrder:       5,
		},
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, item := range seeds {
			var existing model.PositionLevel
			err := tx.Where("level_code = ?", item.LevelCode).First(&existing).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				record := model.PositionLevel{
					LevelCode:       item.LevelCode,
					LevelName:       item.LevelName,
					Description:     item.Description,
					IsSystem:        item.IsSystem,
					IsForAssessment: item.IsForAssessment,
					SortOrder:       item.SortOrder,
					Status:          "active",
				}
				if createErr := tx.Create(&record).Error; createErr != nil {
					return fmt.Errorf("failed to create position level %s: %w", item.LevelCode, createErr)
				}
			case err != nil:
				return fmt.Errorf("failed to query position level %s: %w", item.LevelCode, err)
			default:
				existing.LevelName = item.LevelName
				existing.Description = item.Description
				existing.IsSystem = item.IsSystem
				existing.IsForAssessment = item.IsForAssessment
				existing.SortOrder = item.SortOrder
				existing.Status = "active"
				if saveErr := tx.Save(&existing).Error; saveErr != nil {
					return fmt.Errorf("failed to update position level %s: %w", item.LevelCode, saveErr)
				}
			}
		}
		return nil
	})
}
