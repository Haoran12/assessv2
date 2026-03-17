package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	roleCodePattern       = regexp.MustCompile(`^[a-z][a-z0-9:_-]{1,49}$`)
	roleCodeSanitizeChars = regexp.MustCompile(`[^a-z0-9:_-]+`)
)

type UserService struct {
	userRepo        *repository.UserRepository
	roleRepo        *repository.RoleRepository
	userRoleRepo    *repository.UserRoleRepository
	auditRepo       *repository.AuditRepository
	defaultPassword string
}

type ListUsersInput struct {
	Page     int
	PageSize int
	Keyword  string
	Status   string
}

type UserListItem struct {
	ID                 uint                     `json:"id"`
	Username           string                   `json:"username"`
	RealName           string                   `json:"realName"`
	Status             string                   `json:"status"`
	MustChangePassword bool                     `json:"mustChangePassword"`
	LastLoginAt        *int64                   `json:"lastLoginAt,omitempty"`
	LastLoginIP        *string                  `json:"lastLoginIp,omitempty"`
	Roles              []string                 `json:"roles"`
	RoleNames          []string                 `json:"roleNames"`
	PrimaryRole        string                   `json:"primaryRole"`
	Organizations      []auth.OrganizationScope `json:"organizations"`
	PermissionBindings []auth.PermissionBinding `json:"permissionBindings"`
	CreatedAt          int64                    `json:"createdAt"`
	UpdatedAt          int64                    `json:"updatedAt"`
}

type ListUsersOutput struct {
	Items    []UserListItem `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

type UserGroupItem struct {
	ID          uint   `json:"id"`
	RoleCode    string `json:"roleCode"`
	RoleName    string `json:"roleName"`
	Description string `json:"description"`
	IsSystem    bool   `json:"isSystem"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type CreateUserGroupInput struct {
	RoleCode    string
	RoleName    string
	Description string
}

type UpdateUserGroupInput struct {
	RoleCode    string
	RoleName    string
	Description string
}

type UpdateUserGroupsInput struct {
	RoleIDs       []uint
	PrimaryRoleID uint
}

func NewUserService(
	userRepo *repository.UserRepository,
	roleRepo *repository.RoleRepository,
	userRoleRepo *repository.UserRoleRepository,
	auditRepo *repository.AuditRepository,
	defaultPassword string,
) *UserService {
	return &UserService{
		userRepo:        userRepo,
		roleRepo:        roleRepo,
		userRoleRepo:    userRoleRepo,
		auditRepo:       auditRepo,
		defaultPassword: defaultPassword,
	}
}

func (s *UserService) ListUsers(ctx context.Context, input ListUsersInput) (*ListUsersOutput, error) {
	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	users, total, err := s.userRepo.List(ctx, repository.UserListFilter{
		Keyword: strings.TrimSpace(input.Keyword),
		Status:  strings.TrimSpace(input.Status),
		Offset:  (page - 1) * pageSize,
		Limit:   pageSize,
	})
	if err != nil {
		return nil, err
	}

	items := make([]UserListItem, 0, len(users))
	for _, user := range users {
		primaryRole, roleCodes, _, orgScopes, bindings, identityErr := extractIdentity(&user)
		if identityErr != nil {
			return nil, identityErr
		}
		items = append(items, UserListItem{
			ID:                 user.ID,
			Username:           user.Username,
			RealName:           user.RealName,
			Status:             user.Status,
			MustChangePassword: user.MustChangePassword,
			LastLoginAt:        user.LastLoginAt,
			LastLoginIP:        user.LastLoginIP,
			Roles:              roleCodes,
			RoleNames:          collectRoleNames(&user),
			PrimaryRole:        primaryRole,
			Organizations:      orgScopes,
			PermissionBindings: bindings,
			CreatedAt:          user.CreatedAt,
			UpdatedAt:          user.UpdatedAt,
		})
	}

	return &ListUsersOutput{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *UserService) ListUserGroups(ctx context.Context) ([]UserGroupItem, error) {
	roles, err := s.roleRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]UserGroupItem, 0, len(roles))
	for _, role := range roles {
		items = append(items, mapRoleToGroupItem(role))
	}
	return items, nil
}

