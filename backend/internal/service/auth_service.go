package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const defaultTokenTTL = 24 * time.Hour

type AuthService struct {
	userRepo                  *repository.UserRepository
	auditRepo                 *repository.AuditRepository
	jwtSecret                 string
	tokenTTL                  time.Duration
	enforceMustChangePassword bool
}

type LoginResult struct {
	Token              string               `json:"token"`
	TokenType          string               `json:"tokenType"`
	ExpiresIn          int64                `json:"expiresIn"`
	MustChangePassword bool                 `json:"mustChangePassword"`
	User               AuthenticatedUserDTO `json:"user"`
}

type AuthenticatedUserDTO struct {
	ID                 uint                     `json:"id"`
	Username           string                   `json:"username"`
	Role               string                   `json:"role"`
	Roles              []string                 `json:"roles"`
	Permissions        []string                 `json:"permissions"`
	Organizations      []auth.OrganizationScope `json:"organizations"`
	PermissionBindings []auth.PermissionBinding `json:"permissionBindings"`
}

func NewAuthService(
	userRepo *repository.UserRepository,
	auditRepo *repository.AuditRepository,
	jwtSecret string,
	enforceMustChangePassword bool,
) *AuthService {
	return &AuthService{
		userRepo:                  userRepo,
		auditRepo:                 auditRepo,
		jwtSecret:                 jwtSecret,
		tokenTTL:                  defaultTokenTTL,
		enforceMustChangePassword: enforceMustChangePassword,
	}
}

func (s *AuthService) Login(ctx context.Context, username, password, ipAddress, userAgent string) (*LoginResult, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if user.Status == "inactive" {
		return nil, ErrAccountInactive
	}
	if user.Status == "locked" {
		return nil, ErrAccountLocked
	}

	if compareErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); compareErr != nil {
		return nil, ErrInvalidCredentials
	}

	primaryRole, roles, permissions, orgScopes, bindings, err := extractIdentity(user)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	claims := auth.Claims{
		UserID:             user.ID,
		Username:           user.Username,
		Roles:              roles,
		Permissions:        permissions,
		OrgScopes:          orgScopes,
		PermissionBindings: bindings,
		RegisteredClaims:   jwtRegisteredClaims(user.ID, now, now.Add(s.tokenTTL)),
	}
	token, err := auth.SignToken(s.jwtSecret, claims)
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	if err := s.userRepo.UpdateLastLogin(ctx, user.ID, normalizeIPAddress(ipAddress)); err != nil {
		return nil, err
	}
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&user.ID, "login", "users", &user.ID, map[string]any{
		"username": user.Username,
		"roles":    roles,
	}, ipAddress, userAgent))

	return &LoginResult{
		Token:              token,
		TokenType:          "Bearer",
		ExpiresIn:          int64(s.tokenTTL.Seconds()),
		MustChangePassword: user.MustChangePassword && s.enforceMustChangePassword,
		User: AuthenticatedUserDTO{
			ID:                 user.ID,
			Username:           user.Username,
			Role:               primaryRole,
			Roles:              roles,
			Permissions:        permissions,
			Organizations:      orgScopes,
			PermissionBindings: bindings,
		},
	}, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword, ipAddress, userAgent string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.Status != "active" {
		return ErrForbidden
	}

	if compareErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); compareErr != nil {
		return ErrInvalidPassword
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, user.ID, string(hashBytes), false); err != nil {
		return err
	}

	_ = s.auditRepo.Create(ctx, buildAuditRecord(&userID, "update", "users", &userID, map[string]any{
		"event": "change_password",
	}, ipAddress, userAgent))
	return nil
}

func (s *AuthService) Logout(ctx context.Context, userID uint, ipAddress, userAgent string) error {
	if err := s.userRepo.EnsureExists(ctx, userID); err != nil {
		return err
	}
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&userID, "logout", "users", &userID, map[string]any{
		"event": "logout",
	}, ipAddress, userAgent))
	return nil
}

