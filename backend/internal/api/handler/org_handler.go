package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/middleware"
	"assessv2/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type OrgHandler struct {
	orgService *service.OrgService
}

func NewOrgHandler(orgService *service.OrgService) *OrgHandler {
	return &OrgHandler{orgService: orgService}
}

type upsertOrganizationRequest struct {
	OrgName   string `json:"orgName"`
	OrgType   string `json:"orgType"`
	ParentID  *uint  `json:"parentId"`
	LeaderID  *uint  `json:"leaderId"`
	SortOrder int    `json:"sortOrder"`
	Status    string `json:"status"`
}

type upsertDepartmentRequest struct {
	DeptName       string `json:"deptName"`
	OrganizationID uint   `json:"organizationId"`
	ParentDeptID   *uint  `json:"parentDeptId"`
	LeaderID       *uint  `json:"leaderId"`
	SortOrder      int    `json:"sortOrder"`
	Status         string `json:"status"`
}

type upsertEmployeeRequest struct {
	EmpName         string `json:"empName"`
	OrganizationID  uint   `json:"organizationId"`
	DepartmentID    *uint  `json:"departmentId"`
	PositionLevelID uint   `json:"positionLevelId"`
	PositionTitle   string `json:"positionTitle"`
	HireDate        string `json:"hireDate"`
	Status          string `json:"status"`
}

type upsertPositionLevelRequest struct {
	LevelCode       string `json:"levelCode"`
	LevelName       string `json:"levelName"`
	Description     string `json:"description"`
	IsForAssessment *bool  `json:"isForAssessment"`
	SortOrder       int    `json:"sortOrder"`
	Status          string `json:"status"`
}

type transferEmployeeRequest struct {
	ChangeType         string  `json:"changeType"`
	NewOrganizationID  *uint   `json:"newOrganizationId"`
	NewDepartmentID    *uint   `json:"newDepartmentId"`
	NewPositionLevelID *uint   `json:"newPositionLevelId"`
	NewPositionTitle   *string `json:"newPositionTitle"`
	ChangeReason       string  `json:"changeReason"`
	EffectiveDate      string  `json:"effectiveDate"`
}

func (h *OrgHandler) Tree(c *gin.Context) {
	includeInactive := strings.EqualFold(strings.TrimSpace(c.DefaultQuery("includeInactive", "false")), "true")
	result, err := h.orgService.Tree(c.Request.Context(), includeInactive)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query organization tree")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *OrgHandler) ListOrganizations(c *gin.Context) {
	result, err := h.orgService.ListOrganizations(c.Request.Context(), service.ListOrganizationFilter{Status: c.Query("status"), Keyword: c.Query("keyword")})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query organizations")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *OrgHandler) CreateOrganization(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	var req upsertOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid organization payload")
		return
	}
	result, err := h.orgService.CreateOrganization(c.Request.Context(), operatorID, service.CreateOrganizationInput{OrgName: strings.TrimSpace(req.OrgName), OrgType: strings.TrimSpace(req.OrgType), ParentID: req.ParentID, LeaderID: req.LeaderID, SortOrder: req.SortOrder, Status: strings.TrimSpace(req.Status)}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleOrgError(c, err, "failed to create organization")
		return
	}
	response.Success(c, result)
}

func (h *OrgHandler) UpdateOrganization(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	organizationID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid organization id")
		return
	}
	var req upsertOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid organization payload")
		return
	}
	result, err := h.orgService.UpdateOrganization(c.Request.Context(), operatorID, organizationID, service.UpdateOrganizationInput{OrgName: strings.TrimSpace(req.OrgName), OrgType: strings.TrimSpace(req.OrgType), ParentID: req.ParentID, LeaderID: req.LeaderID, SortOrder: req.SortOrder, Status: strings.TrimSpace(req.Status)}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleOrgError(c, err, "failed to update organization")
		return
	}
	response.Success(c, result)
}

