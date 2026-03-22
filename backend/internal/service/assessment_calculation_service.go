package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const extraAdjustModuleKey = "__extra_adjust__"

type CalculatedAssessmentObject struct {
	model.AssessmentSessionObject
	ModuleScores map[string]*float64 `json:"moduleScores,omitempty"`
	TotalScore   *float64            `json:"totalScore,omitempty"`
	Rank         *int                `json:"rank,omitempty"`
	Grade        string              `json:"grade,omitempty"`
	ScoreSource  string              `json:"scoreSource,omitempty"`
}

type SessionObjectModuleScoreUpsertItem struct {
	PeriodCode string  `json:"periodCode"`
	ObjectID   uint    `json:"objectId"`
	ModuleKey  string  `json:"moduleKey"`
	Score      float64 `json:"score"`
}

type calculationRuleContent struct {
	Version      int                 `json:"version"`
	ScopedRules  []calculationScoped `json:"scopedRules"`
	Dependencies []map[string]any    `json:"dependencies"`
	Raw          map[string]any      `json:"-"`
}

type calculationScoped struct {
	ID                    string                 `json:"id"`
	ApplicablePeriods     []string               `json:"applicablePeriods"`
	ApplicableObjectGroup []string               `json:"applicableObjectGroups"`
	ScoreModules          []calculationScoreNode `json:"scoreModules"`
	Grades                []calculationGradeRule `json:"grades"`
}

type calculationScoreNode struct {
	ID                string  `json:"id"`
	ModuleKey         string  `json:"moduleKey"`
	ModuleName        string  `json:"moduleName"`
	Weight            float64 `json:"weight"`
	CalculationMethod string  `json:"calculationMethod"`
}

type calculationGradeRule struct {
	ID              string                    `json:"id"`
	Title           string                    `json:"title"`
	ScoreNode       calculationGradeScoreNode `json:"scoreNode"`
	ConditionLogic  string                    `json:"conditionLogic"`
	MaxRatioPercent *float64                  `json:"maxRatioPercent"`
}

type calculationGradeScoreNode struct {
	HasUpperLimit bool    `json:"hasUpperLimit"`
	UpperScore    float64 `json:"upperScore"`
	UpperOperator string  `json:"upperOperator"`
	HasLowerLimit bool    `json:"hasLowerLimit"`
	LowerScore    float64 `json:"lowerScore"`
	LowerOperator string  `json:"lowerOperator"`
}

type calculationEdge struct {
	From string
	To   string
	Type string
}

type calculationNodeState struct {
	HasValue bool
	Value    float64
	Source   string
}