func (s *AuthService) GetProfile(ctx context.Context, userID uint) (*AuthenticatedUserDTO, bool, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, false, err
	}

	primaryRole, roles, permissions, orgScopes, bindings, err := extractIdentity(user)
	if err != nil {
		return nil, false, err
	}
	return &AuthenticatedUserDTO{
		ID:                 user.ID,
		Username:           user.Username,
		Role:               primaryRole,
		Roles:              roles,
		Permissions:        permissions,
		Organizations:      orgScopes,
		PermissionBindings: bindings,
	}, user.MustChangePassword && s.enforceMustChangePassword, nil
}

func extractIdentity(user *model.User) (string, []string, []string, []auth.OrganizationScope, []auth.PermissionBinding, error) {
	roleSet := make(map[string]struct{})
	permissionSet := make(map[string]struct{})
	roleCodes := make([]string, 0, len(user.UserRoles))
	permissions := make([]string, 0, 8)
	primaryRole := ""

	for _, userRole := range user.UserRoles {
		code := userRole.Role.RoleCode
		if code == "" {
			continue
		}
		if _, exists := roleSet[code]; !exists {
			roleSet[code] = struct{}{}
			roleCodes = append(roleCodes, code)
		}
		if primaryRole == "" || userRole.IsPrimary {
			primaryRole = code
		}

		parsedPermissions := make([]string, 0, 8)
		if err := json.Unmarshal([]byte(userRole.Role.Permissions), &parsedPermissions); err != nil {
			return "", nil, nil, nil, nil, fmt.Errorf("invalid role permissions for %s: %w", code, err)
		}
		for _, permission := range parsedPermissions {
			if _, exists := permissionSet[permission]; exists {
				continue
			}
			permissionSet[permission] = struct{}{}
			permissions = append(permissions, permission)
		}
	}
	for _, permission := range auth.PermissionsForRoles(roleCodes) {
		if _, exists := permissionSet[permission]; exists {
			continue
		}
		permissionSet[permission] = struct{}{}
		permissions = append(permissions, permission)
	}

	orgScopes := make([]auth.OrganizationScope, 0, len(user.UserOrganizations))
	for _, item := range user.UserOrganizations {
		orgScopes = append(orgScopes, auth.OrganizationScope{
			OrganizationType: item.OrganizationType,
			OrganizationID:   item.OrganizationID,
			RoleInOrg:        item.RoleInOrg,
			IsPrimary:        item.IsPrimary,
		})
	}
	bindings := make([]auth.PermissionBinding, 0, len(user.UserPermissionBindings))
	for _, item := range user.UserPermissionBindings {
		bindings = append(bindings, auth.PermissionBinding{
			RoleCode:       item.RoleCode,
			ScopeOrgType:   item.ScopeOrgType,
			ScopeOrgID:     item.ScopeOrgID,
			PersonObjectID: item.PersonObjectID,
			TeamObjectID:   item.TeamObjectID,
			IsPrimary:      item.IsPrimary,
		})
	}

	if primaryRole == "" && len(roleCodes) > 0 {
		primaryRole = roleCodes[0]
	}
	return primaryRole, roleCodes, permissions, orgScopes, bindings, nil
}

func buildAuditRecord(
	userID *uint,
	actionType string,
	targetType string,
	targetID *uint,
	detail map[string]any,
	ipAddress string,
	userAgent string,
) model.AuditLog {
	detailBytes, _ := json.Marshal(detail)
	return model.AuditLog{
		UserID:       userID,
		ActionType:   actionType,
		TargetType:   targetType,
		TargetID:     targetID,
		ActionDetail: string(detailBytes),
		IPAddress:    normalizeIPAddress(ipAddress),
		UserAgent:    userAgent,
	}
}

func jwtRegisteredClaims(userID uint, issuedAt, expiresAt time.Time) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(uint64(userID), 10),
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		NotBefore: jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		Issuer:    "assessv2",
	}
}

func normalizeIPAddress(ipAddress string) string {
	if ipAddress == "" {
		return ""
	}
	parsed := net.ParseIP(ipAddress)
	if parsed == nil {
		return ipAddress
	}
	return parsed.String()
}
