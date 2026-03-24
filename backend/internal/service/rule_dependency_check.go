package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"gorm.io/gorm"
)

const (
	dependencyTypeObjectParent = "object_parent"
	dependencyTypePeriodRollup = "period_rollup"

	dependencySeverityError   = "error"
	dependencySeverityWarning = "warning"

	dependencyIssueCycle              = "DEPENDENCY_CYCLE"
	dependencyIssueInvalidContentJSON = "INVALID_RULE_CONTENT_JSON"
	dependencyIssueUnknownType        = "UNKNOWN_DEPENDENCY_TYPE"
	dependencyIssueMissingParent      = "MISSING_PARENT_OBJECT"
	dependencyIssueInvalidParentType  = "INVALID_PARENT_OBJECT_TYPE"
	dependencyIssueMissingTarget      = "MISSING_TARGET_PERIOD"
	dependencyIssueMissingSource      = "MISSING_SOURCE_PERIOD"
	dependencyIssueInvalidRollup      = "INVALID_PERIOD_ROLLUP_CONFIG"
)

type RuleDependencyCheckSummary struct {
	ErrorCount   int `json:"errorCount"`
	WarningCount int `json:"warningCount"`
	NodeCount    int `json:"nodeCount"`
	EdgeCount    int `json:"edgeCount"`
}

