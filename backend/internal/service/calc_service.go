package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"assessv2/backend/internal/auth"
	"assessv2/backend/internal/model"
	"assessv2/backend/internal/repository"
	"github.com/Knetic/govaluate"
	"gorm.io/gorm"
)

const (
	calcTriggerAuto   = "auto"
	calcTriggerManual = "manual"

	rankingScopeOverall = "overall"
	rankingScopeParent  = "parent_object"

	ruleMatchModeBinding = "binding_owner_segment"
	ruleMatchModeSegment = "segment"
	ruleMatchModeLegacy  = "legacy_object_category"

	voteGradeScoresSettingKey = "vote.grade_scores"
)

var (
	defaultVoteGradeScores = map[string]float64{
		"excellent": 100,
		"good":      85,
		"average":   70,
		"poor":      60,
	}
)

type CalculationService struct {
	db        *gorm.DB
	auditRepo *repository.AuditRepository
}

type RecalculateInput struct {
	YearID         uint
	PeriodCode     string
	ObjectIDs      []uint
	ObjectType     string
	ObjectCategory string
	TargetType     string
	TargetID       *uint
	TriggerMode    string
}

type RecalculateResult struct {
	YearID            uint   `json:"yearId"`
	PeriodCode        string `json:"periodCode"`
	TriggerMode       string `json:"triggerMode"`
	TotalObjects      int    `json:"totalObjects"`
	CalculatedObjects int    `json:"calculatedObjects"`
	SkippedObjects    int    `json:"skippedObjects"`
	DurationMs        int64  `json:"durationMs"`
}

type ListCalculatedScoreFilter struct {
	YearID         *uint
	PeriodCode     string
	ObjectID       *uint
	ObjectType     string
	ObjectCategory string
}

type CalculatedScoreListItem struct {
	model.CalculatedScore
	ObjectName     string `json:"objectName"`
	ObjectType     string `json:"objectType"`
	ObjectCategory string `json:"objectCategory"`
	ParentObjectID *uint  `json:"parentObjectId,omitempty"`
	OverallRank    *int   `json:"overallRank,omitempty"`
}

type ListRankingFilter struct {
	YearID         *uint
	PeriodCode     string
	RankingScope   string
	ScopeKey       string
	ObjectType     string
	ObjectCategory string
}

type RankingListItem struct {
	model.Ranking
	ObjectName string `json:"objectName"`
}

type calcObjectNode struct {
	Object           model.AssessmentObject
	Rule             model.AssessmentRule
	SegmentCode      string
	RuleMatchMode    string
	OwnerOrgID       *uint
	OwnerOrgType     string
	BelongOrgID      *uint
	MatchedBindingID *uint
}

type objectCalcResult struct {
	ObjectID      uint
	RuleID        uint
	WeightedScore float64
	ExtraPoints   float64
	FinalScore    float64
	RankBasis     string
	DetailJSON    string
	Modules       []model.CalculatedModuleScore
}

type voteAggRow struct {
	ModuleID    uint
	ObjectID    uint
	VoteGroupID uint
	GradeOption string
	Count       int
}

type modulePriorityValue struct {
	SortOrder int
	RawScore  float64
}

type rankingContextRow struct {
	CalculatedScoreID uint
	ObjectID          uint
	ObjectType        string
	ObjectCategory    string
	ObjectName        string
	ParentObjectID    *uint
	Score             float64
}

func NewCalculationService(db *gorm.DB, auditRepo *repository.AuditRepository) *CalculationService {
	return &CalculationService{db: db, auditRepo: auditRepo}
}

func (s *CalculationService) Recalculate(
	ctx context.Context,
	operatorID *uint,
	input RecalculateInput,
	ipAddress string,
	userAgent string,
) (*RecalculateResult, error) {
	startedAt := time.Now()
	periodCode := normalizePeriodCode(input.PeriodCode)
	if input.YearID == 0 || !isValidPeriodCode(periodCode) {
		return nil, ErrInvalidParam
	}
	triggerMode := strings.ToLower(strings.TrimSpace(input.TriggerMode))
	if triggerMode == "" {
		triggerMode = calcTriggerManual
	}
	if triggerMode != calcTriggerAuto && triggerMode != calcTriggerManual {
		return nil, ErrInvalidParam
	}

	var result RecalculateResult
	result.YearID = input.YearID
	result.PeriodCode = periodCode
	result.TriggerMode = triggerMode

	auditOperatorRef := operatorID
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if operatorID != nil {
			auditOperatorRef = resolveBusinessWriteOperatorRefTx(tx, *operatorID)
		}
		if err := ensurePeriodWritableTx(tx, input.YearID, periodCode); err != nil {
			return err
		}

		objects, err := s.loadTargetObjectsTx(
			tx,
			input.YearID,
			input.ObjectIDs,
			strings.TrimSpace(input.ObjectType),
			strings.TrimSpace(input.ObjectCategory),
			strings.TrimSpace(input.TargetType),
			input.TargetID,
		)
		if err != nil {
			return err
		}
		if len(objects) == 0 {
			return nil
		}
		result.TotalObjects = len(objects)

		nodes, noRuleObjectIDs, err := s.buildCalcNodesTx(tx, input.YearID, periodCode, objects)
		if err != nil {
			return err
		}

		if len(noRuleObjectIDs) > 0 {
			if err := tx.Where("year_id = ? AND period_code = ? AND object_id IN ?", input.YearID, periodCode, noRuleObjectIDs).
				Delete(&model.CalculatedScore{}).Error; err != nil {
				return fmt.Errorf("failed to clear stale calculated scores: %w", err)
			}
		}
		if len(nodes) == 0 {
			return s.refreshRankingsTx(tx, input.YearID, periodCode, time.Now().Unix())
		}

		order, err := s.topoSortObjects(nodes)
		if err != nil {
			return err
		}
		objectIDs := make([]uint, 0, len(nodes))
		ruleIDs := make([]uint, 0, len(nodes))
		for objectID, node := range nodes {
			objectIDs = append(objectIDs, objectID)
			ruleIDs = append(ruleIDs, node.Rule.ID)
		}

		modulesByRule, voteGroupsByModule, err := s.loadRuleModulesTx(tx, ruleIDs)
		if err != nil {
			return err
		}
		directScoreMap, err := s.loadDirectScoreMapTx(tx, input.YearID, periodCode, objectIDs)
		if err != nil {
			return err
		}
		extraPointMap, err := s.loadExtraPointMapTx(tx, input.YearID, periodCode, objectIDs)
		if err != nil {
			return err
		}
		voteAggMap, err := s.loadVoteAggMapTx(tx, input.YearID, periodCode, objectIDs)
		if err != nil {
			return err
		}
		voteGradeScores, err := s.loadVoteGradeScoresTx(tx)
		if err != nil {
			return err
		}
		parentScoreMap, parentRankMap, err := s.loadExistingParentContextTx(tx, input.YearID, periodCode, objectIDs)
		if err != nil {
			return err
		}
		quarterScoreMap, err := s.loadQuarterScoreContextTx(tx, input.YearID, objectIDs)
		if err != nil {
			return err
		}

		now := time.Now().Unix()
		for _, objectID := range order {
			node := nodes[objectID]
			modules := modulesByRule[node.Rule.ID]
			if len(modules) == 0 {
				result.SkippedObjects++
				continue
			}

			calcResult, err := s.calculateObjectScore(
				node,
				modules,
				voteGroupsByModule,
				directScoreMap[objectID],
				extraPointMap[objectID],
				voteAggMap[objectID],
				voteGradeScores,
				parentScoreMap,
				parentRankMap,
				quarterScoreMap[objectID],
			)
			if err != nil {
				return err
			}
			if err := s.persistObjectCalculationTx(tx, auditOperatorRef, triggerMode, now, periodCode, calcResult); err != nil {
				return err
			}
			parentScoreMap[objectID] = calcResult.FinalScore
			result.CalculatedObjects++
		}

		if err := s.refreshRankingsTx(tx, input.YearID, periodCode, now); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result.DurationMs = time.Since(startedAt).Milliseconds()
	if auditOperatorRef != nil {
		_ = s.auditRepo.Create(ctx, buildAuditRecord(auditOperatorRef, "update", "calculated_scores", nil, map[string]any{
			"event":             "recalculate_scores",
			"yearId":            result.YearID,
			"periodCode":        result.PeriodCode,
			"triggerMode":       result.TriggerMode,
			"totalObjects":      result.TotalObjects,
			"calculatedObjects": result.CalculatedObjects,
			"skippedObjects":    result.SkippedObjects,
			"durationMs":        result.DurationMs,
		}, ipAddress, userAgent))
	}
	return &result, nil
}