func (s *AssessmentSessionService) ListCalculatedObjects(
	ctx context.Context,
	claims *auth.Claims,
	sessionID uint,
	periodCode string,
	objectGroupCode string,
) ([]CalculatedAssessmentObject, error) {
	if sessionID == 0 {
		return nil, ErrInvalidParam
	}
	targetPeriod := strings.ToUpper(strings.TrimSpace(periodCode))
	targetGroup := strings.TrimSpace(objectGroupCode)
	if targetPeriod == "" || targetGroup == "" {
		return nil, ErrInvalidParam
	}

	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, summary.OrganizationID); err != nil {
		return nil, err
	}

	periods, err := s.listPeriods(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	periodSet := make(map[string]struct{}, len(periods))
	for _, item := range periods {
		periodSet[strings.ToUpper(strings.TrimSpace(item.PeriodCode))] = struct{}{}
	}
	if _, exists := periodSet[targetPeriod]; !exists {
		return nil, ErrPeriodNotFound
	}

	ruleFile, err := s.pickSessionRuleFile(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	ruleContent, err := parseCalculationRuleContent(ruleFile.ContentJSON)
	if err != nil {
		return nil, ErrInvalidExpression
	}
	targetScoped := matchScopedRule(ruleContent, targetPeriod, targetGroup)
	if targetScoped == nil {
		return nil, ErrRuleNotFound
	}

	objects, err := s.listSessionObjects(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	activeObjects := make([]model.AssessmentSessionObject, 0, len(objects))
	objectByID := make(map[uint]model.AssessmentSessionObject, len(objects))
	for _, item := range objects {
		if !item.IsActive {
			continue
		}
		activeObjects = append(activeObjects, item)
		objectByID[item.ID] = item
	}
	if len(activeObjects) == 0 {
		return []CalculatedAssessmentObject{}, nil
	}

	collector := newDependencyIssueCollector(300)
	dependencies := resolveDependencyConfigs(ruleContent.Raw, collector)
	nodes, edges := buildCalculationEdges(periods, activeObjects, dependencies, collector)
	if hasCycle(nodes, edges) {
		return nil, ErrCalcDependencyCycle
	}
	order, ok := topoSort(nodes, edges)
	if !ok {
		return nil, ErrCalcDependencyCycle
	}

	periodCodes := make([]string, 0, len(periodSet))
	for code := range periodSet {
		periodCodes = append(periodCodes, code)
	}
	sort.Strings(periodCodes)

	moduleScoreRows, err := s.listModuleScores(ctx, sessionID, periodCodes)
	if err != nil {
		return nil, err
	}
	rawScoresByNode := map[string]map[string]float64{}
	for _, row := range moduleScoreRows {
		key := buildDependencyNode(row.PeriodCode, row.ObjectID)
		if _, exists := rawScoresByNode[key]; !exists {
			rawScoresByNode[key] = map[string]float64{}
		}
		rawScoresByNode[key][strings.TrimSpace(row.ModuleKey)] = row.Score
	}

	states := make(map[string]*calculationNodeState, len(nodes))
	for _, node := range nodes {
		period, objectID, ok := parseCalculationNodeKey(node)
		if !ok {
			continue
		}
		object, exists := objectByID[objectID]
		if !exists {
			continue
		}
		scoped := matchScopedRule(ruleContent, period, object.GroupCode)
		if scoped == nil {
			states[node] = &calculationNodeState{}
			continue
		}
		scoreModules := toScoreModules(scoped.ScoreModules)
		rawModuleScores := rawScoresByNode[node]
		moduleScoreMap := map[string]float64{}
		hasBaseInput := false
		for _, module := range scoreModules {
			score, exists := rawModuleScores[module.ModuleKey]
			if !exists {
				continue
			}
			moduleScoreMap[module.ModuleKey] = score
			hasBaseInput = true
		}
		extraAdjust := 0.0
		if rawModuleScores != nil {
			if value, exists := rawModuleScores[extraAdjustModuleKey]; exists {
				extraAdjust = value
				hasBaseInput = true
			}
		}
		if hasBaseInput {
			value := CalculateTotalScore(moduleScoreMap, scoreModules, extraAdjust)
			states[node] = &calculationNodeState{
				HasValue: true,
				Value:    value,
				Source:   "base",
			}
			continue
		}
		states[node] = &calculationNodeState{}
	}

	incoming := incomingEdges(edges)
	for _, node := range order {
		state, exists := states[node]
		if !exists {
			state = &calculationNodeState{}
			states[node] = state
		}
		if state.HasValue {
			continue
		}
		periodRollupValue, hasPeriodRollup := resolvePeriodRollupValue(incoming[node], states)
		if hasPeriodRollup {
			state.HasValue = true
			state.Value = periodRollupValue
			state.Source = dependencyTypePeriodRollup
			continue
		}
		parentValue, hasParent := resolveParentValue(incoming[node], states)
		if hasParent {
			state.HasValue = true
			state.Value = parentValue
			state.Source = dependencyTypeObjectParent
		}
	}

	targetObjects := make([]model.AssessmentSessionObject, 0, len(activeObjects))
	for _, item := range activeObjects {
		if item.GroupCode == targetGroup {
			targetObjects = append(targetObjects, item)
		}
	}
	sort.SliceStable(targetObjects, func(i, j int) bool {
		left := targetObjects[i]
		right := targetObjects[j]
		if left.SortOrder != right.SortOrder {
			return left.SortOrder < right.SortOrder
		}
		return left.ID < right.ID
	})

	rows := make([]CalculatedAssessmentObject, 0, len(targetObjects))
	scoringItems := make([]RuleEngineObject, 0, len(targetObjects))
	rowIndexByObjectID := make(map[uint]int, len(targetObjects))
	for _, object := range targetObjects {
		node := buildDependencyNode(targetPeriod, object.ID)
		row := CalculatedAssessmentObject{
			AssessmentSessionObject: object,
			ModuleScores:            buildOutputModuleScores(rawScoresByNode[node], targetScoped.ScoreModules),
			Grade:                   "",
		}
		if state := states[node]; state != nil && state.HasValue {
			value := state.Value
			row.TotalScore = &value
			row.ScoreSource = state.Source
			scoringItems = append(scoringItems, RuleEngineObject{
				ObjectID:     object.ID,
				GroupKey:     targetGroup,
				TotalScore:   value,
				ModuleScores: map[string]float64{},
			})
		}
		rowIndexByObjectID[object.ID] = len(rows)
		rows = append(rows, row)
	}

	if len(scoringItems) == 0 {
		return rows, nil
	}
	graded := AssignGradesByGroup(scoringItems, toGradeRules(targetScoped.Grades), nil)
	for _, item := range graded {
		index, exists := rowIndexByObjectID[item.ObjectID]
		if !exists {
			continue
		}
		rank := item.Rank
		rows[index].Rank = &rank
		rows[index].Grade = item.Grade
	}
	return rows, nil
}

func (s *AssessmentSessionService) UpsertModuleScores(
	ctx context.Context,
	claims *auth.Claims,
	operatorID uint,
	sessionID uint,
	items []SessionObjectModuleScoreUpsertItem,
	ipAddress string,
	userAgent string,
) ([]model.AssessmentObjectModuleScore, error) {
	if sessionID == 0 || len(items) == 0 {
		return nil, ErrInvalidParam
	}
	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, summary.OrganizationID); err != nil {
		return nil, err
	}

	periods, err := s.listPeriods(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	validPeriods := make(map[string]struct{}, len(periods))
	for _, item := range periods {
		code := strings.ToUpper(strings.TrimSpace(item.PeriodCode))
		if code != "" {
			validPeriods[code] = struct{}{}
		}
	}

	objects, err := s.listSessionObjects(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	validObjectIDs := make(map[uint]struct{}, len(objects))
	for _, item := range objects {
		validObjectIDs[item.ID] = struct{}{}
	}

	operatorRef := resolveBusinessWriteOperatorRef(s.db.WithContext(ctx), operatorID)
	normalized := make([]model.AssessmentObjectModuleScore, 0, len(items))
	seen := map[string]struct{}{}
	targetPeriods := make([]string, 0, len(items))
	periodSeen := map[string]struct{}{}
	for _, item := range items {
		periodCode := strings.ToUpper(strings.TrimSpace(item.PeriodCode))
		if _, exists := validPeriods[periodCode]; !exists {
			return nil, ErrPeriodNotFound
		}
		if _, exists := validObjectIDs[item.ObjectID]; !exists {
			return nil, ErrAssessmentObjectNotFound
		}
		moduleKey := strings.TrimSpace(item.ModuleKey)
		if moduleKey == "" {
			return nil, ErrInvalidScoreModule
		}
		key := periodCode + "|" + strconv.FormatUint(uint64(item.ObjectID), 10) + "|" + moduleKey
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, model.AssessmentObjectModuleScore{
			AssessmentID: sessionID,
			PeriodCode:   periodCode,
			ObjectID:     item.ObjectID,
			ModuleKey:    moduleKey,
			Score:        item.Score,
			CreatedBy:    operatorRef,
			UpdatedBy:    operatorRef,
		})
		if _, exists := periodSeen[periodCode]; !exists {
			periodSeen[periodCode] = struct{}{}
			targetPeriods = append(targetPeriods, periodCode)
		}
	}
	if len(normalized) == 0 {
		return []model.AssessmentObjectModuleScore{}, nil
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		for _, item := range normalized {
			row := item
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "assessment_id"},
					{Name: "period_code"},
					{Name: "object_id"},
					{Name: "module_key"},
				},
				DoUpdates: clause.Assignments(map[string]any{
					"score":      row.Score,
					"updated_by": operatorRef,
					"updated_at": now,
				}),
			}).Create(&row).Error; err != nil {
				return fmt.Errorf("failed to upsert module score: %w", err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	result := make([]model.AssessmentObjectModuleScore, 0, len(normalized))
	if err := s.db.WithContext(ctx).
		Where("assessment_id = ? AND period_code IN ?", sessionID, targetPeriods).
		Order("period_code ASC, object_id ASC, module_key ASC").
		Find(&result).Error; err != nil {
		return nil, fmt.Errorf("failed to query updated module scores: %w", err)
	}

	targetID := sessionID
	_ = s.auditRepo.Create(ctx, buildAuditRecord(operatorRef, "update", "assessment_sessions", &targetID, map[string]any{
		"event":       "upsert_module_scores",
		"itemCount":   len(normalized),
		"periodCodes": targetPeriods,
	}, ipAddress, userAgent))
	return result, nil
}

func (s *AssessmentSessionService) pickSessionRuleFile(ctx context.Context, sessionID uint) (*model.RuleFile, error) {
	items := make([]model.RuleFile, 0, 8)
	if err := s.db.WithContext(ctx).
		Where("assessment_id = ?", sessionID).
		Order("updated_at DESC, id DESC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list rule files: %w", err)
	}
	if len(items) == 0 {
		return nil, ErrRuleNotFound
	}
	var picked *model.RuleFile
	for index := range items {
		if !items[index].IsCopy {
			picked = &items[index]
			break
		}
	}
	if picked == nil {
		picked = &items[0]
	}
	return picked, nil
}

func parseCalculationRuleContent(raw string) (*calculationRuleContent, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, ErrRuleNotFound
	}
	contentRaw := map[string]any{}
	if err := json.Unmarshal([]byte(text), &contentRaw); err != nil {
		return nil, err
	}
	content := &calculationRuleContent{}
	if err := json.Unmarshal([]byte(text), content); err != nil {
		return nil, err
	}
	content.Raw = contentRaw
	return content, nil
}