func (s *UserService) CreateUserGroup(
	ctx context.Context,
	operatorID uint,
	input CreateUserGroupInput,
	ipAddress string,
	userAgent string,
) (*UserGroupItem, error) {
	roleName := strings.TrimSpace(input.RoleName)
	if roleName == "" {
		return nil, ErrInvalidRoleName
	}
	roleCode, err := normalizeRoleCode(input.RoleCode, roleName)
	if err != nil {
		return nil, err
	}

	if _, err = s.roleRepo.GetByCode(ctx, roleCode); err == nil {
		return nil, ErrRoleCodeExists
	} else if !repository.IsRecordNotFound(err) {
		return nil, err
	}

	permissionsJSON, _ := json.Marshal([]string{})
	role := model.Role{
		RoleCode:    roleCode,
		RoleName:    roleName,
		Description: strings.TrimSpace(input.Description),
		Permissions: string(permissionsJSON),
		IsSystem:    false,
	}
	if err := s.roleRepo.Create(ctx, &role); err != nil {
		return nil, err
	}

	operator := operatorID
	targetID := role.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "create", "roles", &targetID, map[string]any{
		"event":       "create_user_group",
		"roleCode":    role.RoleCode,
		"roleName":    role.RoleName,
		"description": role.Description,
	}, ipAddress, userAgent))

	item := mapRoleToGroupItem(role)
	return &item, nil
}

func (s *UserService) UpdateUserGroup(
	ctx context.Context,
	operatorID uint,
	roleID uint,
	input UpdateUserGroupInput,
	ipAddress string,
	userAgent string,
) (*UserGroupItem, error) {
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	if role.IsSystem {
		return nil, ErrSystemRoleLocked
	}

	roleName := strings.TrimSpace(input.RoleName)
	if roleName == "" {
		return nil, ErrInvalidRoleName
	}

	roleCode := strings.TrimSpace(input.RoleCode)
	if roleCode == "" {
		roleCode = role.RoleCode
	}
	roleCode, err = normalizeRoleCode(roleCode, roleName)
	if err != nil {
		return nil, err
	}

	if roleCode != role.RoleCode {
		if _, err = s.roleRepo.GetByCode(ctx, roleCode); err == nil {
			return nil, ErrRoleCodeExists
		} else if !repository.IsRecordNotFound(err) {
			return nil, err
		}
	}

	role.RoleCode = roleCode
	role.RoleName = roleName
	role.Description = strings.TrimSpace(input.Description)
	role.UpdatedAt = time.Now().Unix()
	if err := s.roleRepo.Save(ctx, role); err != nil {
		return nil, err
	}

	operator := operatorID
	targetID := role.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "roles", &targetID, map[string]any{
		"event":       "update_user_group",
		"roleCode":    role.RoleCode,
		"roleName":    role.RoleName,
		"description": role.Description,
	}, ipAddress, userAgent))

	item := mapRoleToGroupItem(*role)
	return &item, nil
}

func (s *UserService) DeleteUserGroup(ctx context.Context, operatorID, roleID uint, ipAddress, userAgent string) error {
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if repository.IsRecordNotFound(err) {
			return ErrRoleNotFound
		}
		return err
	}
	if role.IsSystem {
		return ErrSystemRoleLocked
	}

	inUse, err := s.userRoleRepo.ExistsByRoleID(ctx, roleID)
	if err != nil {
		return err
	}
	if inUse {
		return ErrRoleInUse
	}

	if err := s.roleRepo.DeleteByID(ctx, roleID); err != nil {
		if repository.IsRecordNotFound(err) {
			return ErrRoleNotFound
		}
		return err
	}

	operator := operatorID
	targetID := roleID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "delete", "roles", &targetID, map[string]any{
		"event":    "delete_user_group",
		"roleCode": role.RoleCode,
		"roleName": role.RoleName,
	}, ipAddress, userAgent))
	return nil
}