func (s *CalculationService) ListCalculatedScores(ctx context.Context, claims *auth.Claims, filter ListCalculatedScoreFilter) ([]CalculatedScoreListItem, error) {
	scope, err := buildAssessmentAccessScope(ctx, s.db, claims)
	if err != nil {
		return nil, err
	}

	type row struct {
		model.CalculatedScore
		ObjectName     string
		ObjectType     string
		ObjectCategory string
		ParentObjectID *uint
		OverallRank    *int
	}

	query := s.db.WithContext(ctx).Table("calculated_scores cs").
		Select("cs.*, ao.object_name, ao.object_type, ao.object_category, ao.parent_object_id, r.rank_no AS overall_rank").
		Joins("JOIN assessment_objects ao ON ao.id = cs.object_id").
		Joins("LEFT JOIN rankings r ON r.calculated_score_id = cs.id AND r.ranking_scope = ?", rankingScopeOverall)
	if filter.YearID != nil {
		query = query.Where("cs.year_id = ?", *filter.YearID)
	}
	if periodCode := normalizePeriodCode(filter.PeriodCode); periodCode != "" {
		query = query.Where("cs.period_code = ?", periodCode)
	}
	if filter.ObjectID != nil {
		query = query.Where("cs.object_id = ?", *filter.ObjectID)
	}
	if objectType := strings.TrimSpace(filter.ObjectType); objectType != "" {
		query = query.Where("ao.object_type = ?", objectType)
	}
	if objectCategory := normalizeObjectCategory(strings.TrimSpace(filter.ObjectCategory)); objectCategory != "" {
		query = query.Where("ao.object_category = ?", objectCategory)
	}
	query = scope.applyReadableObjectFilter(query, "cs.object_id")

	var rows []row
	if err := query.Order("r.rank_no ASC, cs.final_score DESC, cs.id ASC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list calculated scores: %w", err)
	}

	items := make([]CalculatedScoreListItem, 0, len(rows))
	for _, item := range rows {
		items = append(items, CalculatedScoreListItem{
			CalculatedScore: item.CalculatedScore,
			ObjectName:      item.ObjectName,
			ObjectType:      item.ObjectType,
			ObjectCategory:  item.ObjectCategory,
			ParentObjectID:  item.ParentObjectID,
			OverallRank:     item.OverallRank,
		})
	}
	return items, nil
}

func (s *CalculationService) ListCalculatedModuleScores(ctx context.Context, claims *auth.Claims, calculatedScoreID uint) ([]model.CalculatedModuleScore, error) {
	if calculatedScoreID == 0 {
		return nil, ErrInvalidParam
	}

	scope, err := buildAssessmentAccessScope(ctx, s.db, claims)
	if err != nil {
		return nil, err
	}

	type scoreOwnership struct {
		ID       uint
		ObjectID uint
	}
	var ownership scoreOwnership
	if err := s.db.WithContext(ctx).Table("calculated_scores").
		Select("id, object_id").
		Where("id = ?", calculatedScoreID).
		First(&ownership).Error; err != nil {
		if repository.IsRecordNotFound(err) {
			return nil, ErrAssessmentObjectNotFound
		}
		return nil, fmt.Errorf("failed to query calculated score ownership: %w", err)
	}
	if !scope.allowsDetailObject(ownership.ObjectID) {
		return nil, ErrForbidden
	}

	var items []model.CalculatedModuleScore
	if err := s.db.WithContext(ctx).
		Where("calculated_score_id = ?", calculatedScoreID).
		Order("sort_order ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list calculated module scores: %w", err)
	}
	return items, nil
}

func (s *CalculationService) ListRankings(ctx context.Context, claims *auth.Claims, filter ListRankingFilter) ([]RankingListItem, error) {
	scope, err := buildAssessmentAccessScope(ctx, s.db, claims)
	if err != nil {
		return nil, err
	}

	type row struct {
		model.Ranking
		ObjectName string
	}

	query := s.db.WithContext(ctx).Table("rankings r").
		Select("r.*, ao.object_name").
		Joins("JOIN assessment_objects ao ON ao.id = r.object_id")
	if filter.YearID != nil {
		query = query.Where("r.year_id = ?", *filter.YearID)
	}
	if periodCode := normalizePeriodCode(filter.PeriodCode); periodCode != "" {
		query = query.Where("r.period_code = ?", periodCode)
	}
	if scope := strings.TrimSpace(filter.RankingScope); scope != "" {
		query = query.Where("r.ranking_scope = ?", scope)
	}
	if scopeKey := strings.TrimSpace(filter.ScopeKey); scopeKey != "" {
		query = query.Where("r.scope_key = ?", scopeKey)
	}
	if objectType := strings.TrimSpace(filter.ObjectType); objectType != "" {
		query = query.Where("r.object_type = ?", objectType)
	}
	if objectCategory := normalizeObjectCategory(strings.TrimSpace(filter.ObjectCategory)); objectCategory != "" {
		query = query.Where("r.object_category = ?", objectCategory)
	}
	query = scope.applyReadableObjectFilter(query, "r.object_id")

	var rows []row
	if err := query.Order("r.rank_no ASC, r.id ASC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list rankings: %w", err)
	}

	items := make([]RankingListItem, 0, len(rows))
	for _, item := range rows {
		items = append(items, RankingListItem{
			Ranking:    item.Ranking,
			ObjectName: item.ObjectName,
		})
	}
	return items, nil
}

