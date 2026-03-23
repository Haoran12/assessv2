package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

var positionLevelCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_:-]{1,49}$`)

type OrgService struct {
	db        *gorm.DB
	auditRepo *repository.AuditRepository
}

type OrgTreeNode struct {
	ID             uint           `json:"id"`
	NodeType       string         `json:"nodeType"`
	Name           string         `json:"name"`
	Status         string         `json:"status"`
	OrganizationID *uint          `json:"organizationId,omitempty"`
	DepartmentID   *uint          `json:"departmentId,omitempty"`
	ParentID       *uint          `json:"parentId,omitempty"`
	SortOrder      int            `json:"sortOrder"`
	Children       []*OrgTreeNode `json:"children,omitempty"`
}

type ListOrganizationFilter struct {
	Status  string
	Keyword string
}

type CreateOrganizationInput struct {
	OrgName   string
	OrgType   string
	ParentID  *uint
	LeaderID  *uint
	SortOrder int
	Status    string
}

type UpdateOrganizationInput = CreateOrganizationInput

type ListDepartmentFilter struct {
	OrganizationID *uint
	Status         string
	Keyword        string
}

type CreateDepartmentInput struct {
	DeptName       string
	OrganizationID uint
	ParentDeptID   *uint
	LeaderID       *uint
	SortOrder      int
	Status         string
}

type UpdateDepartmentInput = CreateDepartmentInput

type ListEmployeeFilter struct {
	OrganizationID *uint
	DepartmentID   *uint
	Status         string
	Keyword        string
}

type CreateEmployeeInput struct {
	EmpName         string
	OrganizationID  uint
	DepartmentID    *uint
	PositionLevelID uint
	PositionTitle   string
	HireDate        *time.Time
	Status          string
}

type UpdateEmployeeInput = CreateEmployeeInput

type TransferEmployeeInput struct {
	ChangeType         string
	NewOrganizationID  *uint
	NewDepartmentID    *uint
	NewPositionLevelID *uint
	NewPositionTitle   *string
	ChangeReason       string
	EffectiveDate      *time.Time
}

func NewOrgService(db *gorm.DB, auditRepo *repository.AuditRepository) *OrgService {
	return &OrgService{db: db, auditRepo: auditRepo}
}

func (s *OrgService) Tree(ctx context.Context, includeInactive bool) ([]*OrgTreeNode, error) {
	var organizations []model.Organization
	orgQuery := s.db.WithContext(ctx).Where("deleted_at IS NULL").Order("sort_order ASC, id ASC")
	if !includeInactive {
		orgQuery = orgQuery.Where("status = ?", "active")
	}
	if err := orgQuery.Find(&organizations).Error; err != nil {
		return nil, fmt.Errorf("failed to query organizations: %w", err)
	}

	var departments []model.Department
	deptQuery := s.db.WithContext(ctx).Where("deleted_at IS NULL").Order("sort_order ASC, id ASC")
	if !includeInactive {
		deptQuery = deptQuery.Where("status = ?", "active")
	}
	if err := deptQuery.Find(&departments).Error; err != nil {
		return nil, fmt.Errorf("failed to query departments: %w", err)
	}

	var employees []model.Employee
	empQuery := s.db.WithContext(ctx).Where("deleted_at IS NULL").Order("id ASC")
	if !includeInactive {
		empQuery = empQuery.Where("status = ?", "active")
	}
	if err := empQuery.Find(&employees).Error; err != nil {
		return nil, fmt.Errorf("failed to query employees: %w", err)
	}

	orgNodes := map[uint]*OrgTreeNode{}
	deptNodes := map[uint]*OrgTreeNode{}
	roots := make([]*OrgTreeNode, 0, len(organizations))

	for _, item := range organizations {
		node := &OrgTreeNode{ID: item.ID, NodeType: "organization", Name: item.OrgName, Status: item.Status, ParentID: item.ParentID, SortOrder: item.SortOrder}
		orgID := item.ID
		node.OrganizationID = &orgID
		orgNodes[item.ID] = node
	}
	for _, item := range organizations {
		node := orgNodes[item.ID]
		if item.ParentID != nil {
			if parent, ok := orgNodes[*item.ParentID]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}

	for _, item := range departments {
		node := &OrgTreeNode{ID: item.ID, NodeType: "department", Name: item.DeptName, Status: item.Status, ParentID: item.ParentDeptID, SortOrder: item.SortOrder}
		orgID := item.OrganizationID
		node.OrganizationID = &orgID
		deptID := item.ID
		node.DepartmentID = &deptID
		deptNodes[item.ID] = node
	}
	for _, item := range departments {
		node := deptNodes[item.ID]
		if item.ParentDeptID != nil {
			if parent, ok := deptNodes[*item.ParentDeptID]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		if parentOrg, ok := orgNodes[item.OrganizationID]; ok {
			parentOrg.Children = append(parentOrg.Children, node)
		}
	}

	for _, item := range employees {
		node := &OrgTreeNode{ID: item.ID, NodeType: "employee", Name: item.EmpName, Status: item.Status}
		orgID := item.OrganizationID
		node.OrganizationID = &orgID
		if item.DepartmentID != nil {
			deptID := *item.DepartmentID
			node.DepartmentID = &deptID
			if deptNode, ok := deptNodes[deptID]; ok {
				deptNode.Children = append(deptNode.Children, node)
				continue
			}
		}
		if orgNode, ok := orgNodes[item.OrganizationID]; ok {
			orgNode.Children = append(orgNode.Children, node)
		}
	}

	sortTreeNodes(roots)
	return roots, nil
}

func (s *OrgService) ListOrganizations(ctx context.Context, filter ListOrganizationFilter) ([]model.Organization, error) {
	query := s.db.WithContext(ctx).Where("deleted_at IS NULL").Order("sort_order ASC, id ASC")
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		kw := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("org_name LIKE ?", kw)
	}
	var items []model.Organization
	if err := query.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}
	return items, nil
}

func (s *OrgService) CreateOrganization(ctx context.Context, claims *auth.Claims, operatorID uint, input CreateOrganizationInput, ipAddress string, userAgent string) (*model.Organization, error) {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return nil, err
	}
	writeScope, err := requireOrgWriteScope(ctx, s.db, claims)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.OrgName) == "" {
		return nil, ErrInvalidParam
	}
	if !isValidOrganizationType(strings.TrimSpace(input.OrgType)) {
		return nil, ErrInvalidOrganizationType
	}
	status := normalizeStatus(input.Status, "active")
	if !isValidOrgStatus(status) {
		return nil, ErrInvalidOrganizationStatus
	}
	if !writeScope.unrestricted {
		if strings.TrimSpace(input.OrgType) == "group" {
			return nil, ErrForbidden
		}
		if input.ParentID == nil || *input.ParentID == 0 {
			return nil, ErrForbidden
		}
		if !writeScope.allowsOrganization(*input.ParentID) {
			return nil, ErrForbidden
		}
	}
	if input.ParentID != nil && *input.ParentID > 0 {
		if err := s.requireOrganization(ctx, *input.ParentID); err != nil {
			return nil, err
		}
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	record := model.Organization{OrgName: strings.TrimSpace(input.OrgName), OrgType: strings.TrimSpace(input.OrgType), ParentID: input.ParentID, LeaderID: input.LeaderID, SortOrder: input.SortOrder, Status: status, CreatedBy: operatorRef, UpdatedBy: operatorRef}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"create",
		"organizations",
		&targetID,
		buildAuditDetail("org.organization.create", map[string]any{}, serializeOrganizationForAudit(&record), nil),
		ipAddress,
		userAgent,
	))
	return &record, nil
}

func (s *OrgService) UpdateOrganization(ctx context.Context, claims *auth.Claims, operatorID, organizationID uint, input UpdateOrganizationInput, ipAddress string, userAgent string) (*model.Organization, error) {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return nil, err
	}
	writeScope, err := requireOrgWriteScope(ctx, s.db, claims)
	if err != nil {
		return nil, err
	}
	if !writeScope.allowsOrganization(organizationID) {
		return nil, ErrForbidden
	}
	if strings.TrimSpace(input.OrgName) == "" {
		return nil, ErrInvalidParam
	}
	if !isValidOrganizationType(strings.TrimSpace(input.OrgType)) {
		return nil, ErrInvalidOrganizationType
	}
	status := normalizeStatus(input.Status, "active")
	if !isValidOrgStatus(status) {
		return nil, ErrInvalidOrganizationStatus
	}
	if input.ParentID != nil && *input.ParentID > 0 {
		if *input.ParentID == organizationID {
			return nil, ErrInvalidParam
		}
		if !writeScope.allowsOrganization(*input.ParentID) {
			return nil, ErrForbidden
		}
		if err := s.requireOrganization(ctx, *input.ParentID); err != nil {
			return nil, err
		}
	}

	var existing model.Organization
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", organizationID).First(&existing).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrOrganizationNotFound
		}
		return nil, fmt.Errorf("failed to query organization: %w", err)
	}
	beforeAudit := serializeOrganizationForAudit(&existing)

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	updates := map[string]any{"org_name": strings.TrimSpace(input.OrgName), "org_type": strings.TrimSpace(input.OrgType), "parent_id": input.ParentID, "leader_id": input.LeaderID, "sort_order": input.SortOrder, "status": status, "updated_by": operatorRef, "updated_at": time.Now().Unix()}
	if err := s.db.WithContext(ctx).Model(&model.Organization{}).Where("id = ? AND deleted_at IS NULL", organizationID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", organizationID).First(&existing).Error; err != nil {
		return nil, fmt.Errorf("failed to reload organization: %w", err)
	}
	targetID := existing.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"update",
		"organizations",
		&targetID,
		buildAuditDetail("org.organization.update", beforeAudit, serializeOrganizationForAudit(&existing), nil),
		ipAddress,
		userAgent,
	))
	return &existing, nil
}

func (s *OrgService) DeleteOrganization(ctx context.Context, claims *auth.Claims, operatorID, organizationID uint, ipAddress string, userAgent string) error {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return err
	}
	writeScope, err := requireOrgWriteScope(ctx, s.db, claims)
	if err != nil {
		return err
	}
	if !writeScope.allowsOrganization(organizationID) {
		return ErrForbidden
	}

	var existing model.Organization
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", organizationID).First(&existing).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return ErrOrganizationNotFound
		}
		return fmt.Errorf("failed to query organization: %w", err)
	}
	beforeAudit := serializeOrganizationForAudit(&existing)
	if err := s.ensureOrganizationNotInUse(ctx, organizationID); err != nil {
		return err
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	now := time.Now().Unix()
	updates := map[string]any{
		"deleted_at": now,
		"updated_by": operatorRef,
		"updated_at": now,
	}
	if err := s.db.WithContext(ctx).Model(&model.Organization{}).Where("id = ? AND deleted_at IS NULL", organizationID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to delete organization: %w", err)
	}

	targetID := existing.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"delete",
		"organizations",
		&targetID,
		buildAuditDetail("org.organization.delete", beforeAudit, map[string]any{}, nil),
		ipAddress,
		userAgent,
	))
	return nil
}

func (s *OrgService) ListDepartments(ctx context.Context, filter ListDepartmentFilter) ([]model.Department, error) {
	query := s.db.WithContext(ctx).Where("deleted_at IS NULL").Order("sort_order ASC, id ASC")
	if filter.OrganizationID != nil && *filter.OrganizationID > 0 {
		query = query.Where("organization_id = ?", *filter.OrganizationID)
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		kw := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("dept_name LIKE ?", kw)
	}
	var items []model.Department
	if err := query.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list departments: %w", err)
	}
	return items, nil
}

func (s *OrgService) CreateDepartment(ctx context.Context, claims *auth.Claims, operatorID uint, input CreateDepartmentInput, ipAddress string, userAgent string) (*model.Department, error) {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return nil, err
	}
	writeScope, err := requireOrgWriteScope(ctx, s.db, claims)
	if err != nil {
		return nil, err
	}
	if !writeScope.allowsOrganization(input.OrganizationID) {
		return nil, ErrForbidden
	}

	if strings.TrimSpace(input.DeptName) == "" {
		return nil, ErrInvalidParam
	}
	if err := s.requireOrganization(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	if input.ParentDeptID != nil && *input.ParentDeptID > 0 {
		if err := s.requireDepartment(ctx, *input.ParentDeptID); err != nil {
			return nil, err
		}
		parentOrgID, err := s.departmentOrganizationID(ctx, *input.ParentDeptID)
		if err != nil {
			return nil, err
		}
		if parentOrgID != input.OrganizationID || !writeScope.allowsOrganization(parentOrgID) {
			return nil, ErrForbidden
		}
	}
	status := normalizeStatus(input.Status, "active")
	if !isValidDepartmentStatus(status) {
		return nil, ErrInvalidDepartmentStatus
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	record := model.Department{DeptName: strings.TrimSpace(input.DeptName), OrganizationID: input.OrganizationID, ParentDeptID: input.ParentDeptID, LeaderID: input.LeaderID, SortOrder: input.SortOrder, Status: status, CreatedBy: operatorRef, UpdatedBy: operatorRef}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, fmt.Errorf("failed to create department: %w", err)
	}
	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"create",
		"departments",
		&targetID,
		buildAuditDetail("org.department.create", map[string]any{}, serializeDepartmentForAudit(&record), nil),
		ipAddress,
		userAgent,
	))
	return &record, nil
}

func (s *OrgService) UpdateDepartment(ctx context.Context, claims *auth.Claims, operatorID, departmentID uint, input UpdateDepartmentInput, ipAddress string, userAgent string) (*model.Department, error) {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return nil, err
	}
	writeScope, err := requireOrgWriteScope(ctx, s.db, claims)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.DeptName) == "" {
		return nil, ErrInvalidParam
	}
	if !writeScope.allowsOrganization(input.OrganizationID) {
		return nil, ErrForbidden
	}
	if err := s.requireOrganization(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	if input.ParentDeptID != nil && *input.ParentDeptID > 0 {
		if *input.ParentDeptID == departmentID {
			return nil, ErrInvalidParam
		}
		if err := s.requireDepartment(ctx, *input.ParentDeptID); err != nil {
			return nil, err
		}
		parentOrgID, err := s.departmentOrganizationID(ctx, *input.ParentDeptID)
		if err != nil {
			return nil, err
		}
		if parentOrgID != input.OrganizationID || !writeScope.allowsOrganization(parentOrgID) {
			return nil, ErrForbidden
		}
	}
	status := normalizeStatus(input.Status, "active")
	if !isValidDepartmentStatus(status) {
		return nil, ErrInvalidDepartmentStatus
	}

	var existing model.Department
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", departmentID).First(&existing).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrDepartmentNotFound
		}
		return nil, fmt.Errorf("failed to query department: %w", err)
	}
	beforeAudit := serializeDepartmentForAudit(&existing)
	if !writeScope.allowsOrganization(existing.OrganizationID) {
		return nil, ErrForbidden
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	updates := map[string]any{"dept_name": strings.TrimSpace(input.DeptName), "organization_id": input.OrganizationID, "parent_dept_id": input.ParentDeptID, "leader_id": input.LeaderID, "sort_order": input.SortOrder, "status": status, "updated_by": operatorRef, "updated_at": time.Now().Unix()}
	if err := s.db.WithContext(ctx).Model(&model.Department{}).Where("id = ? AND deleted_at IS NULL", departmentID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update department: %w", err)
	}
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", departmentID).First(&existing).Error; err != nil {
		return nil, fmt.Errorf("failed to reload department: %w", err)
	}
	targetID := existing.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"update",
		"departments",
		&targetID,
		buildAuditDetail("org.department.update", beforeAudit, serializeDepartmentForAudit(&existing), nil),
		ipAddress,
		userAgent,
	))
	return &existing, nil
}

func (s *OrgService) DeleteDepartment(ctx context.Context, claims *auth.Claims, operatorID, departmentID uint, ipAddress string, userAgent string) error {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return err
	}
	writeScope, err := requireOrgWriteScope(ctx, s.db, claims)
	if err != nil {
		return err
	}
	var existing model.Department
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", departmentID).First(&existing).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return ErrDepartmentNotFound
		}
		return fmt.Errorf("failed to query department: %w", err)
	}
	beforeAudit := serializeDepartmentForAudit(&existing)
	if !writeScope.allowsOrganization(existing.OrganizationID) {
		return ErrForbidden
	}
	if err := s.ensureDepartmentNotInUse(ctx, departmentID); err != nil {
		return err
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	now := time.Now().Unix()
	updates := map[string]any{
		"deleted_at": now,
		"updated_by": operatorRef,
		"updated_at": now,
	}
	if err := s.db.WithContext(ctx).Model(&model.Department{}).Where("id = ? AND deleted_at IS NULL", departmentID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to delete department: %w", err)
	}

	targetID := existing.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"delete",
		"departments",
		&targetID,
		buildAuditDetail("org.department.delete", beforeAudit, map[string]any{}, nil),
		ipAddress,
		userAgent,
	))
	return nil
}

type ListPositionLevelFilter struct {
	Status string
}

type ListAssessmentCategoryFilter struct {
	ObjectType string
	Status     string
}

type CreatePositionLevelInput struct {
	LevelCode       string
	LevelName       string
	Description     string
	IsForAssessment *bool
	SortOrder       int
	Status          string
}

type UpdatePositionLevelInput = CreatePositionLevelInput

func (s *OrgService) ListAssessmentCategories(ctx context.Context, filter ListAssessmentCategoryFilter) ([]model.AssessmentCategory, error) {
	query := s.db.WithContext(ctx).Model(&model.AssessmentCategory{}).Order("sort_order ASC, id ASC")

	status := strings.TrimSpace(filter.Status)
	if status == "" {
		status = "active"
	}
	query = query.Where("status = ?", status)

	if objectType := strings.TrimSpace(filter.ObjectType); objectType != "" {
		if !isValidAssessmentObjectType(objectType) {
			return nil, ErrInvalidRuleObjectType
		}
		query = query.Where("object_type = ?", objectType)
	}

	var items []model.AssessmentCategory
	if err := query.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list assessment categories: %w", err)
	}
	return items, nil
}

func (s *OrgService) ListPositionLevels(ctx context.Context, filter ListPositionLevelFilter) ([]model.PositionLevel, error) {
	query := s.db.WithContext(ctx).Order("sort_order ASC, id ASC")
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	var items []model.PositionLevel
	if err := query.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list position levels: %w", err)
	}
	return items, nil
}

func (s *OrgService) CreatePositionLevel(ctx context.Context, operatorID uint, input CreatePositionLevelInput, ipAddress string, userAgent string) (*model.PositionLevel, error) {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return nil, err
	}
	levelCode, err := normalizePositionLevelCode(input.LevelCode)
	if err != nil {
		return nil, err
	}
	levelName := strings.TrimSpace(input.LevelName)
	if levelName == "" {
		return nil, ErrInvalidParam
	}
	status := normalizeStatus(input.Status, "active")
	if !isValidPositionLevelStatus(status) {
		return nil, ErrInvalidPositionLevelStatus
	}

	var duplicateCount int64
	if err := s.db.WithContext(ctx).Model(&model.PositionLevel{}).Where("level_code = ?", levelCode).Count(&duplicateCount).Error; err != nil {
		return nil, fmt.Errorf("failed to verify position level uniqueness: %w", err)
	}
	if duplicateCount > 0 {
		return nil, ErrPositionLevelCodeExists
	}

	isForAssessment := true
	if input.IsForAssessment != nil {
		isForAssessment = *input.IsForAssessment
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	record := model.PositionLevel{
		LevelCode:       levelCode,
		LevelName:       levelName,
		Description:     strings.TrimSpace(input.Description),
		IsSystem:        false,
		IsForAssessment: isForAssessment,
		SortOrder:       input.SortOrder,
		Status:          status,
		CreatedBy:       operatorRef,
		UpdatedBy:       operatorRef,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrPositionLevelCodeExists
		}
		return nil, fmt.Errorf("failed to create position level: %w", err)
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"create",
		"position_levels",
		&targetID,
		buildAuditDetail("org.position_level.create", map[string]any{}, serializePositionLevelForAudit(&record), nil),
		ipAddress,
		userAgent,
	))
	return &record, nil
}

func (s *OrgService) UpdatePositionLevel(ctx context.Context, operatorID, positionLevelID uint, input UpdatePositionLevelInput, ipAddress string, userAgent string) (*model.PositionLevel, error) {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return nil, err
	}
	levelCode, err := normalizePositionLevelCode(input.LevelCode)
	if err != nil {
		return nil, err
	}
	levelName := strings.TrimSpace(input.LevelName)
	if levelName == "" {
		return nil, ErrInvalidParam
	}
	status := normalizeStatus(input.Status, "active")
	if !isValidPositionLevelStatus(status) {
		return nil, ErrInvalidPositionLevelStatus
	}

	var existing model.PositionLevel
	if err := s.db.WithContext(ctx).Where("id = ?", positionLevelID).First(&existing).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrPositionLevelNotFound
		}
		return nil, fmt.Errorf("failed to query position level: %w", err)
	}
	beforeAudit := serializePositionLevelForAudit(&existing)
	if existing.IsSystem && levelCode != existing.LevelCode {
		return nil, ErrSystemPositionLevelLocked
	}
	if levelCode != existing.LevelCode {
		var duplicateCount int64
		if err := s.db.WithContext(ctx).Model(&model.PositionLevel{}).Where("level_code = ? AND id <> ?", levelCode, positionLevelID).Count(&duplicateCount).Error; err != nil {
			return nil, fmt.Errorf("failed to verify position level uniqueness: %w", err)
		}
		if duplicateCount > 0 {
			return nil, ErrPositionLevelCodeExists
		}
	}

	isForAssessment := existing.IsForAssessment
	if input.IsForAssessment != nil {
		isForAssessment = *input.IsForAssessment
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	updates := map[string]any{
		"level_code":        levelCode,
		"level_name":        levelName,
		"description":       strings.TrimSpace(input.Description),
		"is_for_assessment": isForAssessment,
		"sort_order":        input.SortOrder,
		"status":            status,
		"updated_by":        operatorRef,
		"updated_at":        time.Now().Unix(),
	}
	if err := s.db.WithContext(ctx).Model(&model.PositionLevel{}).Where("id = ?", positionLevelID).Updates(updates).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrPositionLevelCodeExists
		}
		return nil, fmt.Errorf("failed to update position level: %w", err)
	}
	if err := s.db.WithContext(ctx).Where("id = ?", positionLevelID).First(&existing).Error; err != nil {
		return nil, fmt.Errorf("failed to reload position level: %w", err)
	}

	targetID := existing.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"update",
		"position_levels",
		&targetID,
		buildAuditDetail("org.position_level.update", beforeAudit, serializePositionLevelForAudit(&existing), nil),
		ipAddress,
		userAgent,
	))
	return &existing, nil
}

func (s *OrgService) DeletePositionLevel(ctx context.Context, operatorID, positionLevelID uint, ipAddress string, userAgent string) error {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return err
	}
	var existing model.PositionLevel
	if err := s.db.WithContext(ctx).Where("id = ?", positionLevelID).First(&existing).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return ErrPositionLevelNotFound
		}
		return fmt.Errorf("failed to query position level: %w", err)
	}
	beforeAudit := serializePositionLevelForAudit(&existing)
	if err := s.ensurePositionLevelNotInUse(ctx, positionLevelID); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Delete(&model.PositionLevel{}, positionLevelID).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "foreign key constraint failed") {
			return ErrPositionLevelInUse
		}
		return fmt.Errorf("failed to delete position level: %w", err)
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	targetID := existing.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"delete",
		"position_levels",
		&targetID,
		buildAuditDetail("org.position_level.delete", beforeAudit, map[string]any{}, nil),
		ipAddress,
		userAgent,
	))
	return nil
}

func (s *OrgService) ListEmployees(ctx context.Context, filter ListEmployeeFilter) ([]model.Employee, error) {
	query := s.db.WithContext(ctx).Where("deleted_at IS NULL").Order("id ASC")
	if filter.OrganizationID != nil && *filter.OrganizationID > 0 {
		query = query.Where("organization_id = ?", *filter.OrganizationID)
	}
	if filter.DepartmentID != nil && *filter.DepartmentID > 0 {
		query = query.Where("department_id = ?", *filter.DepartmentID)
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		kw := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("emp_name LIKE ?", kw)
	}
	var items []model.Employee
	if err := query.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list employees: %w", err)
	}
	return items, nil
}

func (s *OrgService) CreateEmployee(ctx context.Context, claims *auth.Claims, operatorID uint, input CreateEmployeeInput, ipAddress string, userAgent string) (*model.Employee, error) {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return nil, err
	}
	writeScope, err := requireOrgWriteScope(ctx, s.db, claims)
	if err != nil {
		return nil, err
	}
	if !writeScope.allowsOrganization(input.OrganizationID) {
		return nil, ErrForbidden
	}

	if strings.TrimSpace(input.EmpName) == "" {
		return nil, ErrInvalidParam
	}
	if err := s.requireOrganization(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	if err := s.requirePositionLevel(ctx, input.PositionLevelID); err != nil {
		return nil, err
	}
	if input.DepartmentID != nil && *input.DepartmentID > 0 {
		if err := s.requireDepartmentWithOrg(ctx, *input.DepartmentID, input.OrganizationID); err != nil {
			return nil, err
		}
	}
	status := normalizeStatus(input.Status, "active")
	if !isValidEmployeeStatus(status) {
		return nil, ErrInvalidEmployeeStatus
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	record := model.Employee{EmpName: strings.TrimSpace(input.EmpName), OrganizationID: input.OrganizationID, DepartmentID: input.DepartmentID, PositionLevelID: input.PositionLevelID, PositionTitle: strings.TrimSpace(input.PositionTitle), HireDate: input.HireDate, Status: status, CreatedBy: operatorRef, UpdatedBy: operatorRef}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, fmt.Errorf("failed to create employee: %w", err)
	}
	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"create",
		"employees",
		&targetID,
		buildAuditDetail("org.employee.create", map[string]any{}, serializeEmployeeForAudit(&record), nil),
		ipAddress,
		userAgent,
	))
	return &record, nil
}

func (s *OrgService) UpdateEmployee(ctx context.Context, claims *auth.Claims, operatorID, employeeID uint, input UpdateEmployeeInput, ipAddress string, userAgent string) (*model.Employee, error) {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return nil, err
	}
	writeScope, err := requireOrgWriteScope(ctx, s.db, claims)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.EmpName) == "" {
		return nil, ErrInvalidParam
	}
	if !writeScope.allowsOrganization(input.OrganizationID) {
		return nil, ErrForbidden
	}
	if err := s.requireOrganization(ctx, input.OrganizationID); err != nil {
		return nil, err
	}
	if err := s.requirePositionLevel(ctx, input.PositionLevelID); err != nil {
		return nil, err
	}
	if input.DepartmentID != nil && *input.DepartmentID > 0 {
		if err := s.requireDepartmentWithOrg(ctx, *input.DepartmentID, input.OrganizationID); err != nil {
			return nil, err
		}
	}
	status := normalizeStatus(input.Status, "active")
	if !isValidEmployeeStatus(status) {
		return nil, ErrInvalidEmployeeStatus
	}

	var existing model.Employee
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", employeeID).First(&existing).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrEmployeeNotFound
		}
		return nil, fmt.Errorf("failed to query employee: %w", err)
	}
	beforeAudit := serializeEmployeeForAudit(&existing)
	if !writeScope.allowsOrganization(existing.OrganizationID) {
		return nil, ErrForbidden
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	updates := map[string]any{"emp_name": strings.TrimSpace(input.EmpName), "organization_id": input.OrganizationID, "department_id": input.DepartmentID, "position_level_id": input.PositionLevelID, "position_title": strings.TrimSpace(input.PositionTitle), "hire_date": input.HireDate, "status": status, "updated_by": operatorRef, "updated_at": time.Now().Unix()}
	if err := s.db.WithContext(ctx).Model(&model.Employee{}).Where("id = ? AND deleted_at IS NULL", employeeID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update employee: %w", err)
	}
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", employeeID).First(&existing).Error; err != nil {
		return nil, fmt.Errorf("failed to reload employee: %w", err)
	}
	targetID := existing.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"update",
		"employees",
		&targetID,
		buildAuditDetail("org.employee.update", beforeAudit, serializeEmployeeForAudit(&existing), nil),
		ipAddress,
		userAgent,
	))
	return &existing, nil
}

func (s *OrgService) DeleteEmployee(ctx context.Context, claims *auth.Claims, operatorID, employeeID uint, ipAddress string, userAgent string) error {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return err
	}
	writeScope, err := requireOrgWriteScope(ctx, s.db, claims)
	if err != nil {
		return err
	}
	var existing model.Employee
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", employeeID).First(&existing).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return ErrEmployeeNotFound
		}
		return fmt.Errorf("failed to query employee: %w", err)
	}
	beforeAudit := serializeEmployeeForAudit(&existing)
	if !writeScope.allowsOrganization(existing.OrganizationID) {
		return ErrForbidden
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	now := time.Now().Unix()
	updates := map[string]any{
		"deleted_at": now,
		"updated_by": operatorRef,
		"updated_at": now,
	}
	if err := s.db.WithContext(ctx).Model(&model.Employee{}).Where("id = ? AND deleted_at IS NULL", employeeID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to delete employee: %w", err)
	}

	targetID := existing.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"delete",
		"employees",
		&targetID,
		buildAuditDetail("org.employee.delete", beforeAudit, map[string]any{}, nil),
		ipAddress,
		userAgent,
	))
	return nil
}

func (s *OrgService) TransferEmployee(ctx context.Context, claims *auth.Claims, operatorID, employeeID uint, input TransferEmployeeInput, ipAddress string, userAgent string) (*model.Employee, error) {
	if err := ensureLatestAssessmentConfigWritableTx(s.db.WithContext(ctx)); err != nil {
		return nil, err
	}
	writeScope, err := requireOrgWriteScope(ctx, s.db, claims)
	if err != nil {
		return nil, err
	}
	if !isValidTransferType(strings.TrimSpace(input.ChangeType)) {
		return nil, ErrInvalidTransferType
	}
	if input.EffectiveDate == nil {
		return nil, ErrInvalidEffectiveDate
	}

	var employee model.Employee
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", employeeID).First(&employee).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrEmployeeNotFound
		}
		return nil, fmt.Errorf("failed to query employee: %w", err)
	}
	beforeAudit := serializeEmployeeForAudit(&employee)
	if !writeScope.allowsOrganization(employee.OrganizationID) {
		return nil, ErrForbidden
	}

	newOrgID := employee.OrganizationID
	if input.NewOrganizationID != nil {
		if *input.NewOrganizationID == 0 {
			return nil, ErrInvalidParam
		}
		newOrgID = *input.NewOrganizationID
	}
	if err := s.requireOrganization(ctx, newOrgID); err != nil {
		return nil, err
	}
	if !writeScope.allowsOrganization(newOrgID) {
		return nil, ErrForbidden
	}

	newDeptID := employee.DepartmentID
	if input.NewDepartmentID != nil {
		if *input.NewDepartmentID == 0 {
			newDeptID = nil
		} else {
			candidate := *input.NewDepartmentID
			newDeptID = &candidate
		}
	}
	if newDeptID != nil {
		if err := s.requireDepartmentWithOrg(ctx, *newDeptID, newOrgID); err != nil {
			return nil, err
		}
	}

	newPositionLevelID := employee.PositionLevelID
	if input.NewPositionLevelID != nil {
		if *input.NewPositionLevelID == 0 {
			return nil, ErrInvalidParam
		}
		newPositionLevelID = *input.NewPositionLevelID
	}
	if err := s.requirePositionLevel(ctx, newPositionLevelID); err != nil {
		return nil, err
	}

	newPositionTitle := employee.PositionTitle
	if input.NewPositionTitle != nil {
		newPositionTitle = strings.TrimSpace(*input.NewPositionTitle)
	}

	operator := operatorID
	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operator)
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		history := model.EmployeeHistory{EmployeeID: employee.ID, ChangeType: strings.TrimSpace(input.ChangeType), OldOrganizationID: uintPtr(employee.OrganizationID), NewOrganizationID: uintPtr(newOrgID), OldDepartmentID: employee.DepartmentID, NewDepartmentID: newDeptID, OldPositionLevelID: uintPtr(employee.PositionLevelID), NewPositionLevelID: uintPtr(newPositionLevelID), OldPositionTitle: employee.PositionTitle, NewPositionTitle: newPositionTitle, ChangeReason: strings.TrimSpace(input.ChangeReason), EffectiveDate: *input.EffectiveDate, CreatedBy: operatorRef}
		if err := tx.Create(&history).Error; err != nil {
			return fmt.Errorf("failed to create employee history: %w", err)
		}

		updates := map[string]any{"organization_id": newOrgID, "department_id": newDeptID, "position_level_id": newPositionLevelID, "position_title": newPositionTitle, "updated_by": operatorRef, "updated_at": time.Now().Unix()}
		if err := tx.Model(&model.Employee{}).Where("id = ? AND deleted_at IS NULL", employeeID).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update employee for transfer: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", employeeID).First(&employee).Error; err != nil {
		return nil, fmt.Errorf("failed to reload employee after transfer: %w", err)
	}

	targetID := employee.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(
		operatorRef,
		"update",
		"employees",
		&targetID,
		buildAuditDetail("org.employee.transfer", beforeAudit, serializeEmployeeForAudit(&employee), map[string]any{
			"change_type":      strings.TrimSpace(input.ChangeType),
			"change_reason":    strings.TrimSpace(input.ChangeReason),
			"new_organization": newOrgID,
			"effective_date":   input.EffectiveDate.Format("2006-01-02"),
		}),
		ipAddress,
		userAgent,
	))
	return &employee, nil
}

func (s *OrgService) ListEmployeeHistory(ctx context.Context, employeeID uint) ([]model.EmployeeHistory, error) {
	if err := s.requireEmployee(ctx, employeeID); err != nil {
		return nil, err
	}
	var items []model.EmployeeHistory
	if err := s.db.WithContext(ctx).Where("employee_id = ?", employeeID).Order("effective_date DESC, id DESC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list employee history: %w", err)
	}
	return items, nil
}

func (s *OrgService) requireOrganization(ctx context.Context, organizationID uint) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.Organization{}).Where("id = ? AND deleted_at IS NULL", organizationID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to verify organization: %w", err)
	}
	if count == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

func (s *OrgService) ensureOrganizationNotInUse(ctx context.Context, organizationID uint) error {
	var childOrgCount int64
	if err := s.db.WithContext(ctx).Model(&model.Organization{}).Where("parent_id = ? AND deleted_at IS NULL", organizationID).Count(&childOrgCount).Error; err != nil {
		return fmt.Errorf("failed to verify organization child organizations: %w", err)
	}
	if childOrgCount > 0 {
		return ErrOrganizationInUse
	}

	var deptCount int64
	if err := s.db.WithContext(ctx).Model(&model.Department{}).Where("organization_id = ? AND deleted_at IS NULL", organizationID).Count(&deptCount).Error; err != nil {
		return fmt.Errorf("failed to verify organization departments: %w", err)
	}
	if deptCount > 0 {
		return ErrOrganizationInUse
	}

	var employeeCount int64
	if err := s.db.WithContext(ctx).Model(&model.Employee{}).Where("organization_id = ? AND deleted_at IS NULL", organizationID).Count(&employeeCount).Error; err != nil {
		return fmt.Errorf("failed to verify organization employees: %w", err)
	}
	if employeeCount > 0 {
		return ErrOrganizationInUse
	}
	return nil
}

func (s *OrgService) requireDepartment(ctx context.Context, departmentID uint) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.Department{}).Where("id = ? AND deleted_at IS NULL", departmentID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to verify department: %w", err)
	}
	if count == 0 {
		return ErrDepartmentNotFound
	}
	return nil
}

func (s *OrgService) ensureDepartmentNotInUse(ctx context.Context, departmentID uint) error {
	var childDeptCount int64
	if err := s.db.WithContext(ctx).Model(&model.Department{}).Where("parent_dept_id = ? AND deleted_at IS NULL", departmentID).Count(&childDeptCount).Error; err != nil {
		return fmt.Errorf("failed to verify department child departments: %w", err)
	}
	if childDeptCount > 0 {
		return ErrDepartmentInUse
	}

	var employeeCount int64
	if err := s.db.WithContext(ctx).Model(&model.Employee{}).Where("department_id = ? AND deleted_at IS NULL", departmentID).Count(&employeeCount).Error; err != nil {
		return fmt.Errorf("failed to verify department employees: %w", err)
	}
	if employeeCount > 0 {
		return ErrDepartmentInUse
	}
	return nil
}

func (s *OrgService) requireDepartmentWithOrg(ctx context.Context, departmentID, organizationID uint) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.Department{}).Where("id = ? AND organization_id = ? AND deleted_at IS NULL", departmentID, organizationID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to verify department org relation: %w", err)
	}
	if count == 0 {
		return ErrDepartmentNotFound
	}
	return nil
}

func (s *OrgService) departmentOrganizationID(ctx context.Context, departmentID uint) (uint, error) {
	var department model.Department
	if err := s.db.WithContext(ctx).Select("id", "organization_id").Where("id = ? AND deleted_at IS NULL", departmentID).First(&department).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return 0, ErrDepartmentNotFound
		}
		return 0, fmt.Errorf("failed to query department organization id: %w", err)
	}
	return department.OrganizationID, nil
}

func (s *OrgService) requirePositionLevel(ctx context.Context, positionLevelID uint) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.PositionLevel{}).Where("id = ?", positionLevelID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to verify position level: %w", err)
	}
	if count == 0 {
		return ErrPositionLevelNotFound
	}
	return nil
}

func (s *OrgService) ensurePositionLevelNotInUse(ctx context.Context, positionLevelID uint) error {
	var employeeCount int64
	if err := s.db.WithContext(ctx).Model(&model.Employee{}).Where("position_level_id = ? AND deleted_at IS NULL", positionLevelID).Count(&employeeCount).Error; err != nil {
		return fmt.Errorf("failed to verify employee position level usage: %w", err)
	}
	if employeeCount > 0 {
		return ErrPositionLevelInUse
	}

	var historyCount int64
	if err := s.db.WithContext(ctx).Model(&model.EmployeeHistory{}).Where("old_position_level_id = ? OR new_position_level_id = ?", positionLevelID, positionLevelID).Count(&historyCount).Error; err != nil {
		return fmt.Errorf("failed to verify employee history position level usage: %w", err)
	}
	if historyCount > 0 {
		return ErrPositionLevelInUse
	}
	return nil
}

func (s *OrgService) requireEmployee(ctx context.Context, employeeID uint) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.Employee{}).Where("id = ? AND deleted_at IS NULL", employeeID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to verify employee: %w", err)
	}
	if count == 0 {
		return ErrEmployeeNotFound
	}
	return nil
}

func sortTreeNodes(nodes []*OrgTreeNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].SortOrder == nodes[j].SortOrder {
			if nodes[i].NodeType == nodes[j].NodeType {
				return nodes[i].ID < nodes[j].ID
			}
			return nodes[i].NodeType < nodes[j].NodeType
		}
		return nodes[i].SortOrder < nodes[j].SortOrder
	})
	for _, item := range nodes {
		if len(item.Children) > 0 {
			sortTreeNodes(item.Children)
		}
	}
}

func isValidOrganizationType(orgType string) bool {
	switch orgType {
	case "group", "company":
		return true
	default:
		return false
	}
}

func isValidOrgStatus(status string) bool {
	switch status {
	case "active", "inactive":
		return true
	default:
		return false
	}
}

func isValidDepartmentStatus(status string) bool {
	switch status {
	case "active", "inactive":
		return true
	default:
		return false
	}
}

func isValidEmployeeStatus(status string) bool {
	switch status {
	case "active", "inactive":
		return true
	default:
		return false
	}
}

func isValidPositionLevelStatus(status string) bool {
	switch status {
	case "active", "inactive":
		return true
	default:
		return false
	}
}

func isValidTransferType(changeType string) bool {
	switch changeType {
	case "transfer", "promotion", "demotion", "position_change":
		return true
	default:
		return false
	}
}

func isValidAssessmentObjectType(objectType string) bool {
	switch objectType {
	case ObjectTypeTeam, ObjectTypeIndividual:
		return true
	default:
		return false
	}
}

func normalizeStatus(status, fallback string) string {
	text := strings.TrimSpace(status)
	if text == "" {
		return fallback
	}
	return text
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "unique constraint failed") || strings.Contains(text, "duplicate")
}

func uintPtr(value uint) *uint {
	copyValue := value
	return &copyValue
}

func normalizePositionLevelCode(code string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(code))
	if !positionLevelCodePattern.MatchString(normalized) {
		return "", ErrInvalidParam
	}
	return normalized, nil
}