func matchScopedRule(content *calculationRuleContent, periodCode string, groupCode string) *calculationScoped {
	if content == nil {
		return nil
	}
	targetPeriod := strings.ToUpper(strings.TrimSpace(periodCode))
	targetGroup := strings.TrimSpace(groupCode)
	for index := range content.ScopedRules {
		item := &content.ScopedRules[index]
		if !containsPeriod(item.ApplicablePeriods, targetPeriod) {
			continue
		}
		if !containsGroupCode(item.ApplicableObjectGroup, targetGroup) {
			continue
		}
		return item
	}
	return nil
}

func containsPeriod(periods []string, target string) bool {
	if target == "" {
		return false
	}
	for _, item := range periods {
		if strings.ToUpper(strings.TrimSpace(item)) == target {
			return true
		}
	}
	return false
}

func containsGroupCode(groups []string, target string) bool {
	if target == "" {
		return false
	}
	for _, item := range groups {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func toScoreModules(modules []calculationScoreNode) []RuleEngineScoreModule {
	result := make([]RuleEngineScoreModule, 0, len(modules))
	for _, item := range modules {
		key := strings.TrimSpace(item.ModuleKey)
		if key == "" {
			key = strings.TrimSpace(item.ID)
		}
		if key == "" {
			continue
		}
		weight := item.Weight
		if weight <= 0 {
			continue
		}
		result = append(result, RuleEngineScoreModule{
			ModuleKey: key,
			Weight:    weight,
		})
	}
	return result
}

func toGradeRules(grades []calculationGradeRule) []RuleEngineGradeRule {
	result := make([]RuleEngineGradeRule, 0, len(grades))
	for _, item := range grades {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		rule := RuleEngineGradeRule{
			Title: title,
			ScoreNode: RuleEngineGradeScoreNode{
				HasUpperLimit: item.ScoreNode.HasUpperLimit,
				UpperScore:    item.ScoreNode.UpperScore,
				UpperOperator: normalizeUpperOperator(item.ScoreNode.UpperOperator),
				HasLowerLimit: item.ScoreNode.HasLowerLimit,
				LowerScore:    item.ScoreNode.LowerScore,
				LowerOperator: normalizeLowerOperator(item.ScoreNode.LowerOperator),
			},
			ConditionLogic: normalizeConditionLogic(item.ConditionLogic),
		}
		if item.MaxRatioPercent != nil {
			percent := *item.MaxRatioPercent
			if percent > 0 && percent <= 100 {
				ratio := percent / 100
				rule.MaxRatio = &ratio
			}
		}
		result = append(result, rule)
	}
	return result
}

func normalizeUpperOperator(value string) string {
	if strings.TrimSpace(value) == "<" {
		return "<"
	}
	return "<="
}

func normalizeLowerOperator(value string) string {
	if strings.TrimSpace(value) == ">" {
		return ">"
	}
	return ">="
}

func buildCalculationEdges(
	periods []model.AssessmentSessionPeriod,
	objects []model.AssessmentSessionObject,
	dependencies []dependencyConfig,
	collector *dependencyIssueCollector,
) ([]string, []calculationEdge) {
	periodCodes := make([]string, 0, len(periods))
	periodSet := map[string]struct{}{}
	for _, item := range periods {
		code := strings.ToUpper(strings.TrimSpace(item.PeriodCode))
		if code == "" {
			continue
		}
		if _, exists := periodSet[code]; exists {
			continue
		}
		periodSet[code] = struct{}{}
		periodCodes = append(periodCodes, code)
	}

	nodes := make([]string, 0, len(periodCodes)*len(objects))
	objectByID := make(map[uint]model.AssessmentSessionObject, len(objects))
	for _, object := range objects {
		objectByID[object.ID] = object
		for _, period := range periodCodes {
			nodes = append(nodes, buildDependencyNode(period, object.ID))
		}
	}
	sort.Strings(nodes)

	edgeMap := map[string]calculationEdge{}
	for _, dependency := range dependencies {
		switch dependency.Type {
		case dependencyTypeObjectParent:
			targetType := normalizeObjectTypeToken(dependency.TargetObjectType, ObjectTypeIndividual)
			sourceType := normalizeObjectTypeToken(dependency.SourceObjectType, ObjectTypeTeam)
			for _, object := range objects {
				if !strings.EqualFold(object.ObjectType, targetType) {
					continue
				}
				if object.ParentObjectID == nil || *object.ParentObjectID == 0 {
					collector.add(
						dependencySeverityWarning,
						dependencyIssueMissingParent,
						fmt.Sprintf("object %d has no parent object for object_parent dependency", object.ID),
						fmt.Sprintf("object:%d", object.ID),
					)
					continue
				}
				parent, exists := objectByID[*object.ParentObjectID]
				if !exists {
					collector.add(
						dependencySeverityWarning,
						dependencyIssueMissingParent,
						fmt.Sprintf("object %d parent %d not found", object.ID, *object.ParentObjectID),
						fmt.Sprintf("object:%d", object.ID),
						fmt.Sprintf("parent:%d", *object.ParentObjectID),
					)
					continue
				}
				if sourceType != "" && !strings.EqualFold(parent.ObjectType, sourceType) {
					collector.add(
						dependencySeverityWarning,
						dependencyIssueInvalidParentType,
						fmt.Sprintf(
							"object %d parent %d type %s does not match required source type %s",
							object.ID,
							parent.ID,
							parent.ObjectType,
							sourceType,
						),
						fmt.Sprintf("object:%d", object.ID),
						fmt.Sprintf("parent:%d", parent.ID),
					)
					continue
				}
				for _, period := range periodCodes {
					from := buildDependencyNode(period, parent.ID)
					to := buildDependencyNode(period, object.ID)
					key := from + "->" + to + "|" + dependencyTypeObjectParent
					edgeMap[key] = calculationEdge{
						From: from,
						To:   to,
						Type: dependencyTypeObjectParent,
					}
				}
			}
		case dependencyTypePeriodRollup:
			targetPeriod := strings.ToUpper(strings.TrimSpace(dependency.TargetPeriod))
			if targetPeriod == "" {
				targetPeriod = "YEAR_END"
			}
			if _, exists := periodSet[targetPeriod]; !exists {
				collector.add(
					dependencySeverityWarning,
					dependencyIssueMissingTarget,
					fmt.Sprintf("target period %s does not exist in session periods", targetPeriod),
					targetPeriod,
				)
				continue
			}
			sourcePeriods := normalizePeriodCodes(stringsToAnySlice(dependency.SourcePeriods))
			if len(sourcePeriods) == 0 {
				collector.add(
					dependencySeverityError,
					dependencyIssueInvalidRollup,
					fmt.Sprintf("period_rollup target %s has empty sourcePeriods", targetPeriod),
					targetPeriod,
				)
				continue
			}

			validSources := make([]string, 0, len(sourcePeriods))
			for _, source := range sourcePeriods {
				if source == targetPeriod {
					collector.add(
						dependencySeverityError,
						dependencyIssueInvalidRollup,
						fmt.Sprintf("period_rollup target %s cannot include itself as source", targetPeriod),
						targetPeriod,
						source,
					)
					continue
				}
				if _, exists := periodSet[source]; !exists {
					collector.add(
						dependencySeverityWarning,
						dependencyIssueMissingSource,
						fmt.Sprintf("source period %s does not exist in session periods", source),
						source,
						targetPeriod,
					)
					continue
				}
				validSources = append(validSources, source)
			}
			if len(validSources) == 0 {
				continue
			}

			for _, object := range objects {
				to := buildDependencyNode(targetPeriod, object.ID)
				for _, source := range validSources {
					from := buildDependencyNode(source, object.ID)
					key := from + "->" + to + "|" + dependencyTypePeriodRollup
					edgeMap[key] = calculationEdge{
						From: from,
						To:   to,
						Type: dependencyTypePeriodRollup,
					}
				}
			}
		default:
			collector.add(
				dependencySeverityWarning,
				dependencyIssueUnknownType,
				fmt.Sprintf("unknown dependency type: %s", dependency.Type),
			)
		}
	}

	edges := make([]calculationEdge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		edges = append(edges, edge)
	}
	sort.SliceStable(edges, func(i, j int) bool {
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].Type < edges[j].Type
	})
	return nodes, edges
}

func topoSort(nodes []string, edges []calculationEdge) ([]string, bool) {
	inDegree := make(map[string]int, len(nodes))
	outgoing := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		inDegree[node] = 0
	}
	for _, edge := range edges {
		outgoing[edge.From] = append(outgoing[edge.From], edge.To)
		inDegree[edge.To]++
	}

	queue := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}
	sort.Strings(queue)

	order := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)
		next := outgoing[node]
		sort.Strings(next)
		for _, target := range next {
			inDegree[target]--
			if inDegree[target] == 0 {
				queue = append(queue, target)
			}
		}
		sort.Strings(queue)
	}
	if len(order) != len(nodes) {
		return nil, false
	}
	return order, true
}