func (s *CalculationService) loadTargetObjectsTx(
	tx *gorm.DB,
	yearID uint,
	objectIDs []uint,
	objectType string,
	objectCategory string,
	targetType string,
	targetID *uint,
) ([]model.AssessmentObject, error) {
	query := tx.Model(&model.AssessmentObject{}).Where("year_id = ? AND is_active = 1", yearID)
	segmentFilter := ""

	dedup := make([]uint, 0, len(objectIDs))
	seen := make(map[uint]struct{}, len(objectIDs))
	for _, id := range objectIDs {
		if id == 0 {
			return nil, ErrInvalidParam
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		dedup = append(dedup, id)
	}
	if len(dedup) > 0 {
		query = query.Where("id IN ?", dedup)
	}
	if text := strings.TrimSpace(objectType); text != "" {
		query = query.Where("object_type = ?", text)
	}
	if text := normalizeObjectCategory(strings.TrimSpace(objectCategory)); text != "" {
		if segmentCode := normalizeSegmentCode(text); segmentCode != "" {
			segmentFilter = segmentCode
		} else {
			query = query.Where("object_category = ?", text)
		}
	}
	if targetID != nil {
		if strings.TrimSpace(targetType) == "" || *targetID == 0 {
			return nil, ErrInvalidParam
		}
		query = query.Where("target_type = ? AND target_id = ?", strings.TrimSpace(targetType), *targetID)
	}

	var objects []model.AssessmentObject
	if err := query.Order("id ASC").Find(&objects).Error; err != nil {
		return nil, fmt.Errorf("failed to query target assessment objects: %w", err)
	}
	if segmentFilter != "" && len(objects) > 0 {
		segmentMap, err := buildObjectSegmentMapTx(tx, objects)
		if err != nil {
			return nil, err
		}
		filtered := make([]model.AssessmentObject, 0, len(objects))
		for _, object := range objects {
			if segmentMap[object.ID] != segmentFilter {
				continue
			}
			filtered = append(filtered, object)
		}
		objects = filtered
	}
	if len(dedup) > 0 && len(objects) != len(dedup) {
		return nil, ErrAssessmentObjectNotFound
	}
	return objects, nil
}

func (s *CalculationService) buildCalcNodesTx(
	tx *gorm.DB,
	yearID uint,
	periodCode string,
	objects []model.AssessmentObject,
) (map[uint]*calcObjectNode, []uint, error) {
	var rules []model.AssessmentRule
	if err := tx.Where("year_id = ? AND period_code = ? AND is_active = 1", yearID, periodCode).Find(&rules).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to query active rules: %w", err)
	}
	ruleByID := make(map[uint]model.AssessmentRule, len(rules))
	legacyRuleMap := make(map[string]model.AssessmentRule, len(rules))
	segmentRuleMap := make(map[string]model.AssessmentRule, len(rules))
	for _, item := range rules {
		ruleByID[item.ID] = item
		category := normalizeObjectCategory(item.ObjectCategory)
		key := item.ObjectType + "|" + category
		if segmentCode := normalizeSegmentCode(category); segmentCode != "" {
			segmentRuleMap[key] = item
			continue
		}
		legacyRuleMap[key] = item
	}

	segmentMap, err := buildObjectSegmentMapTx(tx, objects)
	if err != nil {
		return nil, nil, err
	}
	ownerContextMap, err := buildObjectOwnerContextMapTx(tx, objects, segmentMap)
	if err != nil {
		return nil, nil, err
	}

	var bindings []model.RuleBinding
	if err := tx.Where("year_id = ? AND period_code = ? AND is_active = 1", yearID, periodCode).
		Order("priority DESC, id ASC").
		Find(&bindings).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to query active rule bindings: %w", err)
	}

	nodes := make(map[uint]*calcObjectNode, len(objects))
	noRuleObjectIDs := make([]uint, 0)
	for _, item := range objects {
		ownerContext := ownerContextMap[item.ID]
		segmentCode := segmentMap[item.ID]
		ruleMatchMode := ""
		var matchedBindingID *uint
		rule := model.AssessmentRule{}
		matched := false

		if segmentCode != "" {
			if binding := selectRuleBindingForObject(bindings, item.ObjectType, segmentCode, ownerContext); binding != nil {
				if matchedRule, ok := ruleByID[binding.RuleID]; ok {
					rule = matchedRule
					ruleMatchMode = ruleMatchModeBinding
					matchedBindingID = uintPtr(binding.ID)
					matched = true
				}
			}
		}
		if !matched && segmentCode != "" {
			if matchedRule, ok := segmentRuleMap[item.ObjectType+"|"+segmentCode]; ok {
				rule = matchedRule
				ruleMatchMode = ruleMatchModeSegment
				matched = true
			}
		}
		if !matched {
			if matchedRule, ok := legacyRuleMap[item.ObjectType+"|"+normalizeObjectCategory(item.ObjectCategory)]; ok {
				rule = matchedRule
				ruleMatchMode = ruleMatchModeLegacy
				matched = true
			}
		}
		if !matched {
			noRuleObjectIDs = append(noRuleObjectIDs, item.ID)
			continue
		}
		nodes[item.ID] = &calcObjectNode{
			Object:           item,
			Rule:             rule,
			SegmentCode:      segmentCode,
			RuleMatchMode:    ruleMatchMode,
			OwnerOrgID:       ownerContext.OwnerOrgID,
			OwnerOrgType:     ownerContext.OwnerOrgType,
			BelongOrgID:      ownerContext.BelongOrgID,
			MatchedBindingID: matchedBindingID,
		}
	}
	return nodes, noRuleObjectIDs, nil
}

