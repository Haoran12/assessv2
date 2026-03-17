package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"assessv2/backend/internal/api/response"
	"assessv2/backend/internal/middleware"
	"assessv2/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type RuleHandler struct {
	ruleService *service.RuleService
}

type upsertRuleVoteGroupRequest struct {
	GroupCode  string          `json:"groupCode"`
	GroupName  string          `json:"groupName"`
	Weight     float64         `json:"weight"`
	VoterType  string          `json:"voterType"`
	VoterScope json.RawMessage `json:"voterScope"`
	MaxScore   float64         `json:"maxScore"`
	SortOrder  int             `json:"sortOrder"`
	IsActive   *bool           `json:"isActive"`
}

type upsertRuleModuleRequest struct {
	ModuleCode        string                       `json:"moduleCode"`
	ModuleKey         string                       `json:"moduleKey"`
	ModuleName        string                       `json:"moduleName"`
	Weight            *float64                     `json:"weight"`
	MaxScore          *float64                     `json:"maxScore"`
	CalculationMethod string                       `json:"calculationMethod"`
	Expression        string                       `json:"expression"`
	ContextScope      json.RawMessage              `json:"contextScope"`
	SortOrder         int                          `json:"sortOrder"`
	IsActive          *bool                        `json:"isActive"`
	VoteGroups        []upsertRuleVoteGroupRequest `json:"voteGroups"`
}

type createRuleRequest struct {
	YearID         uint                      `json:"yearId"`
	PeriodCode     string                    `json:"periodCode"`
	ObjectType     string                    `json:"objectType"`
	ObjectCategory string                    `json:"objectCategory"`
	RuleName       string                    `json:"ruleName"`
	Description    string                    `json:"description"`
	IsActive       *bool                     `json:"isActive"`
	SyncQuarterly  bool                      `json:"syncQuarterly"`
	Modules        []upsertRuleModuleRequest `json:"modules"`
}

type updateRuleRequest struct {
	RuleName      string                    `json:"ruleName"`
	Description   string                    `json:"description"`
	IsActive      *bool                     `json:"isActive"`
	SyncQuarterly bool                      `json:"syncQuarterly"`
	Modules       []upsertRuleModuleRequest `json:"modules"`
}

type createTemplateRequest struct {
	TemplateName   string                    `json:"templateName"`
	ObjectType     string                    `json:"objectType"`
	ObjectCategory string                    `json:"objectCategory"`
	Description    string                    `json:"description"`
	Config         templateConfigRequestBody `json:"config"`
}

type templateConfigRequestBody struct {
	RuleName    string                    `json:"ruleName"`
	Description string                    `json:"description"`
	Modules     []upsertRuleModuleRequest `json:"modules"`
}

type createTemplateFromRuleRequest struct {
	TemplateName string `json:"templateName"`
	Description  string `json:"description"`
}

type applyTemplateRequest struct {
	YearID         uint   `json:"yearId"`
	PeriodCode     string `json:"periodCode"`
	ObjectType     string `json:"objectType"`
	ObjectCategory string `json:"objectCategory"`
	RuleName       string `json:"ruleName"`
	Description    string `json:"description"`
	SyncQuarterly  bool   `json:"syncQuarterly"`
	IsActive       *bool  `json:"isActive"`
	Overwrite      bool   `json:"overwrite"`
}

type upsertRuleBindingRequest struct {
	YearID       uint   `json:"yearId"`
	PeriodCode   string `json:"periodCode"`
	ObjectType   string `json:"objectType"`
	SegmentCode  string `json:"segmentCode"`
	OwnerScope   string `json:"ownerScope"`
	OwnerOrgType string `json:"ownerOrgType"`
	OwnerOrgID   *uint  `json:"ownerOrgId"`
	RuleID       uint   `json:"ruleId"`
	Priority     int    `json:"priority"`
	Description  string `json:"description"`
	IsActive     *bool  `json:"isActive"`
}

func NewRuleHandler(ruleService *service.RuleService) *RuleHandler {
	return &RuleHandler{ruleService: ruleService}
}

