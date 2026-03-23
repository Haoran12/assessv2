package service

import (
	"context"
	"sort"
	"strings"
	"unicode"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
)

type RuleExpressionVariable struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	InsertText  string `json:"insertText"`
}

type RuleExpressionFunction struct {
	Name        string `json:"name"`
	Signature   string `json:"signature"`
	ReturnType  string `json:"returnType"`
	Description string `json:"description"`
	InsertText  string `json:"insertText"`
}

type RuleExpressionObjectOption struct {
	ObjectID       uint   `json:"objectId"`
	ObjectName     string `json:"objectName"`
	ObjectType     string `json:"objectType"`
	GroupCode      string `json:"groupCode"`
	TargetType     string `json:"targetType"`
	TargetID       uint   `json:"targetId"`
	ParentObjectID *uint  `json:"parentObjectId,omitempty"`
	IsPriority     bool   `json:"isPriority"`
}

type RuleExpressionContext struct {
	AssessmentID    uint                         `json:"assessmentId"`
	PeriodCode      string                       `json:"periodCode,omitempty"`
	ObjectGroupCode string                       `json:"objectGroupCode,omitempty"`
	ModuleVariables []RuleExpressionVariable     `json:"moduleVariables"`
	GradeVariables  []RuleExpressionVariable     `json:"gradeVariables"`
	Functions       []RuleExpressionFunction     `json:"functions"`
	Periods         []string                     `json:"periods"`
	Objects         []RuleExpressionObjectOption `json:"objects"`
}

func (s *RuleManagementService) GetRuleExpressionContext(
	ctx context.Context,
	claims *auth.Claims,
	assessmentID uint,
	periodCode string,
	objectGroupCode string,
) (*RuleExpressionContext, error) {
	if assessmentID == 0 {
		return nil, ErrInvalidParam
	}
	session, err := s.loadSessionSummary(ctx, assessmentID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, session.OrganizationID); err != nil {
		return nil, err
	}

	periods, err := s.listSessionPeriods(ctx, assessmentID)
	if err != nil {
		return nil, err
	}
	objects, err := s.listSessionObjects(ctx, assessmentID)
	if err != nil {
		return nil, err
	}

	normalizedPeriod := strings.ToUpper(strings.TrimSpace(periodCode))
	normalizedGroup := strings.TrimSpace(objectGroupCode)

	periodList := make([]string, 0, len(periods))
	for _, period := range periods {
		code := strings.ToUpper(strings.TrimSpace(period.PeriodCode))
		if code == "" {
			continue
		}
		periodList = append(periodList, code)
	}

	moduleKeys := make([]string, 0, 8)
	record, err := s.ensureSessionRuleFile(ctx, session, nil)
	if err == nil {
		if parsed, parseErr := parseCalculationRuleContent(record.ContentJSON); parseErr == nil {
			scoped := matchScopedRule(parsed, normalizedPeriod, normalizedGroup)
			if scoped == nil && len(parsed.ScopedRules) > 0 {
				scoped = &parsed.ScopedRules[0]
			}
			if scoped != nil {
				moduleKeys = collectRuleModuleKeys(scoped.ScoreModules)
			}
		}
	}

	priorityObjectIDs := buildExpressionPriorityObjectIDs(objects, normalizedGroup)
	objectOptions := make([]RuleExpressionObjectOption, 0, len(objects))
	for _, object := range objects {
		if !object.IsActive {
			continue
		}
		_, isPriority := priorityObjectIDs[object.ID]
		objectOptions = append(objectOptions, RuleExpressionObjectOption{
			ObjectID:       object.ID,
			ObjectName:     object.ObjectName,
			ObjectType:     object.ObjectType,
			GroupCode:      object.GroupCode,
			TargetType:     object.TargetType,
			TargetID:       object.TargetID,
			ParentObjectID: object.ParentObjectID,
			IsPriority:     isPriority,
		})
	}
	sort.SliceStable(objectOptions, func(i, j int) bool {
		left := objectOptions[i]
		right := objectOptions[j]
		if left.IsPriority != right.IsPriority {
			return left.IsPriority
		}
		if normalizedGroup != "" {
			leftInCurrentGroup := left.GroupCode == normalizedGroup
			rightInCurrentGroup := right.GroupCode == normalizedGroup
			if leftInCurrentGroup != rightInCurrentGroup {
				return leftInCurrentGroup
			}
		}
		if left.GroupCode != right.GroupCode {
			return left.GroupCode < right.GroupCode
		}
		if left.ObjectType != right.ObjectType {
			return left.ObjectType < right.ObjectType
		}
		if left.ObjectID != right.ObjectID {
			return left.ObjectID < right.ObjectID
		}
		return left.ObjectName < right.ObjectName
	})

	return &RuleExpressionContext{
		AssessmentID:    assessmentID,
		PeriodCode:      normalizedPeriod,
		ObjectGroupCode: normalizedGroup,
		ModuleVariables: buildModuleExpressionVariables(moduleKeys),
		GradeVariables:  buildGradeExpressionVariables(moduleKeys),
		Functions:       buildExpressionFunctions(moduleKeys),
		Periods:         periodList,
		Objects:         objectOptions,
	}, nil
}