func (s *CalculationService) topoSortObjects(nodes map[uint]*calcObjectNode) ([]uint, error) {
	if len(nodes) == 0 {
		return []uint{}, nil
	}
	keys := make([]uint, 0, len(nodes))
	for objectID := range nodes {
		keys = append(keys, objectID)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	requiresTeam := make(map[uint]bool, len(nodes))
	for _, objectID := range keys {
		node := nodes[objectID]
		if node.Object.ParentObjectID == nil {
			continue
		}
		if node.Object.ObjectType != "individual" {
			continue
		}
		requiresTeam[objectID] = true
	}

	indegree := make(map[uint]int, len(nodes))
	edges := make(map[uint][]uint, len(nodes))
	for _, objectID := range keys {
		indegree[objectID] = 0
	}
	for _, objectID := range keys {
		node := nodes[objectID]
		if !requiresTeam[objectID] || node.Object.ParentObjectID == nil {
			continue
		}
		parentID := *node.Object.ParentObjectID
		if _, ok := nodes[parentID]; !ok {
			continue
		}
		edges[parentID] = append(edges[parentID], objectID)
		indegree[objectID]++
	}
	for key := range edges {
		sort.Slice(edges[key], func(i, j int) bool { return edges[key][i] < edges[key][j] })
	}

	queue := make([]uint, 0, len(nodes))
	for _, objectID := range keys {
		if indegree[objectID] == 0 {
			queue = append(queue, objectID)
		}
	}
	result := make([]uint, 0, len(nodes))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		for _, next := range edges[current] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
		sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })
	}
	if len(result) != len(nodes) {
		return nil, ErrCalcDependencyCycle
	}
	return result, nil
}

func (s *CalculationService) loadRuleModulesTx(
	tx *gorm.DB,
	ruleIDs []uint,
) (map[uint][]model.ScoreModule, map[uint][]model.VoteGroup, error) {
	if len(ruleIDs) == 0 {
		return map[uint][]model.ScoreModule{}, map[uint][]model.VoteGroup{}, nil
	}
	var modules []model.ScoreModule
	if err := tx.Where("rule_id IN ? AND is_active = 1", ruleIDs).
		Order("rule_id ASC, sort_order ASC, id ASC").
		Find(&modules).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to query score modules: %w", err)
	}
	modulesByRule := make(map[uint][]model.ScoreModule, len(ruleIDs))
	moduleIDs := make([]uint, 0, len(modules))
	for _, item := range modules {
		modulesByRule[item.RuleID] = append(modulesByRule[item.RuleID], item)
		moduleIDs = append(moduleIDs, item.ID)
	}

	voteGroupsByModule := make(map[uint][]model.VoteGroup)
	if len(moduleIDs) == 0 {
		return modulesByRule, voteGroupsByModule, nil
	}
	var groups []model.VoteGroup
	if err := tx.Where("module_id IN ? AND is_active = 1", moduleIDs).
		Order("module_id ASC, sort_order ASC, id ASC").
		Find(&groups).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to query vote groups: %w", err)
	}
	for _, item := range groups {
		voteGroupsByModule[item.ModuleID] = append(voteGroupsByModule[item.ModuleID], item)
	}
	return modulesByRule, voteGroupsByModule, nil
}

func (s *CalculationService) loadDirectScoreMapTx(tx *gorm.DB, yearID uint, periodCode string, objectIDs []uint) (map[uint]map[uint]float64, error) {
	result := make(map[uint]map[uint]float64, len(objectIDs))
	if len(objectIDs) == 0 {
		return result, nil
	}
	var rows []model.DirectScore
	if err := tx.Where("year_id = ? AND period_code = ? AND object_id IN ?", yearID, periodCode, objectIDs).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query direct scores for calculation: %w", err)
	}
	for _, row := range rows {
		moduleMap, ok := result[row.ObjectID]
		if !ok {
			moduleMap = map[uint]float64{}
			result[row.ObjectID] = moduleMap
		}
		moduleMap[row.ModuleID] = row.Score
	}
	return result, nil
}

func (s *CalculationService) loadExtraPointMapTx(tx *gorm.DB, yearID uint, periodCode string, objectIDs []uint) (map[uint]float64, error) {
	result := make(map[uint]float64, len(objectIDs))
	if len(objectIDs) == 0 {
		return result, nil
	}
	var rows []model.ExtraPoint
	if err := tx.Where("year_id = ? AND period_code = ? AND object_id IN ?", yearID, periodCode, objectIDs).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query extra points for calculation: %w", err)
	}
	for _, row := range rows {
		value := signedExtraPoints(row.PointType, row.Points)
		result[row.ObjectID] = roundToScale(result[row.ObjectID]+value, 6)
	}
	return result, nil
}

func (s *CalculationService) loadVoteAggMapTx(tx *gorm.DB, yearID uint, periodCode string, objectIDs []uint) (map[uint]map[uint][]voteAggRow, error) {
	result := make(map[uint]map[uint][]voteAggRow, len(objectIDs))
	if len(objectIDs) == 0 {
		return result, nil
	}
	var rows []voteAggRow
	err := tx.Table("vote_tasks vt").
		Select("vg.module_id AS module_id, vt.object_id, vt.vote_group_id, vr.grade_option, COUNT(1) AS count").
		Joins("JOIN vote_groups vg ON vg.id = vt.vote_group_id").
		Joins("JOIN vote_records vr ON vr.task_id = vt.id").
		Where("vt.year_id = ? AND vt.period_code = ? AND vt.status = 'completed' AND vt.object_id IN ?", yearID, periodCode, objectIDs).
		Group("vg.module_id, vt.object_id, vt.vote_group_id, vr.grade_option").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query vote aggregates for calculation: %w", err)
	}
	for _, row := range rows {
		moduleMap, ok := result[row.ObjectID]
		if !ok {
			moduleMap = map[uint][]voteAggRow{}
			result[row.ObjectID] = moduleMap
		}
		moduleMap[row.ModuleID] = append(moduleMap[row.ModuleID], row)
	}
	return result, nil
}

func (s *CalculationService) loadVoteGradeScoresTx(tx *gorm.DB) (map[string]float64, error) {
	defaults := cloneVoteGradeScores(defaultVoteGradeScores)

	var setting model.SystemSetting
	err := tx.Select("setting_value").
		Where("setting_key = ?", voteGradeScoresSettingKey).
		First(&setting).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return defaults, nil
	case err != nil:
		return nil, fmt.Errorf("failed to load vote grade scores: %w", err)
	}

	scores, parseErr := parseVoteGradeScoresSetting(setting.SettingValue)
	if parseErr != nil {
		return defaults, nil
	}
	return scores, nil
}

