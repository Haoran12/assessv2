package service

import (
	"context"
	"errors"
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
	dependencies := resolveDependencyConfigs(ruleContent.Raw, periods, collector)
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
	lookup := newExpressionScoreLookup(periods, activeObjects, rawScoresByNode)

	states := make(map[string]*calculationNodeState, len(nodes))
	calculatedModuleScoresByNode := make(map[string]map[string]float64, len(nodes))
	extraAdjustByNode := make(map[string]float64, len(nodes))
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
		extraAdjust := 0.0
		if rawModuleScores != nil {
			if value, exists := rawModuleScores[extraAdjustModuleKey]; exists {
				extraAdjust = value
			}
		}
		moduleScoreMap, hasModuleInput, err := evaluateRuleModuleScores(
			period,
			object,
			scoreModules,
			rawModuleScores,
			extraAdjust,
			lookup,
		)
		if err != nil {
			return nil, mapToCalculationEvalError(err)
		}
		calculatedModuleScoresByNode[node] = moduleScoreMap
		lookup.setNodeModuleScores(period, object.ID, moduleScoreMap)
		extraAdjustByNode[node] = extraAdjust
		hasBaseInput := hasModuleInput
		if rawModuleScores != nil {
			if _, exists := rawModuleScores[extraAdjustModuleKey]; exists {
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
			lookup.setNodeTotal(period, object.ID, value)
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
			if periodCode, objectID, parseOK := parseCalculationNodeKey(node); parseOK {
				lookup.setNodeTotal(periodCode, objectID, periodRollupValue)
			}
			continue
		}
		parentValue, hasParent := resolveParentValue(incoming[node], states)
		if hasParent {
			state.HasValue = true
			state.Value = parentValue
			state.Source = dependencyTypeObjectParent
			if periodCode, objectID, parseOK := parseCalculationNodeKey(node); parseOK {
				lookup.setNodeTotal(periodCode, objectID, parentValue)
			}
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
		nodeModuleScores := cloneFloatMap(calculatedModuleScoresByNode[node])
		row := CalculatedAssessmentObject{
			AssessmentSessionObject: object,
			ModuleScores:            buildOutputModuleScores(nodeModuleScores, targetScoped.ScoreModules),
			Grade:                   "",
		}
		if state := states[node]; state != nil && state.HasValue {
			value := state.Value
			row.TotalScore = &value
			row.ScoreSource = state.Source
			scoringItems = append(scoringItems, RuleEngineObject{
				ObjectID:       object.ID,
				GroupKey:       targetGroup,
				PeriodCode:     targetPeriod,
				ObjectType:     object.ObjectType,
				TargetID:       object.TargetID,
				TargetType:     object.TargetType,
				ParentObjectID: object.ParentObjectID,
				TotalScore:     value,
				ExtraAdjust:    extraAdjustByNode[node],
				ModuleScores:   nodeModuleScores,
			})
		}
		rowIndexByObjectID[object.ID] = len(rows)
		rows = append(rows, row)
	}

	// Build rank lookup for all groups in target period so scripts can query rank(period, objectId).
	allPeriodScoringItems := make([]RuleEngineObject, 0, len(activeObjects))
	for _, object := range activeObjects {
		node := buildDependencyNode(targetPeriod, object.ID)
		state := states[node]
		if state == nil || !state.HasValue {
			continue
		}
		allPeriodScoringItems = append(allPeriodScoringItems, RuleEngineObject{
			ObjectID:       object.ID,
			GroupKey:       object.GroupCode,
			PeriodCode:     targetPeriod,
			ObjectType:     object.ObjectType,
			TargetID:       object.TargetID,
			TargetType:     object.TargetType,
			ParentObjectID: object.ParentObjectID,
			TotalScore:     state.Value,
			ExtraAdjust:    extraAdjustByNode[node],
			ModuleScores:   cloneFloatMap(calculatedModuleScoresByNode[node]),
		})
	}
	for _, item := range RankObjectsByGroup(allPeriodScoringItems) {
		lookup.setNodeRank(targetPeriod, item.ObjectID, item.Rank)
	}

	if len(scoringItems) == 0 {
		return rows, nil
	}
	var gradeEvalErr error
	graded := AssignGradesByGroup(scoringItems, toGradeRules(targetScoped.Grades), func(object RuleEngineObject, rule RuleEngineGradeRule) (bool, error) {
		passed, err := EvalBool(rule.ExtraConditionScript, buildGradeScriptEnv(object, lookup))
		if err != nil && gradeEvalErr == nil {
			gradeEvalErr = err
		}
		return passed, err
	})
	if gradeEvalErr != nil {
		return nil, mapToCalculationEvalError(gradeEvalErr)
	}
	for _, item := range graded {
		index, exists := rowIndexByObjectID[item.ObjectID]
		if !exists {
			continue
		}
		lookup.setNodeRank(targetPeriod, item.ObjectID, item.Rank)
		lookup.setNodeGrade(targetPeriod, item.ObjectID, item.Grade)
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
				defaultTarget, _, ok := resolveDefaultPeriodRollup(periodCodes)
				if ok {
					targetPeriod = defaultTarget
				}
			}
			if targetPeriod == "" {
				collector.add(
					dependencySeverityError,
					dependencyIssueInvalidRollup,
					"period_rollup targetPeriod is empty and cannot be derived from session periods",
				)
				continue
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
				sourcePeriods = defaultRollupSources(periodCodes, targetPeriod)
			}
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

func evaluateRuleModuleScores(
	periodCode string,
	object model.AssessmentSessionObject,
	scoreModules []RuleEngineScoreModule,
	rawModuleScores map[string]float64,
	extraAdjust float64,
	lookup *expressionScoreLookup,
) (map[string]float64, bool, error) {
	calculated := make(map[string]float64, len(scoreModules))
	hasInput := false
	for _, module := range scoreModules {
		moduleKey := strings.TrimSpace(module.ModuleKey)
		if moduleKey == "" {
			continue
		}
		if normalizeCalculationMethod(module.CalculationMethod) != "custom_script" {
			if rawModuleScores == nil {
				continue
			}
			value, exists := rawModuleScores[moduleKey]
			if !exists {
				continue
			}
			calculated[moduleKey] = value
			hasInput = true
			continue
		}
		script := strings.TrimSpace(module.CustomScript)
		if script == "" {
			calculated[moduleKey] = 0
			hasInput = true
			continue
		}
		score, err := EvalNumber(
			script,
			buildModuleScriptEnv(periodCode, object, calculated, rawModuleScores, extraAdjust, lookup),
		)
		if err != nil {
			calculated[moduleKey] = 0
			hasInput = true
			continue
		}
		calculated[moduleKey] = score
		hasInput = true
	}
	return calculated, hasInput, nil
}

func buildModuleScriptEnv(
	periodCode string,
	object model.AssessmentSessionObject,
	moduleScores map[string]float64,
	rawModuleScores map[string]float64,
	extraAdjust float64,
	lookup *expressionScoreLookup,
) map[string]any {
	env := map[string]any{
		"periodCode":      strings.ToUpper(strings.TrimSpace(periodCode)),
		"objectId":        object.ID,
		"groupCode":       object.GroupCode,
		"objectType":      object.ObjectType,
		"targetId":        object.TargetID,
		"targetType":      object.TargetType,
		"parentObjectId":  parentObjectIDValue(object.ParentObjectID),
		"extraAdjust":     extraAdjust,
		"moduleScores":    cloneFloatMap(moduleScores),
		"rawModuleScores": cloneFloatMap(rawModuleScores),
	}
	for key, value := range rawModuleScores {
		env[key] = value
	}
	for key, value := range moduleScores {
		env[key] = value
	}
	for key, value := range lookup.expressionFunctions() {
		env[key] = value
	}
	return env
}

func buildGradeScriptEnv(item RuleEngineObject, lookup *expressionScoreLookup) map[string]any {
	env := map[string]any{
		"objectId":       item.ObjectID,
		"groupKey":       item.GroupKey,
		"periodCode":     item.PeriodCode,
		"objectType":     item.ObjectType,
		"targetId":       item.TargetID,
		"targetType":     item.TargetType,
		"parentObjectId": parentObjectIDValue(item.ParentObjectID),
		"totalScore":     item.TotalScore,
		"rank":           item.Rank,
		"extraAdjust":    item.ExtraAdjust,
		"moduleScores":   cloneFloatMap(item.ModuleScores),
	}
	for key, value := range item.ModuleScores {
		env[key] = value
	}
	for key, value := range lookup.expressionFunctions() {
		env[key] = value
	}
	return env
}

func mapToCalculationEvalError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrCalcExpressionEval) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrCalcExpressionEval, err)
}

func cloneFloatMap(source map[string]float64) map[string]float64 {
	if len(source) == 0 {
		return map[string]float64{}
	}
	result := make(map[string]float64, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func parentObjectIDValue(parentObjectID *uint) uint {
	if parentObjectID == nil {
		return 0
	}
	return *parentObjectID
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