func incomingEdges(edges []calculationEdge) map[string][]calculationEdge {
	result := make(map[string][]calculationEdge)
	for _, edge := range edges {
		result[edge.To] = append(result[edge.To], edge)
	}
	return result
}

func resolvePeriodRollupValue(edges []calculationEdge, states map[string]*calculationNodeState) (float64, bool) {
	sum := 0.0
	count := 0
	for _, edge := range edges {
		if edge.Type != dependencyTypePeriodRollup {
			continue
		}
		source := states[edge.From]
		if source == nil || !source.HasValue {
			continue
		}
		sum += source.Value
		count++
	}
	if count == 0 {
		return 0, false
	}
	return sum / float64(count), true
}

func resolveParentValue(edges []calculationEdge, states map[string]*calculationNodeState) (float64, bool) {
	for _, edge := range edges {
		if edge.Type != dependencyTypeObjectParent {
			continue
		}
		source := states[edge.From]
		if source == nil || !source.HasValue {
			continue
		}
		return source.Value, true
	}
	return 0, false
}

func hasCycle(nodes []string, edges []calculationEdge) bool {
	graph := newDependencyGraph()
	for _, node := range nodes {
		graph.addNode(node)
	}
	for _, edge := range edges {
		graph.addEdge(edge.From, edge.To)
	}
	cycles := findDependencyCycles(graph, 1)
	return len(cycles) > 0
}