func (s *CalculationService) loadExistingParentContextTx(
	tx *gorm.DB,
	yearID uint,
	periodCode string,
	objectIDs []uint,
) (map[uint]float64, map[uint]int, error) {
	parentScoreMap := make(map[uint]float64)
	parentRankMap := make(map[uint]int)
	if len(objectIDs) == 0 {
		return parentScoreMap, parentRankMap, nil
	}

	var objects []model.AssessmentObject
	if err := tx.Where("id IN ?", objectIDs).Find(&objects).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to query parent context objects: %w", err)
	}
	parentSet := map[uint]struct{}{}
	for _, item := range objects {
		if item.ParentObjectID != nil {
			parentSet[*item.ParentObjectID] = struct{}{}
		}
	}
	if len(parentSet) == 0 {
		return parentScoreMap, parentRankMap, nil
	}
	parentIDs := make([]uint, 0, len(parentSet))
	for parentID := range parentSet {
		parentIDs = append(parentIDs, parentID)
	}

	var scoreRows []model.CalculatedScore
	if err := tx.Where("year_id = ? AND period_code = ? AND object_id IN ?", yearID, periodCode, parentIDs).Find(&scoreRows).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to query parent score context: %w", err)
	}
	for _, item := range scoreRows {
		parentScoreMap[item.ObjectID] = item.FinalScore
	}

	var rankRows []model.Ranking
	if err := tx.Where("year_id = ? AND period_code = ? AND object_id IN ? AND ranking_scope = ?", yearID, periodCode, parentIDs, rankingScopeOverall).
		Find(&rankRows).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to query parent rank context: %w", err)
	}
	for _, item := range rankRows {
		parentRankMap[item.ObjectID] = item.RankNo
	}
	return parentScoreMap, parentRankMap, nil
}

func (s *CalculationService) loadQuarterScoreContextTx(tx *gorm.DB, yearID uint, objectIDs []uint) (map[uint]map[string]float64, error) {
	result := make(map[uint]map[string]float64, len(objectIDs))
	if len(objectIDs) == 0 {
		return result, nil
	}
	var rows []model.CalculatedScore
	if err := tx.Where("year_id = ? AND period_code IN ? AND object_id IN ?", yearID, []string{"Q1", "Q2", "Q3", "Q4"}, objectIDs).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query quarter score context: %w", err)
	}
	for _, row := range rows {
		periodMap, ok := result[row.ObjectID]
		if !ok {
			periodMap = map[string]float64{}
			result[row.ObjectID] = periodMap
		}
		periodMap[row.PeriodCode] = row.FinalScore
	}
	return result, nil
}

func (s *CalculationService) calculateObjectScore(
	node *calcObjectNode,
	modules []model.ScoreModule,
	voteGroupsByModule map[uint][]model.VoteGroup,
	directMap map[uint]float64,
	extraPoints float64,
	voteAggByModule map[uint][]voteAggRow,
	voteGradeScores map[string]float64,
	parentScoreMap map[uint]float64,
	parentRankMap map[uint]int,
	quarterScoreMap map[string]float64,
) (*objectCalcResult, error) {
	if directMap == nil {
		directMap = map[uint]float64{}
	}
	if voteAggByModule == nil {
		voteAggByModule = map[uint][]voteAggRow{}
	}
	if voteGradeScores == nil {
		voteGradeScores = cloneVoteGradeScores(defaultVoteGradeScores)
	}
	if quarterScoreMap == nil {
		quarterScoreMap = map[string]float64{}
	}

	moduleOrder, err := resolveModuleCalcOrder(modules)
	if err != nil {
		return nil, err
	}
	moduleRaw := make(map[string]float64, len(modules))
	moduleDetails := make([]model.CalculatedModuleScore, 0, len(modules))
	weightedTotal := 0.0
	parentScore := 0.0
	parentRank := 0
	if node.Object.ParentObjectID != nil {
		parentScore = parentScoreMap[*node.Object.ParentObjectID]
		parentRank = parentRankMap[*node.Object.ParentObjectID]
	}

	for _, module := range moduleOrder {
		rawScore := 0.0
		scoreDetail := map[string]any{
			"moduleCode": module.ModuleCode,
			"moduleKey":  module.ModuleKey,
		}

		switch module.ModuleCode {
		case "direct":
			rawScore = roundToScale(directMap[module.ID], 6)
			scoreDetail["source"] = "direct_scores"
		case "vote":
			rawScore = roundToScale(calculateVoteModuleRawScore(module, voteGroupsByModule[module.ID], voteAggByModule[module.ID], voteGradeScores), 6)
			scoreDetail["source"] = "vote_records"
		case "custom":
			value, evalErr := evaluateCustomExpression(module.Expression, customExpressionContext{
				TeamScore:        parentScore,
				TeamRank:         parentRank,
				QuarterScoreByPD: quarterScoreMap,
				ExtraPoints:      extraPoints,
				ModuleRaw:        moduleRaw,
			})
			if evalErr != nil {
				return nil, evalErr
			}
			rawScore = roundToScale(value, 6)
			scoreDetail["source"] = "expression"
			scoreDetail["expression"] = module.Expression
		case "extra":
			rawScore = roundToScale(extraPoints, 6)
			scoreDetail["source"] = "extra_points"
		default:
			rawScore = 0
		}

		if module.MaxScore != nil && *module.MaxScore > 0 {
			rawScore = clamp(rawScore, 0, *module.MaxScore)
		}
		moduleRaw[module.ModuleKey] = rawScore

		weightedScore := 0.0
		if module.Weight != nil {
			weightedScore = roundToScale(rawScore*(*module.Weight), 6)
			weightedTotal = roundToScale(weightedTotal+weightedScore, 6)
		}

		detailJSON, _ := json.Marshal(scoreDetail)
		moduleDetails = append(moduleDetails, model.CalculatedModuleScore{
			ModuleID:      module.ID,
			ModuleCode:    module.ModuleCode,
			ModuleKey:     module.ModuleKey,
			ModuleName:    module.ModuleName,
			SortOrder:     module.SortOrder,
			RawScore:      rawScore,
			WeightedScore: weightedScore,
			ScoreDetail:   string(detailJSON),
		})
	}

	finalScore := roundToScale(weightedTotal+extraPoints, 6)
	finalScore = clamp(finalScore, 0, 120)

	moduleOrderKeys := make([]string, 0, len(moduleOrder))
	for _, item := range moduleOrder {
		moduleOrderKeys = append(moduleOrderKeys, item.ModuleKey)
	}
	rankBasisJSON, _ := json.Marshal(map[string]any{
		"moduleOrder":   moduleOrderKeys,
		"moduleRaw":     moduleRaw,
		"weightedScore": weightedTotal,
		"extraPoints":   extraPoints,
		"segmentCode":   node.SegmentCode,
		"ruleMatchMode": node.RuleMatchMode,
		"ruleId":        node.Rule.ID,
		"ownerOrgId":    node.OwnerOrgID,
		"ownerOrgType":  node.OwnerOrgType,
		"bindingId":     node.MatchedBindingID,
	})
	detailJSON, _ := json.Marshal(map[string]any{
		"objectType":     node.Object.ObjectType,
		"objectCategory": node.Object.ObjectCategory,
		"segmentCode":    node.SegmentCode,
		"ruleMatchMode":  node.RuleMatchMode,
		"ruleCategory":   node.Rule.ObjectCategory,
		"ownerOrgId":     node.OwnerOrgID,
		"ownerOrgType":   node.OwnerOrgType,
		"belongOrgId":    node.BelongOrgID,
		"bindingId":      node.MatchedBindingID,
		"moduleCount":    len(moduleOrder),
	})

	return &objectCalcResult{
		ObjectID:      node.Object.ID,
		RuleID:        node.Rule.ID,
		WeightedScore: weightedTotal,
		ExtraPoints:   extraPoints,
		FinalScore:    finalScore,
		RankBasis:     string(rankBasisJSON),
		DetailJSON:    string(detailJSON),
		Modules:       moduleDetails,
	}, nil
}