func (h *RuleHandler) ListRules(c *gin.Context) {
	var yearID *uint
	if value := strings.TrimSpace(c.Query("yearId")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid yearId")
			return
		}
		cast := uint(parsed)
		yearID = &cast
	}

	items, err := h.ruleService.ListRules(c.Request.Context(), service.ListRuleFilter{
		YearID:         yearID,
		PeriodCode:     strings.TrimSpace(c.Query("periodCode")),
		ObjectType:     strings.TrimSpace(c.Query("objectType")),
		ObjectCategory: strings.TrimSpace(c.Query("objectCategory")),
	})
	if err != nil {
		h.handleRuleError(c, err, "failed to query rules")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *RuleHandler) GetRule(c *gin.Context) {
	ruleID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid rule id")
		return
	}
	result, err := h.ruleService.GetRule(c.Request.Context(), ruleID)
	if err != nil {
		h.handleRuleError(c, err, "failed to query rule")
		return
	}
	response.Success(c, result)
}

func (h *RuleHandler) CreateRule(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	var req createRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid rule payload")
		return
	}
	modules := mapModuleRequests(req.Modules)
	result, err := h.ruleService.CreateRule(c.Request.Context(), claims, operatorID, service.CreateRuleInput{
		YearID:         req.YearID,
		PeriodCode:     strings.TrimSpace(req.PeriodCode),
		ObjectType:     strings.TrimSpace(req.ObjectType),
		ObjectCategory: strings.TrimSpace(req.ObjectCategory),
		RuleName:       strings.TrimSpace(req.RuleName),
		Description:    strings.TrimSpace(req.Description),
		IsActive:       boolOrDefault(req.IsActive, true),
		SyncQuarterly:  req.SyncQuarterly,
		Modules:        modules,
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleRuleError(c, err, "failed to create rule")
		return
	}
	response.Success(c, result)
}

func (h *RuleHandler) UpdateRule(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	ruleID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid rule id")
		return
	}
	var req updateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid rule payload")
		return
	}
	modules := mapModuleRequests(req.Modules)
	result, err := h.ruleService.UpdateRule(c.Request.Context(), claims, operatorID, ruleID, service.UpdateRuleInput{
		RuleName:      strings.TrimSpace(req.RuleName),
		Description:   strings.TrimSpace(req.Description),
		IsActive:      boolOrDefault(req.IsActive, true),
		SyncQuarterly: req.SyncQuarterly,
		Modules:       modules,
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleRuleError(c, err, "failed to update rule")
		return
	}
	response.Success(c, result)
}

func (h *RuleHandler) ListTemplates(c *gin.Context) {
	items, err := h.ruleService.ListTemplates(c.Request.Context(), service.ListRuleTemplateFilter{
		ObjectType:     strings.TrimSpace(c.Query("objectType")),
		ObjectCategory: strings.TrimSpace(c.Query("objectCategory")),
	})
	if err != nil {
		h.handleRuleError(c, err, "failed to query templates")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *RuleHandler) CreateTemplate(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	var req createTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid template payload")
		return
	}
	modules := mapModuleRequests(req.Config.Modules)
	result, err := h.ruleService.CreateTemplate(c.Request.Context(), claims, operatorID, service.CreateRuleTemplateInput{
		TemplateName:   strings.TrimSpace(req.TemplateName),
		ObjectType:     strings.TrimSpace(req.ObjectType),
		ObjectCategory: strings.TrimSpace(req.ObjectCategory),
		Description:    strings.TrimSpace(req.Description),
		Config: service.RuleTemplateConfig{
			RuleName:    strings.TrimSpace(req.Config.RuleName),
			Description: strings.TrimSpace(req.Config.Description),
			Modules:     modules,
		},
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleRuleError(c, err, "failed to create template")
		return
	}
	response.Success(c, result)
}

func (h *RuleHandler) CreateTemplateFromRule(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	ruleID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid rule id")
		return
	}
	var req createTemplateFromRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid template payload")
		return
	}
	result, err := h.ruleService.CreateTemplateFromRule(c.Request.Context(), claims, operatorID, ruleID, strings.TrimSpace(req.TemplateName), strings.TrimSpace(req.Description), c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleRuleError(c, err, "failed to create template from rule")
		return
	}
	response.Success(c, result)
}