func parseCalculationNodeKey(node string) (string, uint, bool) {
	left := strings.SplitN(node, "|object:", 2)
	if len(left) != 2 {
		return "", 0, false
	}
	period := strings.ToUpper(strings.TrimSpace(left[0]))
	right := strings.SplitN(left[1], "|", 2)
	if len(right) != 2 {
		return "", 0, false
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(right[0]), 10, 64)
	if err != nil {
		return "", 0, false
	}
	return period, uint(parsed), true
}

func buildOutputModuleScores(
	rawModuleScores map[string]float64,
	modules []calculationScoreNode,
) map[string]*float64 {
	result := make(map[string]*float64, len(modules))
	for _, module := range modules {
		key := strings.TrimSpace(module.ModuleKey)
		if key == "" {
			key = strings.TrimSpace(module.ID)
		}
		if key == "" {
			continue
		}
		if rawModuleScores == nil {
			result[key] = nil
			continue
		}
		if value, exists := rawModuleScores[key]; exists {
			score := value
			result[key] = &score
			continue
		}
		result[key] = nil
	}
	return result
}

func (s *AssessmentSessionService) listSessionObjects(
	ctx context.Context,
	sessionID uint,
) ([]model.AssessmentSessionObject, error) {
	items := make([]model.AssessmentSessionObject, 0, 64)
	if err := s.db.WithContext(ctx).
		Where("assessment_id = ?", sessionID).
		Order("sort_order ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list assessment objects: %w", err)
	}
	return items, nil
}

func (s *AssessmentSessionService) listModuleScores(
	ctx context.Context,
	sessionID uint,
	periodCodes []string,
) ([]model.AssessmentObjectModuleScore, error) {
	items := make([]model.AssessmentObjectModuleScore, 0, 256)
	query := s.db.WithContext(ctx).
		Where("assessment_id = ?", sessionID)
	if len(periodCodes) > 0 {
		query = query.Where("period_code IN ?", periodCodes)
	}
	if err := query.Order("id ASC").Find(&items).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return []model.AssessmentObjectModuleScore{}, nil
		}
		return nil, fmt.Errorf("failed to list module scores: %w", err)
	}
	return items, nil
}