func (s *CalculationService) persistObjectCalculationTx(
	tx *gorm.DB,
	operatorID *uint,
	triggerMode string,
	now int64,
	periodCode string,
	input *objectCalcResult,
) error {
	var rule model.AssessmentRule
	if err := tx.Where("id = ?", input.RuleID).First(&rule).Error; err != nil {
		return fmt.Errorf("failed to load rule while persisting calculated score: %w", err)
	}

	var record model.CalculatedScore
	err := tx.Where("year_id = ? AND period_code = ? AND object_id = ?",
		rule.YearID, periodCode, input.ObjectID).
		First(&record).Error

	updates := map[string]any{
		"rule_id":        input.RuleID,
		"weighted_score": input.WeightedScore,
		"extra_points":   input.ExtraPoints,
		"final_score":    input.FinalScore,
		"rank_basis":     input.RankBasis,
		"detail_json":    input.DetailJSON,
		"trigger_mode":   triggerMode,
		"triggered_by":   operatorID,
		"calculated_at":  now,
		"updated_at":     now,
	}

	if repository.IsRecordNotFound(err) {
		record = model.CalculatedScore{
			YearID:        rule.YearID,
			PeriodCode:    periodCode,
			ObjectID:      input.ObjectID,
			RuleID:        input.RuleID,
			WeightedScore: input.WeightedScore,
			ExtraPoints:   input.ExtraPoints,
			FinalScore:    input.FinalScore,
			RankBasis:     input.RankBasis,
			DetailJSON:    input.DetailJSON,
			TriggerMode:   triggerMode,
			TriggeredBy:   operatorID,
			CalculatedAt:  now,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("failed to create calculated score: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query calculated score: %w", err)
	} else {
		if err := tx.Model(&model.CalculatedScore{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update calculated score: %w", err)
		}
	}

	if err := tx.Where("calculated_score_id = ?", record.ID).Delete(&model.CalculatedModuleScore{}).Error; err != nil {
		return fmt.Errorf("failed to clear old calculated module scores: %w", err)
	}
	for _, item := range input.Modules {
		row := item
		row.CalculatedScoreID = record.ID
		row.CreatedAt = now
		row.UpdatedAt = now
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("failed to create calculated module score: %w", err)
		}
	}
	return nil
}

func (s *CalculationService) refreshRankingsTx(tx *gorm.DB, yearID uint, periodCode string, now int64) error {
	if err := tx.Where("year_id = ? AND period_code = ?", yearID, periodCode).Delete(&model.Ranking{}).Error; err != nil {
		return fmt.Errorf("failed to clear rankings: %w", err)
	}

	var rows []rankingContextRow
	err := tx.Table("calculated_scores cs").
		Select("cs.id AS calculated_score_id, cs.object_id, ao.object_type, ao.object_category, ao.object_name, ao.parent_object_id, cs.final_score AS score").
		Joins("JOIN assessment_objects ao ON ao.id = cs.object_id").
		Where("cs.year_id = ? AND cs.period_code = ?", yearID, periodCode).
		Scan(&rows).Error
	if err != nil {
		return fmt.Errorf("failed to load ranking context: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}

	calcIDs := make([]uint, 0, len(rows))
	for _, item := range rows {
		calcIDs = append(calcIDs, item.CalculatedScoreID)
	}
	modulePriorityMap, err := s.loadModulePriorityForRankingTx(tx, calcIDs)
	if err != nil {
		return err
	}

	type partitionKey struct {
		Scope    string
		ScopeKey string
	}
	partitions := make(map[partitionKey][]rankingContextRow)
	for _, item := range rows {
		overallKey := partitionKey{
			Scope:    rankingScopeOverall,
			ScopeKey: item.ObjectType + "|" + item.ObjectCategory,
		}
		partitions[overallKey] = append(partitions[overallKey], item)

		if item.ObjectType == "individual" && item.ParentObjectID != nil {
			parentKey := partitionKey{
				Scope:    rankingScopeParent,
				ScopeKey: strconv.FormatUint(uint64(*item.ParentObjectID), 10),
			}
			partitions[parentKey] = append(partitions[parentKey], item)
		}
	}

	insertRows := make([]model.Ranking, 0, len(rows)*2)
	for key, items := range partitions {
		sort.Slice(items, func(i, j int) bool {
			if !floatEquals(items[i].Score, items[j].Score) {
				return items[i].Score > items[j].Score
			}
			pi := modulePriorityMap[items[i].CalculatedScoreID]
			pj := modulePriorityMap[items[j].CalculatedScoreID]
			maxLen := len(pi)
			if len(pj) > maxLen {
				maxLen = len(pj)
			}
			for idx := 0; idx < maxLen; idx++ {
				vi := math.Inf(-1)
				vj := math.Inf(-1)
				if idx < len(pi) {
					vi = pi[idx].RawScore
				}
				if idx < len(pj) {
					vj = pj[idx].RawScore
				}
				if !floatEquals(vi, vj) {
					return vi > vj
				}
			}
			if items[i].ObjectName != items[j].ObjectName {
				return items[i].ObjectName < items[j].ObjectName
			}
			return items[i].ObjectID < items[j].ObjectID
		})

		for idx, item := range items {
			tieBreak, _ := json.Marshal(map[string]any{
				"finalScore":      item.Score,
				"moduleRankBasis": modulePriorityMap[item.CalculatedScoreID],
			})
			insertRows = append(insertRows, model.Ranking{
				YearID:            yearID,
				PeriodCode:        periodCode,
				ObjectID:          item.ObjectID,
				ObjectType:        item.ObjectType,
				ObjectCategory:    item.ObjectCategory,
				RankingScope:      key.Scope,
				ScopeKey:          key.ScopeKey,
				RankNo:            idx + 1,
				Score:             item.Score,
				TieBreakKey:       string(tieBreak),
				CalculatedScoreID: item.CalculatedScoreID,
				CreatedAt:         now,
				UpdatedAt:         now,
			})
		}
	}

	if len(insertRows) > 0 {
		if err := tx.Create(&insertRows).Error; err != nil {
			return fmt.Errorf("failed to save rankings: %w", err)
		}
	}
	return nil
}

func (s *CalculationService) loadModulePriorityForRankingTx(tx *gorm.DB, calculatedScoreIDs []uint) (map[uint][]modulePriorityValue, error) {
	result := make(map[uint][]modulePriorityValue, len(calculatedScoreIDs))
	if len(calculatedScoreIDs) == 0 {
		return result, nil
	}
	var rows []model.CalculatedModuleScore
	if err := tx.Where("calculated_score_id IN ? AND module_code <> 'extra'", calculatedScoreIDs).
		Order("calculated_score_id ASC, sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to load module priority context: %w", err)
	}
	for _, item := range rows {
		result[item.CalculatedScoreID] = append(result[item.CalculatedScoreID], modulePriorityValue{
			SortOrder: item.SortOrder,
			RawScore:  item.RawScore,
		})
	}
	return result, nil
}

func calculateVoteModuleRawScore(module model.ScoreModule, groups []model.VoteGroup, rows []voteAggRow, voteGradeScores map[string]float64) float64 {
	if len(groups) == 0 {
		return 0
	}
	if voteGradeScores == nil {
		voteGradeScores = defaultVoteGradeScores
	}
	type groupAgg struct {
		Total int
		Sum   float64
	}
	aggByGroup := make(map[uint]*groupAgg, len(groups))
	for _, row := range rows {
		baseScore, ok := voteGradeScores[strings.ToLower(strings.TrimSpace(row.GradeOption))]
		if !ok {
			continue
		}
		agg := aggByGroup[row.VoteGroupID]
		if agg == nil {
			agg = &groupAgg{}
			aggByGroup[row.VoteGroupID] = agg
		}
		agg.Total += row.Count
		agg.Sum += baseScore * float64(row.Count)
	}

	total := 0.0
	for _, group := range groups {
		agg := aggByGroup[group.ID]
		if agg == nil || agg.Total == 0 {
			continue
		}
		groupMax := group.MaxScore
		if groupMax <= 0 {
			groupMax = 100
		}
		groupAvg := (agg.Sum / float64(agg.Total)) * (groupMax / 100)
		total += groupAvg * group.Weight
	}
	if module.MaxScore != nil && *module.MaxScore > 0 {
		total = clamp(total, 0, *module.MaxScore)
	}
	return total
}

func parseVoteGradeScoresSetting(value string) (map[string]float64, error) {
	var raw map[string]float64
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &raw); err != nil {
		return nil, err
	}
	result := make(map[string]float64, len(raw))
	for key, score := range raw {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		if _, ok := voteGradeOptionSet[normalizedKey]; !ok {
			return nil, fmt.Errorf("invalid vote grade option")
		}
		if _, exists := result[normalizedKey]; exists {
			return nil, fmt.Errorf("duplicate vote grade option")
		}
		if math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 100 {
			return nil, fmt.Errorf("invalid vote grade score")
		}
		result[normalizedKey] = score
	}
	if len(result) != len(voteGradeOptionSet) {
		return nil, fmt.Errorf("incomplete vote grade score settings")
	}
	return result, nil
}

func cloneVoteGradeScores(source map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func resolveModuleCalcOrder(modules []model.ScoreModule) ([]model.ScoreModule, error) {
	if len(modules) == 0 {
		return []model.ScoreModule{}, nil
	}
	moduleByKey := make(map[string]model.ScoreModule, len(modules))
	inDegree := make(map[string]int, len(modules))
	adj := make(map[string][]string, len(modules))
	keys := make([]string, 0, len(modules))
	for _, item := range modules {
		moduleByKey[item.ModuleKey] = item
		inDegree[item.ModuleKey] = 0
		keys = append(keys, item.ModuleKey)
	}

	for _, module := range modules {
		if module.ModuleCode != "custom" || strings.TrimSpace(module.Expression) == "" {
			continue
		}
		refs := extractModuleRefs(module.Expression)
		for _, ref := range refs {
			if _, ok := moduleByKey[ref]; !ok {
				continue
			}
			adj[ref] = append(adj[ref], module.ModuleKey)
			inDegree[module.ModuleKey]++
		}
	}
	for from := range adj {
		sort.Strings(adj[from])
	}
	sort.Slice(keys, func(i, j int) bool {
		left := moduleByKey[keys[i]]
		right := moduleByKey[keys[j]]
		if left.SortOrder == right.SortOrder {
			return left.ID < right.ID
		}
		return left.SortOrder < right.SortOrder
	})

	queue := make([]string, 0, len(keys))
	for _, key := range keys {
		if inDegree[key] == 0 {
			queue = append(queue, key)
		}
	}

	order := make([]model.ScoreModule, 0, len(modules))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, moduleByKey[current])
		for _, next := range adj[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
		sort.Slice(queue, func(i, j int) bool {
			left := moduleByKey[queue[i]]
			right := moduleByKey[queue[j]]
			if left.SortOrder == right.SortOrder {
				return left.ID < right.ID
			}
			return left.SortOrder < right.SortOrder
		})
	}

	if len(order) != len(modules) {
		return nil, ErrCalcDependencyCycle
	}
	return order, nil
}

func extractModuleRefs(expression string) []string {
	matches := expressionIdentifierPattern.FindAllString(strings.TrimSpace(expression), -1)
	if len(matches) == 0 {
		return []string{}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(matches))
	for _, item := range matches {
		if !strings.HasPrefix(item, "module_") || len(item) <= len("module_") {
			continue
		}
		key := strings.TrimPrefix(item, "module_")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

type customExpressionContext struct {
	TeamScore        float64
	TeamRank         int
	QuarterScoreByPD map[string]float64
	ExtraPoints      float64
	ModuleRaw        map[string]float64
}

func evaluateCustomExpression(expression string, ctx customExpressionContext) (float64, error) {
	funcs := map[string]govaluate.ExpressionFunction{
		"abs": func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, ErrCalcExpressionEval
			}
			value, err := asFloat(args[0])
			if err != nil {
				return nil, err
			}
			return math.Abs(value), nil
		},
		"round": func(args ...interface{}) (interface{}, error) {
			if len(args) == 0 || len(args) > 2 {
				return nil, ErrCalcExpressionEval
			}
			value, err := asFloat(args[0])
			if err != nil {
				return nil, err
			}
			scale := 0
			if len(args) == 2 {
				scaleValue, scaleErr := asFloat(args[1])
				if scaleErr != nil {
					return nil, scaleErr
				}
				scale = int(scaleValue)
			}
			if scale < 0 {
				scale = 0
			}
			if scale > 6 {
				scale = 6
			}
			return roundToScale(value, scale), nil
		},
		"ceil": func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, ErrCalcExpressionEval
			}
			value, err := asFloat(args[0])
			if err != nil {
				return nil, err
			}
			return math.Ceil(value), nil
		},
		"floor": func(args ...interface{}) (interface{}, error) {
			if len(args) != 1 {
				return nil, ErrCalcExpressionEval
			}
			value, err := asFloat(args[0])
			if err != nil {
				return nil, err
			}
			return math.Floor(value), nil
		},
		"max": func(args ...interface{}) (interface{}, error) {
			if len(args) == 0 {
				return nil, ErrCalcExpressionEval
			}
			maxVal := math.Inf(-1)
			for _, arg := range args {
				value, err := asFloat(arg)
				if err != nil {
					return nil, err
				}
				if value > maxVal {
					maxVal = value
				}
			}
			return maxVal, nil
		},
		"min": func(args ...interface{}) (interface{}, error) {
			if len(args) == 0 {
				return nil, ErrCalcExpressionEval
			}
			minVal := math.Inf(1)
			for _, arg := range args {
				value, err := asFloat(arg)
				if err != nil {
					return nil, err
				}
				if value < minVal {
					minVal = value
				}
			}
			return minVal, nil
		},
		"if": func(args ...interface{}) (interface{}, error) {
			if len(args) != 3 {
				return nil, ErrCalcExpressionEval
			}
			cond, err := asBool(args[0])
			if err != nil {
				return nil, err
			}
			if cond {
				return asFloat(args[1])
			}
			return asFloat(args[2])
		},
		"sum": func(args ...interface{}) (interface{}, error) {
			total := 0.0
			for _, arg := range args {
				value, err := asFloat(arg)
				if err != nil {
					return nil, err
				}
				total += value
			}
			return total, nil
		},
		"avg": func(args ...interface{}) (interface{}, error) {
			if len(args) == 0 {
				return 0.0, nil
			}
			total := 0.0
			for _, arg := range args {
				value, err := asFloat(arg)
				if err != nil {
					return nil, err
				}
				total += value
			}
			return total / float64(len(args)), nil
		},
	}
	sanitizedExpression, tokenMap := rewriteExpressionForGovaluate(strings.TrimSpace(expression))
	expr, err := govaluate.NewEvaluableExpressionWithFunctions(sanitizedExpression, funcs)
	if err != nil {
		return 0, ErrCalcExpressionEval
	}

	parameters := map[string]any{}
	for originToken, safeToken := range tokenMap {
		switch {
		case originToken == "team.score":
			parameters[safeToken] = ctx.TeamScore
		case originToken == "team.rank":
			parameters[safeToken] = float64(ctx.TeamRank)
		case originToken == "q1.score":
			parameters[safeToken] = ctx.QuarterScoreByPD["Q1"]
		case originToken == "q2.score":
			parameters[safeToken] = ctx.QuarterScoreByPD["Q2"]
		case originToken == "q3.score":
			parameters[safeToken] = ctx.QuarterScoreByPD["Q3"]
		case originToken == "q4.score":
			parameters[safeToken] = ctx.QuarterScoreByPD["Q4"]
		case originToken == "extra_points":
			parameters[safeToken] = ctx.ExtraPoints
		case strings.HasPrefix(originToken, "module_"):
			moduleKey := strings.TrimPrefix(originToken, "module_")
			parameters[safeToken] = ctx.ModuleRaw[moduleKey]
		case strings.HasPrefix(originToken, "org."):
			parameters[safeToken] = 0.0
		default:
			parameters[safeToken] = 0.0
		}
	}

	value, err := expr.Evaluate(parameters)
	if err != nil {
		return 0, ErrCalcExpressionEval
	}
	score, err := asFloat(value)
	if err != nil {
		return 0, ErrCalcExpressionEval
	}
	return roundToScale(score, 6), nil
}