func (h *RuleHandler) ApplyTemplate(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	templateID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid template id")
		return
	}
	var req applyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid apply template payload")
		return
	}
	result, err := h.ruleService.ApplyTemplate(c.Request.Context(), claims, operatorID, templateID, service.ApplyRuleTemplateInput{
		YearID:         req.YearID,
		PeriodCode:     strings.TrimSpace(req.PeriodCode),
		ObjectType:     strings.TrimSpace(req.ObjectType),
		ObjectCategory: strings.TrimSpace(req.ObjectCategory),
		RuleName:       strings.TrimSpace(req.RuleName),
		Description:    strings.TrimSpace(req.Description),
		SyncQuarterly:  req.SyncQuarterly,
		IsActive:       boolOrDefault(req.IsActive, true),
		Overwrite:      req.Overwrite,
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleRuleError(c, err, "failed to apply template")
		return
	}
	response.Success(c, result)
}

func (h *RuleHandler) ListBindings(c *gin.Context) {
	yearID, err := parseOptionalUintQuery(c.Query("yearId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid yearId")
		return
	}
	ownerOrgID, err := parseOptionalUintQuery(c.Query("ownerOrgId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid ownerOrgId")
		return
	}
	ruleID, err := parseOptionalUintQuery(c.Query("ruleId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid ruleId")
		return
	}

	var isActive *bool
	if raw := strings.TrimSpace(c.Query("isActive")); raw != "" {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid isActive")
			return
		}
		isActive = &parsed
	}

	items, err := h.ruleService.ListRuleBindings(c.Request.Context(), service.ListRuleBindingFilter{
		YearID:       yearID,
		PeriodCode:   strings.TrimSpace(c.Query("periodCode")),
		ObjectType:   strings.TrimSpace(c.Query("objectType")),
		SegmentCode:  strings.TrimSpace(c.Query("segmentCode")),
		OwnerScope:   strings.TrimSpace(c.Query("ownerScope")),
		OwnerOrgType: strings.TrimSpace(c.Query("ownerOrgType")),
		OwnerOrgID:   ownerOrgID,
		RuleID:       ruleID,
		IsActive:     isActive,
	})
	if err != nil {
		h.handleRuleError(c, err, "failed to query rule bindings")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *RuleHandler) CreateBinding(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	var req upsertRuleBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid rule binding payload")
		return
	}

	result, err := h.ruleService.CreateRuleBinding(c.Request.Context(), claims, operatorID, service.CreateRuleBindingInput{
		YearID:       req.YearID,
		PeriodCode:   strings.TrimSpace(req.PeriodCode),
		ObjectType:   strings.TrimSpace(req.ObjectType),
		SegmentCode:  strings.TrimSpace(req.SegmentCode),
		OwnerScope:   strings.TrimSpace(req.OwnerScope),
		OwnerOrgType: strings.TrimSpace(req.OwnerOrgType),
		OwnerOrgID:   req.OwnerOrgID,
		RuleID:       req.RuleID,
		Priority:     req.Priority,
		Description:  strings.TrimSpace(req.Description),
		IsActive:     boolOrDefault(req.IsActive, true),
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleRuleError(c, err, "failed to create rule binding")
		return
	}
	response.Success(c, result)
}

func (h *RuleHandler) UpdateBinding(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	bindingID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid binding id")
		return
	}

	var req upsertRuleBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidPayload, "invalid rule binding payload")
		return
	}
	result, err := h.ruleService.UpdateRuleBinding(c.Request.Context(), claims, operatorID, bindingID, service.UpdateRuleBindingInput{
		YearID:       req.YearID,
		PeriodCode:   strings.TrimSpace(req.PeriodCode),
		ObjectType:   strings.TrimSpace(req.ObjectType),
		SegmentCode:  strings.TrimSpace(req.SegmentCode),
		OwnerScope:   strings.TrimSpace(req.OwnerScope),
		OwnerOrgType: strings.TrimSpace(req.OwnerOrgType),
		OwnerOrgID:   req.OwnerOrgID,
		RuleID:       req.RuleID,
		Priority:     req.Priority,
		Description:  strings.TrimSpace(req.Description),
		IsActive:     boolOrDefault(req.IsActive, true),
	}, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.handleRuleError(c, err, "failed to update rule binding")
		return
	}
	response.Success(c, result)
}