func (h *OrgHandler) DeleteOrganization(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	organizationID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid organization id")
		return
	}
	if err := h.orgService.DeleteOrganization(c.Request.Context(), operatorID, organizationID, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		h.handleOrgError(c, err, "failed to delete organization")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *OrgHandler) ListDepartments(c *gin.Context) {
	var organizationID *uint
	if value := strings.TrimSpace(c.Query("organizationId")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid organizationId")
			return
		}
		cast := uint(parsed)
		organizationID = &cast
	}
	result, err := h.orgService.ListDepartments(c.Request.Context(), service.ListDepartmentFilter{OrganizationID: organizationID, Status: c.Query("status"), Keyword: c.Query("keyword")})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query departments")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *OrgHandler) CreateDepartment(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	var req upsertDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid department payload")
		return
	}
	result, err := h.orgService.CreateDepartment(c.Request.Context(), operatorID, service.CreateDepartmentInput{DeptName: strings.TrimSpace(req.DeptName), OrganizationID: req.OrganizationID, ParentDeptID: req.ParentDeptID, LeaderID: req.LeaderID, SortOrder: req.SortOrder, Status: strings.TrimSpace(req.Status)}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleOrgError(c, err, "failed to create department")
		return
	}
	response.Success(c, result)
}

func (h *OrgHandler) UpdateDepartment(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	departmentID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid department id")
		return
	}
	var req upsertDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid department payload")
		return
	}
	result, err := h.orgService.UpdateDepartment(c.Request.Context(), operatorID, departmentID, service.UpdateDepartmentInput{DeptName: strings.TrimSpace(req.DeptName), OrganizationID: req.OrganizationID, ParentDeptID: req.ParentDeptID, LeaderID: req.LeaderID, SortOrder: req.SortOrder, Status: strings.TrimSpace(req.Status)}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleOrgError(c, err, "failed to update department")
		return
	}
	response.Success(c, result)
}

func (h *OrgHandler) DeleteDepartment(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	departmentID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid department id")
		return
	}
	if err := h.orgService.DeleteDepartment(c.Request.Context(), operatorID, departmentID, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		h.handleOrgError(c, err, "failed to delete department")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *OrgHandler) ListPositionLevels(c *gin.Context) {
	result, err := h.orgService.ListPositionLevels(c.Request.Context(), service.ListPositionLevelFilter{Status: c.Query("status")})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query position levels")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *OrgHandler) ListAssessmentCategories(c *gin.Context) {
	result, err := h.orgService.ListAssessmentCategories(c.Request.Context(), service.ListAssessmentCategoryFilter{
		ObjectType: strings.TrimSpace(c.Query("objectType")),
		Status:     strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		h.handleOrgError(c, err, "failed to query assessment categories")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *OrgHandler) CreatePositionLevel(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	var req upsertPositionLevelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid position level payload")
		return
	}
	result, err := h.orgService.CreatePositionLevel(c.Request.Context(), operatorID, service.CreatePositionLevelInput{
		LevelCode:       strings.TrimSpace(req.LevelCode),
		LevelName:       strings.TrimSpace(req.LevelName),
		Description:     strings.TrimSpace(req.Description),
		IsForAssessment: req.IsForAssessment,
		SortOrder:       req.SortOrder,
		Status:          strings.TrimSpace(req.Status),
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleOrgError(c, err, "failed to create position level")
		return
	}
	response.Success(c, result)
}

func (h *OrgHandler) UpdatePositionLevel(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	positionLevelID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid position level id")
		return
	}
	var req upsertPositionLevelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid position level payload")
		return
	}
	result, err := h.orgService.UpdatePositionLevel(c.Request.Context(), operatorID, positionLevelID, service.UpdatePositionLevelInput{
		LevelCode:       strings.TrimSpace(req.LevelCode),
		LevelName:       strings.TrimSpace(req.LevelName),
		Description:     strings.TrimSpace(req.Description),
		IsForAssessment: req.IsForAssessment,
		SortOrder:       req.SortOrder,
		Status:          strings.TrimSpace(req.Status),
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleOrgError(c, err, "failed to update position level")
		return
	}
	response.Success(c, result)
}

func (h *OrgHandler) DeletePositionLevel(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	positionLevelID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid position level id")
		return
	}
	if err := h.orgService.DeletePositionLevel(c.Request.Context(), operatorID, positionLevelID, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		h.handleOrgError(c, err, "failed to delete position level")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *OrgHandler) ListEmployees(c *gin.Context) {
	var organizationID *uint
	if value := strings.TrimSpace(c.Query("organizationId")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid organizationId")
			return
		}
		cast := uint(parsed)
		organizationID = &cast
	}
	var departmentID *uint
	if value := strings.TrimSpace(c.Query("departmentId")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid departmentId")
			return
		}
		cast := uint(parsed)
		departmentID = &cast
	}
	result, err := h.orgService.ListEmployees(c.Request.Context(), service.ListEmployeeFilter{OrganizationID: organizationID, DepartmentID: departmentID, Status: c.Query("status"), Keyword: c.Query("keyword")})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "failed to query employees")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *OrgHandler) CreateEmployee(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	var req upsertEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid employee payload")
		return
	}
	hireDate, err := parseDateOrNil(req.HireDate)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid hireDate")
		return
	}
	result, err := h.orgService.CreateEmployee(c.Request.Context(), operatorID, service.CreateEmployeeInput{EmpName: strings.TrimSpace(req.EmpName), OrganizationID: req.OrganizationID, DepartmentID: req.DepartmentID, PositionLevelID: req.PositionLevelID, PositionTitle: strings.TrimSpace(req.PositionTitle), HireDate: hireDate, Status: strings.TrimSpace(req.Status)}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleOrgError(c, err, "failed to create employee")
		return
	}
	response.Success(c, result)
}