func rewriteExpressionForGovaluate(expression string) (string, map[string]string) {
	indices := expressionIdentifierPattern.FindAllStringIndex(expression, -1)
	if len(indices) == 0 {
		return expression, map[string]string{}
	}
	var builder strings.Builder
	builder.Grow(len(expression) + 16)
	tokenMap := make(map[string]string, len(indices))
	last := 0
	for _, item := range indices {
		start := item[0]
		end := item[1]
		token := expression[start:end]
		builder.WriteString(expression[last:start])

		isFunction := nextNonSpaceChar(expression[end:]) == '('
		if isFunction {
			builder.WriteString(token)
		} else {
			safeToken := sanitizeVariableToken(token)
			builder.WriteString(safeToken)
			tokenMap[token] = safeToken
		}
		last = end
	}
	builder.WriteString(expression[last:])
	return builder.String(), tokenMap
}

func sanitizeVariableToken(token string) string {
	safe := strings.ReplaceAll(token, ".", "__")
	safe = strings.ReplaceAll(safe, "-", "_")
	return safe
}

func asFloat(input interface{}) (float64, error) {
	switch value := input.(type) {
	case float64:
		return value, nil
	case float32:
		return float64(value), nil
	case int:
		return float64(value), nil
	case int64:
		return float64(value), nil
	case int32:
		return float64(value), nil
	case uint:
		return float64(value), nil
	case uint64:
		return float64(value), nil
	case uint32:
		return float64(value), nil
	case json.Number:
		return value.Float64()
	case bool:
		if value {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, ErrCalcExpressionEval
	}
}

func asBool(input interface{}) (bool, error) {
	switch value := input.(type) {
	case bool:
		return value, nil
	default:
		number, err := asFloat(input)
		if err != nil {
			return false, ErrCalcExpressionEval
		}
		return !floatEquals(number, 0), nil
	}
}

func clamp(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func floatEquals(left float64, right float64) bool {
	return math.Abs(left-right) < 0.0000001
}