func (s *UserService) UpdateUserGroups(
	ctx context.Context,
	operatorID uint,
	targetUserID uint,
	input UpdateUserGroupsInput,
	ipAddress string,
	userAgent string,
) error {
	roleIDs := normalizeRoleIDs(input.RoleIDs)
	if len(roleIDs) == 0 {
		return ErrInvalidRoleList
	}

	roles, err := s.roleRepo.ListByIDs(ctx, roleIDs)
	if err != nil {
		return err
	}
	if len(roles) != len(roleIDs) {
		return ErrInvalidRoleList
	}

	targetUser, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}

	rootRole, err := s.roleRepo.GetByCode(ctx, "root")
	if err != nil && !repository.IsRecordNotFound(err) {
		return err
	}
	if targetUser.Username == "root" && (rootRole == nil || !containsUint(roleIDs, rootRole.ID)) {
		return ErrCannotDemoteRoot
	}

	primaryRoleID := input.PrimaryRoleID
	if primaryRoleID == 0 || !containsUint(roleIDs, primaryRoleID) {
		primaryRoleID = roleIDs[0]
	}

	operator := operatorID
	if err := s.userRoleRepo.ReplaceForUser(ctx, targetUserID, roleIDs, primaryRoleID, &operator); err != nil {
		return err
	}

	targetID := targetUserID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "users", &targetID, map[string]any{
		"event":         "update_user_groups",
		"userID":        targetUserID,
		"roleIDs":       roleIDs,
		"primaryRoleID": primaryRoleID,
	}, ipAddress, userAgent))
	return nil
}

func (s *UserService) ResetPassword(ctx context.Context, operatorID, targetUserID uint, newPassword, ipAddress, userAgent string) error {
	if strings.TrimSpace(newPassword) == "" {
		newPassword = s.defaultPassword
	}
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, targetUserID, string(hashBytes), true); err != nil {
		return err
	}

	targetID := targetUserID
	operator := operatorID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "users", &targetID, map[string]any{
		"event":  "reset_password",
		"target": strconv.FormatUint(uint64(targetUserID), 10),
	}, ipAddress, userAgent))
	return nil
}

func (s *UserService) UpdateStatus(ctx context.Context, operatorID, targetUserID uint, status string, ipAddress, userAgent string) error {
	switch status {
	case "active", "inactive", "locked":
	default:
		return ErrInvalidUserStatus
	}
	if operatorID == targetUserID && status != "active" {
		return ErrCannotDisableSelf
	}

	if err := s.userRepo.UpdateStatus(ctx, targetUserID, status); err != nil {
		return err
	}

	targetID := targetUserID
	operator := operatorID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "users", &targetID, map[string]any{
		"event":  "update_status",
		"status": status,
	}, ipAddress, userAgent))
	return nil
}

func mapRoleToGroupItem(role model.Role) UserGroupItem {
	return UserGroupItem{
		ID:          role.ID,
		RoleCode:    role.RoleCode,
		RoleName:    role.RoleName,
		Description: role.Description,
		IsSystem:    role.IsSystem,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	}
}

func normalizeRoleCode(roleCode, roleName string) (string, error) {
	code := strings.ToLower(strings.TrimSpace(roleCode))
	if code == "" {
		nameToken := strings.ToLower(strings.TrimSpace(roleName))
		nameToken = strings.ReplaceAll(nameToken, " ", "-")
		nameToken = roleCodeSanitizeChars.ReplaceAllString(nameToken, "")
		if nameToken == "" {
			nameToken = fmt.Sprintf("group_%d", time.Now().Unix())
		}
		code = nameToken
	}

	code = roleCodeSanitizeChars.ReplaceAllString(code, "")
	if len(code) > 50 {
		code = code[:50]
	}
	if code == "" || code[0] < 'a' || code[0] > 'z' {
		code = "g" + code
	}

	if !roleCodePattern.MatchString(code) {
		return "", ErrInvalidRoleCode
	}
	return code, nil
}

func collectRoleNames(user *model.User) []string {
	names := make([]string, 0, len(user.UserRoles))
	seen := make(map[string]struct{}, len(user.UserRoles))
	appendRoleName := func(name string) {
		if name == "" {
			return
		}
		if _, exists := seen[name]; exists {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	for _, userRole := range user.UserRoles {
		if userRole.IsPrimary {
			appendRoleName(userRole.Role.RoleName)
		}
	}
	for _, userRole := range user.UserRoles {
		appendRoleName(userRole.Role.RoleName)
	}
	return names
}

func normalizeRoleIDs(roleIDs []uint) []uint {
	normalized := make([]uint, 0, len(roleIDs))
	seen := make(map[uint]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		if roleID == 0 {
			continue
		}
		if _, exists := seen[roleID]; exists {
			continue
		}
		seen[roleID] = struct{}{}
		normalized = append(normalized, roleID)
	}
	return normalized
}

func containsUint(list []uint, target uint) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