func buildModuleExpressionVariables(moduleKeys []string) []RuleExpressionVariable {
	result := []RuleExpressionVariable{
		{Name: "periodCode", Type: "string", Description: "Current calculation period code", InsertText: "periodCode"},
		{Name: "objectId", Type: "number", Description: "Current assessment object ID", InsertText: "objectId"},
		{Name: "groupCode", Type: "string", Description: "Current assessment object group code", InsertText: "groupCode"},
		{Name: "objectType", Type: "string", Description: "Current object type (team/individual)", InsertText: "objectType"},
		{Name: "targetId", Type: "number", Description: "Linked business target ID of current object", InsertText: "targetId"},
		{Name: "targetType", Type: "string", Description: "Linked business target type of current object", InsertText: "targetType"},
		{Name: "parentObjectId", Type: "number", Description: "Parent object ID in this session (0 means none)", InsertText: "parentObjectId"},
		{Name: "extraAdjust", Type: "number", Description: "Extra adjustment module score (__extra_adjust__)", InsertText: "extraAdjust"},
		{Name: "moduleScores", Type: "map<string,number>", Description: "Calculated module scores keyed by moduleKey", InsertText: `moduleScores["module_key"]`},
		{Name: "rawModuleScores", Type: "map<string,number>", Description: "Raw input module scores keyed by moduleKey", InsertText: `rawModuleScores["module_key"]`},
	}
	return append(result, buildModuleKeyVariables(moduleKeys)...)
}

func buildGradeExpressionVariables(moduleKeys []string) []RuleExpressionVariable {
	result := []RuleExpressionVariable{
		{Name: "periodCode", Type: "string", Description: "Current calculation period code", InsertText: "periodCode"},
		{Name: "objectId", Type: "number", Description: "Current assessment object ID", InsertText: "objectId"},
		{Name: "groupKey", Type: "string", Description: "Current group code", InsertText: "groupKey"},
		{Name: "objectType", Type: "string", Description: "Current object type (team/individual)", InsertText: "objectType"},
		{Name: "targetId", Type: "number", Description: "Linked business target ID of current object", InsertText: "targetId"},
		{Name: "targetType", Type: "string", Description: "Linked business target type of current object", InsertText: "targetType"},
		{Name: "parentObjectId", Type: "number", Description: "Parent object ID in this session (0 means none)", InsertText: "parentObjectId"},
		{Name: "totalScore", Type: "number", Description: "Current object total score", InsertText: "totalScore"},
		{Name: "rank", Type: "number", Description: "Current object rank inside the group", InsertText: "rank"},
		{Name: "extraAdjust", Type: "number", Description: "Current object extra adjustment score", InsertText: "extraAdjust"},
		{Name: "moduleScores", Type: "map<string,number>", Description: "Current object module scores keyed by moduleKey", InsertText: `moduleScores["module_key"]`},
	}
	return append(result, buildModuleKeyVariables(moduleKeys)...)
}