type RuleDependencyIssue struct {
	Severity string   `json:"severity"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Path     []string `json:"path,omitempty"`
}

type RuleDependencyCheckResult struct {
	Summary RuleDependencyCheckSummary `json:"summary"`
	Issues  []RuleDependencyIssue      `json:"issues"`
}

type dependencyConfig struct {
	Type             string
	TargetObjectType string
	SourceObjectType string
	TargetPeriod     string
	SourcePeriods    []string
}

type dependencyGraph struct {
	nodes map[string]struct{}
	edges map[string][]string
	seen  map[string]struct{}
}

func newDependencyGraph() *dependencyGraph {
	return &dependencyGraph{
		nodes: make(map[string]struct{}),
		edges: make(map[string][]string),
		seen:  make(map[string]struct{}),
	}
}

func (g *dependencyGraph) addNode(node string) {
	if node == "" {
		return
	}
	g.nodes[node] = struct{}{}
}

func (g *dependencyGraph) addEdge(from string, to string) {
	if from == "" || to == "" {
		return
	}
	g.addNode(from)
	g.addNode(to)
	key := from + "->" + to
	if _, exists := g.seen[key]; exists {
		return
	}
	g.seen[key] = struct{}{}
	g.edges[from] = append(g.edges[from], to)
}

func (g *dependencyGraph) nodeCount() int {
	return len(g.nodes)
}

func (g *dependencyGraph) edgeCount() int {
	return len(g.seen)
}

type dependencyIssueCollector struct {
	max   int
	seen  map[string]struct{}
	items []RuleDependencyIssue
}

func newDependencyIssueCollector(max int) *dependencyIssueCollector {
	if max <= 0 {
		max = 200
	}
	return &dependencyIssueCollector{
		max:  max,
		seen: make(map[string]struct{}),
	}
}

func (c *dependencyIssueCollector) add(severity string, code string, message string, path ...string) {
	if len(c.items) >= c.max {
		return
	}
	if severity != dependencySeverityError {
		severity = dependencySeverityWarning
	}
	cleanPath := make([]string, 0, len(path))
	for _, item := range path {
		text := strings.TrimSpace(item)
		if text != "" {
			cleanPath = append(cleanPath, text)
		}
	}
	key := severity + "|" + code + "|" + message + "|" + strings.Join(cleanPath, "->")
	if _, exists := c.seen[key]; exists {
		return
	}
	c.seen[key] = struct{}{}
	c.items = append(c.items, RuleDependencyIssue{
		Severity: severity,
		Code:     code,
		Message:  message,
		Path:     cleanPath,
	})
}

func (c *dependencyIssueCollector) all() []RuleDependencyIssue {
	result := make([]RuleDependencyIssue, len(c.items))
	copy(result, c.items)
	return result
}

func (s *RuleManagementService) CheckRuleDependencies(
	ctx context.Context,
	claims *auth.Claims,
	assessmentID uint,
	ruleID uint,
) (*RuleDependencyCheckResult, error) {
	if ruleID == 0 {
		return nil, ErrInvalidParam
	}

	var (
		session *AssessmentSessionSummary
		record  *model.RuleFile
		err     error
	)
	if assessmentID > 0 {
		session, record, err = s.findRuleFileInSession(ctx, assessmentID, ruleID)
	} else {
		session, record, err = s.findRuleFileAcrossSessions(ctx, ruleID)
	}
	if err != nil {
		return nil, err
	}
	if err := ensureAssessmentOrganizationScope(claims, session.OrganizationID); err != nil {
		return nil, err
	}

	collector := newDependencyIssueCollector(300)
	content := map[string]any{}
	contentText := strings.TrimSpace(record.ContentJSON)
	if contentText != "" {
		if err := json.Unmarshal([]byte(contentText), &content); err != nil {
			collector.add(
				dependencySeverityError,
				dependencyIssueInvalidContentJSON,
				"rule contentJson is invalid JSON",
			)
		}
	}

	periods, err := s.listSessionPeriods(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	objects, err := s.listSessionObjects(ctx, session.ID)
	if err != nil {
		return nil, err
	}

	dependencies := resolveDependencyConfigs(content, periods, collector)
	graph := compileDependencyGraph(periods, objects, dependencies, collector)
	cycles := findDependencyCycles(graph, 20)
	for _, cyclePath := range cycles {
		collector.add(
			dependencySeverityError,
			dependencyIssueCycle,
			"dependency cycle detected",
			cyclePath...,
		)
	}

	issues := collector.all()
	sort.SliceStable(issues, func(i, j int) bool {
		leftRank := issueSeverityRank(issues[i].Severity)
		rightRank := issueSeverityRank(issues[j].Severity)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if issues[i].Code != issues[j].Code {
			return issues[i].Code < issues[j].Code
		}
		return issues[i].Message < issues[j].Message
	})

	summary := RuleDependencyCheckSummary{
		NodeCount: graph.nodeCount(),
		EdgeCount: graph.edgeCount(),
	}
	for _, issue := range issues {
		if issue.Severity == dependencySeverityError {
			summary.ErrorCount++
		} else {
			summary.WarningCount++
		}
	}
	return &RuleDependencyCheckResult{
		Summary: summary,
		Issues:  issues,
	}, nil
}

func (s *RuleManagementService) listSessionPeriods(
	ctx context.Context,
	sessionID uint,
) ([]model.AssessmentSessionPeriod, error) {
	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	items := make([]model.AssessmentSessionPeriod, 0, 8)
	if err := withSessionBusinessDB(ctx, summary, func(sessionDB *gorm.DB) error {
		if err := sessionDB.
			Where("assessment_id = ?", sessionID).
			Order("sort_order ASC, id ASC").
			Find(&items).Error; err != nil {
			return fmt.Errorf("failed to list assessment periods: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *RuleManagementService) listSessionObjects(
	ctx context.Context,
	sessionID uint,
) ([]model.AssessmentSessionObject, error) {
	summary, err := s.loadSessionSummary(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	items := make([]model.AssessmentSessionObject, 0, 64)
	if err := withSessionBusinessDB(ctx, summary, func(sessionDB *gorm.DB) error {
		if err := sessionDB.
			Where("assessment_id = ?", sessionID).
			Order("sort_order ASC, id ASC").
			Find(&items).Error; err != nil {
			return fmt.Errorf("failed to list assessment objects: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return items, nil
}

func issueSeverityRank(severity string) int {
	if severity == dependencySeverityError {
		return 0
	}
	return 1
}

func resolveDependencyConfigs(
	content map[string]any,
	periods []model.AssessmentSessionPeriod,
	collector *dependencyIssueCollector,
) []dependencyConfig {
	periodCodes := collectOrderedPeriodCodes(periods)
	defaultTargetPeriod, defaultSourcePeriods, hasDefaultRollup := resolveDefaultPeriodRollup(periodCodes)
	result := defaultDependencyConfigs(periodCodes)
	rawItems, ok := content["dependencies"]
	if !ok {
		return result
	}
	items, ok := rawItems.([]any)
	if !ok {
		collector.add(
			dependencySeverityWarning,
			dependencyIssueUnknownType,
			"dependencies field must be an array",
		)
		return result
	}
	for index, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			collector.add(
				dependencySeverityWarning,
				dependencyIssueUnknownType,
				fmt.Sprintf("dependencies[%d] is invalid", index),
			)
			continue
		}
		enabled := true
		if rawEnabled, exists := row["enabled"]; exists {
			if boolEnabled, boolOK := rawEnabled.(bool); boolOK {
				enabled = boolEnabled
			}
		}
		if !enabled {
			continue
		}
		dependencyType := strings.ToLower(strings.TrimSpace(stringValue(row["type"])))
		switch dependencyType {
		case dependencyTypeObjectParent:
			result = append(result, dependencyConfig{
				Type:             dependencyTypeObjectParent,
				TargetObjectType: normalizeObjectTypeToken(stringValue(row["targetObjectType"]), ObjectTypeIndividual),
				SourceObjectType: normalizeObjectTypeToken(stringValue(row["sourceObjectType"]), ObjectTypeTeam),
			})
		case dependencyTypePeriodRollup:
			targetPeriod := strings.ToUpper(strings.TrimSpace(stringValue(row["targetPeriod"])))
			if targetPeriod == "" && hasDefaultRollup {
				targetPeriod = defaultTargetPeriod
			}
			sources := normalizePeriodCodes(anySlice(row["sourcePeriods"]))
			if len(sources) == 0 {
				if hasDefaultRollup {
					sources = append(sources, defaultSourcePeriods...)
				} else if targetPeriod != "" {
					sources = defaultRollupSources(periodCodes, targetPeriod)
				}
			}
			result = append(result, dependencyConfig{
				Type:          dependencyTypePeriodRollup,
				TargetPeriod:  targetPeriod,
				SourcePeriods: sources,
			})
		default:
			collector.add(
				dependencySeverityWarning,
				dependencyIssueUnknownType,
				fmt.Sprintf("dependencies[%d] has unknown type: %s", index, dependencyType),
			)
		}
	}
	return result
}

func defaultDependencyConfigs(periodCodes []string) []dependencyConfig {
	result := []dependencyConfig{
		{
			Type:             dependencyTypeObjectParent,
			TargetObjectType: ObjectTypeIndividual,
			SourceObjectType: ObjectTypeTeam,
		},
	}
	targetPeriod, sourcePeriods, ok := resolveDefaultPeriodRollup(periodCodes)
	if ok {
		result = append(result, dependencyConfig{
			Type:          dependencyTypePeriodRollup,
			TargetPeriod:  targetPeriod,
			SourcePeriods: sourcePeriods,
		})
	}
	return result
}

func compileDependencyGraph(
	periods []model.AssessmentSessionPeriod,
	objects []model.AssessmentSessionObject,
	dependencies []dependencyConfig,
	collector *dependencyIssueCollector,
) *dependencyGraph {
	graph := newDependencyGraph()
	periodCodes := make([]string, 0, len(periods))
	periodSet := make(map[string]struct{}, len(periods))
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

	activeObjects := make([]model.AssessmentSessionObject, 0, len(objects))
	objectByID := make(map[uint]model.AssessmentSessionObject, len(objects))
	for _, item := range objects {
		if !item.IsActive {
			continue
		}
		activeObjects = append(activeObjects, item)
		objectByID[item.ID] = item
	}

	for _, object := range activeObjects {
		for _, periodCode := range periodCodes {
			graph.addNode(buildDependencyNode(periodCode, object.ID))
		}
	}

	for _, dep := range dependencies {
		switch dep.Type {
		case dependencyTypeObjectParent:
			applyObjectParentDependency(graph, activeObjects, objectByID, periodCodes, dep, collector)
		case dependencyTypePeriodRollup:
			applyPeriodRollupDependency(graph, activeObjects, periodCodes, periodSet, dep, collector)
		default:
			collector.add(
				dependencySeverityWarning,
				dependencyIssueUnknownType,
				fmt.Sprintf("unknown dependency type: %s", dep.Type),
			)
		}
	}
	return graph
}

func applyObjectParentDependency(
	graph *dependencyGraph,
	objects []model.AssessmentSessionObject,
	objectByID map[uint]model.AssessmentSessionObject,
	periodCodes []string,
	dependency dependencyConfig,
	collector *dependencyIssueCollector,
) {
	targetObjectType := normalizeObjectTypeToken(dependency.TargetObjectType, ObjectTypeIndividual)
	sourceObjectType := normalizeObjectTypeToken(dependency.SourceObjectType, ObjectTypeTeam)
	for _, item := range objects {
		if !strings.EqualFold(item.ObjectType, targetObjectType) {
			continue
		}
		if item.ParentObjectID == nil || *item.ParentObjectID == 0 {
			collector.add(
				dependencySeverityWarning,
				dependencyIssueMissingParent,
				fmt.Sprintf("object %d has no parent object for object_parent dependency", item.ID),
				fmt.Sprintf("object:%d", item.ID),
			)
			continue
		}
		parent, exists := objectByID[*item.ParentObjectID]
		if !exists {
			collector.add(
				dependencySeverityWarning,
				dependencyIssueMissingParent,
				fmt.Sprintf("object %d parent %d not found", item.ID, *item.ParentObjectID),
				fmt.Sprintf("object:%d", item.ID),
				fmt.Sprintf("parent:%d", *item.ParentObjectID),
			)
			continue
		}
		if sourceObjectType != "" && !strings.EqualFold(parent.ObjectType, sourceObjectType) {
			collector.add(
				dependencySeverityWarning,
				dependencyIssueInvalidParentType,
				fmt.Sprintf(
					"object %d parent %d type %s does not match required source type %s",
					item.ID,
					parent.ID,
					parent.ObjectType,
					sourceObjectType,
				),
				fmt.Sprintf("object:%d", item.ID),
				fmt.Sprintf("parent:%d", parent.ID),
			)
			continue
		}
		for _, periodCode := range periodCodes {
			sourceNode := buildDependencyNode(periodCode, parent.ID)
			targetNode := buildDependencyNode(periodCode, item.ID)
			graph.addEdge(sourceNode, targetNode)
		}
	}
}

func applyPeriodRollupDependency(
	graph *dependencyGraph,
	objects []model.AssessmentSessionObject,
	periodCodes []string,
	periodSet map[string]struct{},
	dependency dependencyConfig,
	collector *dependencyIssueCollector,
) {
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
		return
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
		return
	}
	if _, exists := periodSet[targetPeriod]; !exists {
		collector.add(
			dependencySeverityWarning,
			dependencyIssueMissingTarget,
			fmt.Sprintf("target period %s does not exist in session periods", targetPeriod),
			targetPeriod,
		)
		return
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
		return
	}

	for _, object := range objects {
		targetNode := buildDependencyNode(targetPeriod, object.ID)
		for _, source := range validSources {
			sourceNode := buildDependencyNode(source, object.ID)
			graph.addEdge(sourceNode, targetNode)
		}
	}
}

func buildDependencyNode(periodCode string, objectID uint) string {
	return strings.ToUpper(strings.TrimSpace(periodCode)) + "|object:" + fmt.Sprintf("%d", objectID) + "|final_score"
}

func findDependencyCycles(graph *dependencyGraph, maxCycles int) [][]string {
	if graph == nil || maxCycles <= 0 {
		return nil
	}
	nodes := make([]string, 0, len(graph.nodes))
	for node := range graph.nodes {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	state := make(map[string]int, len(nodes))
	stack := make([]string, 0, len(nodes))
	stackIndex := make(map[string]int, len(nodes))

	cycles := make([][]string, 0, 4)
	seenCycle := make(map[string]struct{})
	var dfs func(node string)
	dfs = func(node string) {
		if len(cycles) >= maxCycles {
			return
		}
		state[node] = 1
		stackIndex[node] = len(stack)
		stack = append(stack, node)

		nextNodes := append([]string(nil), graph.edges[node]...)
		sort.Strings(nextNodes)
		for _, next := range nextNodes {
			if len(cycles) >= maxCycles {
				return
			}
			switch state[next] {
			case 0:
				dfs(next)
			case 1:
				index := stackIndex[next]
				cyclePath := append([]string{}, stack[index:]...)
				cyclePath = append(cyclePath, next)
				signature := strings.Join(cyclePath, "->")
				if _, exists := seenCycle[signature]; exists {
					continue
				}
				seenCycle[signature] = struct{}{}
				cycles = append(cycles, cyclePath)
			}
		}

		stack = stack[:len(stack)-1]
		delete(stackIndex, node)
		state[node] = 2
	}

	for _, node := range nodes {
		if state[node] == 0 {
			dfs(node)
		}
		if len(cycles) >= maxCycles {
			break
		}
	}
	return cycles
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func anySlice(value any) []any {
	if value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	return items
}

func stringsToAnySlice(items []string) []any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}

func normalizePeriodCodes(items []any) []string {
	result := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(value))
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

func collectOrderedPeriodCodes(periods []model.AssessmentSessionPeriod) []string {
	result := make([]string, 0, len(periods))
	seen := map[string]struct{}{}
	for _, period := range periods {
		code := strings.ToUpper(strings.TrimSpace(period.PeriodCode))
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

func resolveDefaultPeriodRollup(periodCodes []string) (string, []string, bool) {
	if len(periodCodes) < 2 {
		return "", nil, false
	}
	targetPeriod := strings.ToUpper(strings.TrimSpace(periodCodes[len(periodCodes)-1]))
	if targetPeriod == "" {
		return "", nil, false
	}
	sourcePeriods := defaultRollupSources(periodCodes, targetPeriod)
	if len(sourcePeriods) == 0 {
		return "", nil, false
	}
	return targetPeriod, sourcePeriods, true
}

func defaultRollupSources(periodCodes []string, targetPeriod string) []string {
	target := strings.ToUpper(strings.TrimSpace(targetPeriod))
	if target == "" {
		return nil
	}
	result := make([]string, 0, len(periodCodes))
	seen := map[string]struct{}{}
	for _, codeRaw := range periodCodes {
		code := strings.ToUpper(strings.TrimSpace(codeRaw))
		if code == "" || code == target {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result
}

func normalizeObjectTypeToken(value string, fallback string) string {
	text := strings.ToLower(strings.TrimSpace(value))
	switch text {
	case ObjectTypeTeam, ObjectTypeIndividual:
		return text
	default:
		return fallback
	}
}