func (h *OrgHandler) UpdateEmployee(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	employeeID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid employee id")
		return
	}
	var req upsertEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid employee payload")
		return
	}
	hireDate, err := parseDateOrNil(req.HireDate)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid hireDate")
		return
	}
	result, err := h.orgService.UpdateEmployee(c.Request.Context(), operatorID, employeeID, service.UpdateEmployeeInput{EmpName: strings.TrimSpace(req.EmpName), OrganizationID: req.OrganizationID, DepartmentID: req.DepartmentID, PositionLevelID: req.PositionLevelID, PositionTitle: strings.TrimSpace(req.PositionTitle), HireDate: hireDate, Status: strings.TrimSpace(req.Status)}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleOrgError(c, err, "failed to update employee")
		return
	}
	response.Success(c, result)
}

func (h *OrgHandler) DeleteEmployee(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	employeeID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid employee id")
		return
	}
	if err := h.orgService.DeleteEmployee(c.Request.Context(), operatorID, employeeID, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		h.handleOrgError(c, err, "failed to delete employee")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *OrgHandler) TransferEmployee(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	employeeID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid employee id")
		return
	}
	var req transferEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid transfer payload")
		return
	}
	effectiveDate, err := parseDateOrNil(req.EffectiveDate)
	if err != nil || effectiveDate == nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid effectiveDate")
		return
	}
	result, err := h.orgService.TransferEmployee(c.Request.Context(), operatorID, employeeID, service.TransferEmployeeInput{ChangeType: strings.TrimSpace(req.ChangeType), NewOrganizationID: req.NewOrganizationID, NewDepartmentID: req.NewDepartmentID, NewPositionLevelID: req.NewPositionLevelID, NewPositionTitle: req.NewPositionTitle, ChangeReason: strings.TrimSpace(req.ChangeReason), EffectiveDate: effectiveDate}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleOrgError(c, err, "failed to transfer employee")
		return
	}
	response.Success(c, result)
}

func (h *OrgHandler) EmployeeHistory(c *gin.Context) {
	employeeID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid employee id")
		return
	}
	result, err := h.orgService.ListEmployeeHistory(c.Request.Context(), employeeID)
	if err != nil {
		h.handleOrgError(c, err, "failed to query employee history")
		return
	}
	response.Success(c, gin.H{"items": result})
}

func (h *OrgHandler) handleOrgError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrInvalidParam),
		errors.Is(err, service.ErrInvalidOrganizationType),
		errors.Is(err, service.ErrInvalidOrganizationStatus),
		errors.Is(err, service.ErrInvalidDepartmentStatus),
		errors.Is(err, service.ErrInvalidRuleObjectType),
		errors.Is(err, service.ErrInvalidPositionLevelStatus),
		errors.Is(err, service.ErrPositionLevelCodeExists),
		errors.Is(err, service.ErrInvalidEmployeeStatus),
		errors.Is(err, service.ErrInvalidTransferType),
		errors.Is(err, service.ErrInvalidEffectiveDate):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
	case errors.Is(err, service.ErrSystemPositionLevelLocked),
		errors.Is(err, service.ErrOrganizationInUse),
		errors.Is(err, service.ErrDepartmentInUse),
		errors.Is(err, service.ErrPositionLevelInUse):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
	case errors.Is(err, service.ErrOrganizationNotFound),
		errors.Is(err, service.ErrDepartmentNotFound),
		errors.Is(err, service.ErrPositionLevelNotFound),
		errors.Is(err, service.ErrEmployeeNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, fallback)
	}
}

func operatorFromClaims(c *gin.Context) (uint, bool) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return 0, false
	}
	return claims.UserID, true
}

func parseDateOrNil(value string) (*time.Time, error) {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", text)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