func (h *RuleHandler) DeleteBinding(c *gin.Context) {
	operatorID, ok := operatorFromClaims(c)
	if !ok {
		return
	}
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "missing auth context")
		return
	}
	bindingID, err := parseUserIDParam(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, "invalid binding id")
		return
	}
	if err := h.ruleService.DeleteRuleBinding(c.Request.Context(), claims, operatorID, bindingID, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		h.handleRuleError(c, err, "failed to delete rule binding")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *RuleHandler) handleRuleError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, service.ErrInvalidParam),
		errors.Is(err, service.ErrInvalidOrganizationType),
		errors.Is(err, service.ErrInvalidYearStatus),
		errors.Is(err, service.ErrInvalidPeriodStatus),
		errors.Is(err, service.ErrInvalidRulePeriodCode),
		errors.Is(err, service.ErrInvalidRuleObjectType),
		errors.Is(err, service.ErrInvalidRuleObjectCategory),
		errors.Is(err, service.ErrInvalidRuleName),
		errors.Is(err, service.ErrInvalidRuleModules),
		errors.Is(err, service.ErrInvalidRuleBindingScope),
		errors.Is(err, service.ErrRuleWeightSumInvalid),
		errors.Is(err, service.ErrVoteGroupWeightInvalid),
		errors.Is(err, service.ErrInvalidModuleCode),
		errors.Is(err, service.ErrInvalidExpression),
		errors.Is(err, service.ErrRuleTemplateNameInvalid):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestInvalidParam, err.Error())
	case errors.Is(err, service.ErrRuleAlreadyExists),
		errors.Is(err, service.ErrAssessmentReadOnly):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequestBusinessRule, err.Error())
	case errors.Is(err, service.ErrForbidden):
		response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
	case errors.Is(err, service.ErrRuleNotFound),
		errors.Is(err, service.ErrRuleBindingNotFound),
		errors.Is(err, service.ErrRuleTemplateNotFound),
		errors.Is(err, service.ErrYearNotFound),
		errors.Is(err, service.ErrPeriodNotFound),
		errors.Is(err, service.ErrOrganizationNotFound):
		response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, fallback)
	}
}

func mapModuleRequests(items []upsertRuleModuleRequest) []service.RuleModuleInput {
	result := make([]service.RuleModuleInput, 0, len(items))
	for _, item := range items {
		module := service.RuleModuleInput{
			ModuleCode:        strings.TrimSpace(item.ModuleCode),
			ModuleKey:         strings.TrimSpace(item.ModuleKey),
			ModuleName:        strings.TrimSpace(item.ModuleName),
			Weight:            item.Weight,
			MaxScore:          item.MaxScore,
			CalculationMethod: strings.TrimSpace(item.CalculationMethod),
			Expression:        strings.TrimSpace(item.Expression),
			ContextScope:      normalizeRawJSON(item.ContextScope),
			SortOrder:         item.SortOrder,
			IsActive:          boolOrDefault(item.IsActive, true),
		}
		if len(item.VoteGroups) > 0 {
			module.VoteGroups = make([]service.RuleVoteGroupInput, 0, len(item.VoteGroups))
			for _, group := range item.VoteGroups {
				module.VoteGroups = append(module.VoteGroups, service.RuleVoteGroupInput{
					GroupCode:  strings.TrimSpace(group.GroupCode),
					GroupName:  strings.TrimSpace(group.GroupName),
					Weight:     group.Weight,
					VoterType:  strings.TrimSpace(group.VoterType),
					VoterScope: normalizeRawJSON(group.VoterScope),
					MaxScore:   group.MaxScore,
					SortOrder:  group.SortOrder,
					IsActive:   boolOrDefault(group.IsActive, true),
				})
			}
		}
		result = append(result, module)
	}
	return result
}

func boolOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func normalizeRawJSON(raw json.RawMessage) string {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return ""
	}
	return text
}
