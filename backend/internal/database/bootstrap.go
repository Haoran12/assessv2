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
)

func SeedBaselineData(db *gorm.DB, defaultPassword string) error {
	if err := SeedAssessmentData(db); err != nil {
		return err
	}
	if err := SeedAccountsData(db, defaultPassword); err != nil {
		return err
	}
	return nil
}

func SeedAssessmentData(db *gorm.DB) error {
	if err := seedSystemSettings(db); err != nil {
		return err
	}
	if err := seedDefaultPositionLevels(db); err != nil {
		return err
	}
	if err := seedDefaultAssessmentCategories(db); err != nil {
		return err
	}
	return nil
}

func SeedAccountsData(db *gorm.DB, defaultPassword string) error {
	if err := seedSystemRoles(db); err != nil {
		return err
	}
	if err := seedDefaultRootUser(db, defaultPassword); err != nil {
		return err
	}
	return nil
}

func seedSystemSettings(db *gorm.DB) error {
	now := time.Now().Unix()
	seeds := []model.SystemSetting{
		{
			SettingKey:   "system.name",
			SettingValue: "AssessV2",
			SettingType:  "string",
			Description:  "System display name",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "system.logo",
			SettingValue: "",
			SettingType:  "string",
			Description:  "System logo URI or base64 content",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "system.timezone",
			SettingValue: "Asia/Shanghai",
			SettingType:  "string",
			Description:  "System timezone",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "score.decimal_places",
			SettingValue: "2",
			SettingType:  "number",
			Description:  "Score display decimal places",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "assessment.ranking_rule",
			SettingValue: "dense",
			SettingType:  "string",
			Description:  "Ranking rule strategy",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "vote.deadline_time",
			SettingValue: "18:00",
			SettingType:  "string",
			Description:  "Default vote deadline time",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "vote.grade_scores",
			SettingValue: `{"excellent":100,"good":85,"average":70,"poor":60}`,
			SettingType:  "json",
			Description:  "Base score mapping for vote grade options",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "security.password_policy",
			SettingValue: `{"minLength":8,"requireUpper":true,"requireLower":true,"requireNumber":true,"requireSymbol":true}`,
			SettingType:  "json",
			Description:  "Password complexity policy",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "security.session_timeout_minutes",
			SettingValue: "120",
			SettingType:  "number",
			Description:  "Session timeout in minutes",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "audit.retention_days",
			SettingValue: "180",
			SettingType:  "number",
			Description:  "Audit log retention days",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "backup.auto_enabled",
			SettingValue: "true",
			SettingType:  "boolean",
			Description:  "Whether auto backup is enabled",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "backup.auto_time",
			SettingValue: "02:00",
			SettingType:  "string",
			Description:  "Auto backup time in 24-hour format",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "backup.retention_days",
			SettingValue: "7",
			SettingType:  "number",
			Description:  "Backup retention days",
			IsSystem:     true,
			UpdatedAt:    now,
		},
		{
			SettingKey:   "backup.max_count",
			SettingValue: "30",
			SettingType:  "number",
			Description:  "Maximum backup file count",
			IsSystem:     true,
			UpdatedAt:    now,
		},
	}

	return upsertSystemSettings(db, seeds, true)
}

func upsertSystemSettings(db *gorm.DB, seeds []model.SystemSetting, preserveValue bool) error {
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
				if !preserveValue {
					existing.SettingValue = item.SettingValue
				}
				existing.SettingType = item.SettingType
				existing.Description = item.Description
				existing.IsSystem = item.IsSystem
				existing.UpdatedAt = item.UpdatedAt
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
			RoleName:    "root",
			Description: "系统超级管理员，拥有全部功能权限与全局数据访问权限",
			Permissions: []string{"*"},
			IsSystem:    true,
		},
		{
			RoleCode:    "assessment_admin",
			RoleName:    "Admin",
			Description: "考核管理员，可在授权组织范围内维护组织、规则、考核与评分等业务数据",
			Permissions: []string{
				"assessment:view", "assessment:update",
				"rule:view", "rule:update",
				"org:view", "org:update",
				"backup:view", "backup:update",
				"audit:view", "audit:rollback",
				"setting:view", "setting:update",
			},
			IsSystem: true,
		},
		{
			RoleCode:    "leader",
			RoleName:    "Leader",
			Description: "领导角色，可查看授权范围内结果并执行投票相关操作",
			Permissions: []string{
				"assessment:view", "rule:view",
			},
			IsSystem: true,
		},
		{
			RoleCode:    "staff",
			RoleName:    "Staff",
			Description: "员工角色，可查看与本人相关的数据并执行基础投票操作",
			Permissions: []string{
				"assessment:view", "rule:view",
			},
			IsSystem: true,
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
		{LevelCode: "leadership_main", LevelName: "Leadership Main", Description: "Assessment category: leadership main", IsSystem: true, IsForAssessment: true, SortOrder: 1},
		{LevelCode: "leadership_deputy", LevelName: "Leadership Deputy", Description: "Assessment category: leadership deputy", IsSystem: true, IsForAssessment: true, SortOrder: 2},
		{LevelCode: "department_main", LevelName: "Department Main", Description: "Assessment category: department main", IsSystem: true, IsForAssessment: true, SortOrder: 3},
		{LevelCode: "department_deputy", LevelName: "Department Deputy", Description: "Assessment category: department deputy", IsSystem: true, IsForAssessment: true, SortOrder: 4},
		{LevelCode: "general_management_personnel", LevelName: "General Staff", Description: "Assessment category: general staff", IsSystem: true, IsForAssessment: true, SortOrder: 5},
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
func seedDefaultAssessmentCategories(db *gorm.DB) error {
	_ = db
	return nil
}