func buildModuleKeyVariables(moduleKeys []string) []RuleExpressionVariable {
	result := make([]RuleExpressionVariable, 0, len(moduleKeys)*2)
	for _, key := range moduleKeys {
		moduleKey := strings.TrimSpace(key)
		if moduleKey == "" {
			continue
		}
		result = append(result, RuleExpressionVariable{
			Name:        `moduleScores["` + moduleKey + `"]`,
			Type:        "number",
			Description: "Module score (moduleKey=" + moduleKey + ")",
			InsertText:  `moduleScores["` + moduleKey + `"]`,
		})
		if isExpressionIdentifier(moduleKey) {
			result = append(result, RuleExpressionVariable{
				Name:        moduleKey,
				Type:        "number",
				Description: "Module score alias (moduleKey=" + moduleKey + ")",
				InsertText:  moduleKey,
			})
		}
	}
	return result
}

func buildExpressionFunctions(moduleKeys []string) []RuleExpressionFunction {
	moduleKeyForInsert := "module_key"
	if len(moduleKeys) > 0 && strings.TrimSpace(moduleKeys[0]) != "" {
		moduleKeyForInsert = strings.TrimSpace(moduleKeys[0])
	}
	return []RuleExpressionFunction{
		{
			Name:        "score",
			Signature:   "score(periodCode, objectId)",
			ReturnType:  "number",
			Description: "Read total score inside current assessment session; returns 0 if not found",
			InsertText:  `score(periodCode, objectId)`,
		},
		{
			Name:        "moduleScore",
			Signature:   "moduleScore(periodCode, objectId, moduleKey)",
			ReturnType:  "number",
			Description: "Read module score inside current assessment session; returns 0 if not found",
			InsertText:  `moduleScore(periodCode, objectId, "` + moduleKeyForInsert + `")`,
		},
		{
			Name:        "targetScore",
			Signature:   "targetScore(periodCode, targetType, targetId)",
			ReturnType:  "number",
			Description: "Read total score by business target in current session; returns 0 if not found",
			InsertText:  `targetScore(periodCode, targetType, targetId)`,
		},
		{
			Name:        "hasScore",
			Signature:   "hasScore(periodCode, objectId)",
			ReturnType:  "bool",
			Description: "Check whether total score exists in current session",
			InsertText:  `hasScore(periodCode, objectId)`,
		},
	}
}

func buildExpressionPriorityObjectIDs(
	objects []model.AssessmentSessionObject,
	objectGroupCode string,
) map[uint]struct{} {
	result := make(map[uint]struct{}, len(objects))
	targetGroupCode := strings.TrimSpace(objectGroupCode)
	if targetGroupCode == "" {
		return result
	}
	activeByID := make(map[uint]model.AssessmentSessionObject, len(objects))
	for _, object := range objects {
		if !object.IsActive {
			continue
		}
		activeByID[object.ID] = object
	}
	for _, object := range activeByID {
		if strings.TrimSpace(object.GroupCode) != targetGroupCode {
			continue
		}
		result[object.ID] = struct{}{}
		if object.ParentObjectID == nil || *object.ParentObjectID == 0 {
			continue
		}
		if _, exists := activeByID[*object.ParentObjectID]; exists {
			result[*object.ParentObjectID] = struct{}{}
		}
	}
	return result
}

func collectRuleModuleKeys(modules []calculationScoreNode) []string {
	seen := map[string]struct{}{}
	keys := make([]string, 0, len(modules))
	for _, module := range modules {
		key := strings.TrimSpace(module.ModuleKey)
		if key == "" {
			key = strings.TrimSpace(module.ID)
		}
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func isExpressionIdentifier(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" {
		return false
	}
	for index, r := range text {
		if index == 0 {
			if !(r == '_' || unicode.IsLetter(r)) {
				return false
			}
			continue
		}
		if !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}
