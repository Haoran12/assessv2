package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
)

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

func (s *OrgService) CreateOrganization(ctx context.Context, operatorID uint, input CreateOrganizationInput, ipAddress string, userAgent string) (*model.Organization, error) {
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
		if err := s.requireOrganization(ctx, *input.ParentID); err != nil {
			return nil, err
		}
	}

	operator := operatorID
	record := model.Organization{OrgName: strings.TrimSpace(input.OrgName), OrgType: strings.TrimSpace(input.OrgType), ParentID: input.ParentID, LeaderID: input.LeaderID, SortOrder: input.SortOrder, Status: status, CreatedBy: &operator, UpdatedBy: &operator}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "create", "organizations", &targetID, map[string]any{"event": "create_organization"}, ipAddress, userAgent))
	return &record, nil
}

func (s *OrgService) UpdateOrganization(ctx context.Context, operatorID, organizationID uint, input UpdateOrganizationInput, ipAddress string, userAgent string) (*model.Organization, error) {
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

	operator := operatorID
	updates := map[string]any{"org_name": strings.TrimSpace(input.OrgName), "org_type": strings.TrimSpace(input.OrgType), "parent_id": input.ParentID, "leader_id": input.LeaderID, "sort_order": input.SortOrder, "status": status, "updated_by": &operator, "updated_at": time.Now().Unix()}
	if err := s.db.WithContext(ctx).Model(&model.Organization{}).Where("id = ? AND deleted_at IS NULL", organizationID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", organizationID).First(&existing).Error; err != nil {
		return nil, fmt.Errorf("failed to reload organization: %w", err)
	}
	targetID := existing.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "organizations", &targetID, map[string]any{"event": "update_organization", "status": existing.Status}, ipAddress, userAgent))
	return &existing, nil
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

func (s *OrgService) CreateDepartment(ctx context.Context, operatorID uint, input CreateDepartmentInput, ipAddress string, userAgent string) (*model.Department, error) {
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
	}
	status := normalizeStatus(input.Status, "active")
	if !isValidDepartmentStatus(status) {
		return nil, ErrInvalidDepartmentStatus
	}

	operator := operatorID
	record := model.Department{DeptName: strings.TrimSpace(input.DeptName), OrganizationID: input.OrganizationID, ParentDeptID: input.ParentDeptID, LeaderID: input.LeaderID, SortOrder: input.SortOrder, Status: status, CreatedBy: &operator, UpdatedBy: &operator}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, fmt.Errorf("failed to create department: %w", err)
	}
	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "create", "departments", &targetID, map[string]any{"event": "create_department"}, ipAddress, userAgent))
	return &record, nil
}

func (s *OrgService) UpdateDepartment(ctx context.Context, operatorID, departmentID uint, input UpdateDepartmentInput, ipAddress string, userAgent string) (*model.Department, error) {
	if strings.TrimSpace(input.DeptName) == "" {
		return nil, ErrInvalidParam
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

	operator := operatorID
	updates := map[string]any{"dept_name": strings.TrimSpace(input.DeptName), "organization_id": input.OrganizationID, "parent_dept_id": input.ParentDeptID, "leader_id": input.LeaderID, "sort_order": input.SortOrder, "status": status, "updated_by": &operator, "updated_at": time.Now().Unix()}
	if err := s.db.WithContext(ctx).Model(&model.Department{}).Where("id = ? AND deleted_at IS NULL", departmentID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update department: %w", err)
	}
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", departmentID).First(&existing).Error; err != nil {
		return nil, fmt.Errorf("failed to reload department: %w", err)
	}
	targetID := existing.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "departments", &targetID, map[string]any{"event": "update_department", "status": existing.Status}, ipAddress, userAgent))
	return &existing, nil
}

type ListPositionLevelFilter struct {
	Status string
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

func (s *OrgService) CreateEmployee(ctx context.Context, operatorID uint, input CreateEmployeeInput, ipAddress string, userAgent string) (*model.Employee, error) {
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
	record := model.Employee{EmpName: strings.TrimSpace(input.EmpName), OrganizationID: input.OrganizationID, DepartmentID: input.DepartmentID, PositionLevelID: input.PositionLevelID, PositionTitle: strings.TrimSpace(input.PositionTitle), HireDate: input.HireDate, Status: status, CreatedBy: &operator, UpdatedBy: &operator}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, fmt.Errorf("failed to create employee: %w", err)
	}
	targetID := record.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "create", "employees", &targetID, map[string]any{"event": "create_employee"}, ipAddress, userAgent))
	return &record, nil
}

func (s *OrgService) UpdateEmployee(ctx context.Context, operatorID, employeeID uint, input UpdateEmployeeInput, ipAddress string, userAgent string) (*model.Employee, error) {
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

	var existing model.Employee
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", employeeID).First(&existing).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrEmployeeNotFound
		}
		return nil, fmt.Errorf("failed to query employee: %w", err)
	}

	operator := operatorID
	updates := map[string]any{"emp_name": strings.TrimSpace(input.EmpName), "organization_id": input.OrganizationID, "department_id": input.DepartmentID, "position_level_id": input.PositionLevelID, "position_title": strings.TrimSpace(input.PositionTitle), "hire_date": input.HireDate, "status": status, "updated_by": &operator, "updated_at": time.Now().Unix()}
	if err := s.db.WithContext(ctx).Model(&model.Employee{}).Where("id = ? AND deleted_at IS NULL", employeeID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update employee: %w", err)
	}
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", employeeID).First(&existing).Error; err != nil {
		return nil, fmt.Errorf("failed to reload employee: %w", err)
	}
	targetID := existing.ID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "employees", &targetID, map[string]any{"event": "update_employee", "status": existing.Status}, ipAddress, userAgent))
	return &existing, nil
}

func (s *OrgService) TransferEmployee(ctx context.Context, operatorID, employeeID uint, input TransferEmployeeInput, ipAddress string, userAgent string) (*model.Employee, error) {
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
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		history := model.EmployeeHistory{EmployeeID: employee.ID, ChangeType: strings.TrimSpace(input.ChangeType), OldOrganizationID: uintPtr(employee.OrganizationID), NewOrganizationID: uintPtr(newOrgID), OldDepartmentID: employee.DepartmentID, NewDepartmentID: newDeptID, OldPositionLevelID: uintPtr(employee.PositionLevelID), NewPositionLevelID: uintPtr(newPositionLevelID), OldPositionTitle: employee.PositionTitle, NewPositionTitle: newPositionTitle, ChangeReason: strings.TrimSpace(input.ChangeReason), EffectiveDate: *input.EffectiveDate, CreatedBy: &operator}
		if err := tx.Create(&history).Error; err != nil {
			return fmt.Errorf("failed to create employee history: %w", err)
		}

		updates := map[string]any{"organization_id": newOrgID, "department_id": newDeptID, "position_level_id": newPositionLevelID, "position_title": newPositionTitle, "updated_by": &operator, "updated_at": time.Now().Unix()}
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
	_ = s.auditRepo.Create(ctx, buildAuditRecord(&operator, "update", "employees", &targetID, map[string]any{"event": "transfer_employee", "changeType": strings.TrimSpace(input.ChangeType), "newOrganizationId": newOrgID, "effectiveDate": input.EffectiveDate.Format("2006-01-02")}, ipAddress, userAgent))
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

func isValidTransferType(changeType string) bool {
	switch changeType {
	case "transfer", "promotion", "demotion", "position_change":
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
