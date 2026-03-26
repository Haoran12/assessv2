package service

import (
	"fmt"
	"strings"
)

const (
	extraAdjustModuleKey   = "__extra_adjust__"
	extraAdjustModuleName  = "额外加减分"
	extraAdjustScoreMin    = -20.0
	extraAdjustScoreMax    = 20.0
	extraAdjustModuleOrder = 0
)

func isExtraAdjustModuleKey(moduleKey string) bool {
	return strings.TrimSpace(moduleKey) == extraAdjustModuleKey
}

func ensureExtraAdjustModuleNodes(modules []calculationScoreNode) []calculationScoreNode {
	normalized := make([]calculationScoreNode, 0, len(modules)+1)
	extra := calculationScoreNode{}
	found := false
	for _, module := range modules {
		moduleKey := strings.TrimSpace(module.ModuleKey)
		if moduleKey == "" {
			moduleKey = strings.TrimSpace(module.ID)
		}
		if !isExtraAdjustModuleKey(moduleKey) {
			normalized = append(normalized, module)
			continue
		}
		if !found {
			extra = module
			found = true
		}
	}
	normalized = append(normalized, normalizedExtraAdjustModuleNode(extra, found))
	return normalized
}

func normalizedExtraAdjustModuleNode(source calculationScoreNode, exists bool) calculationScoreNode {
	item := calculationScoreNode{}
	if exists {
		item = source
	}
	moduleID := strings.TrimSpace(item.ID)
	if moduleID == "" {
		moduleID = extraAdjustModuleKey
	}
	item.ID = moduleID
	item.ModuleKey = extraAdjustModuleKey
	item.ModuleName = extraAdjustModuleName
	item.Weight = extraAdjustModuleOrder
	item.CalculationMethod = "direct_input"
	item.CustomScript = ""
	item.VoteConfig = nil
	item.Detail = nil
	return item
}

func ensureExtraAdjustModuleRows(raw any) []any {
	rows, ok := raw.([]any)
	if !ok {
		rows = []any{}
	}
	normalized := make([]any, 0, len(rows)+1)
	extra := map[string]any{}
	found := false
	for _, entry := range rows {
		row, ok := entry.(map[string]any)
		if !ok {
			normalized = append(normalized, entry)
			continue
		}
		moduleKey := strings.TrimSpace(anyToText(row["moduleKey"]))
		if moduleKey == "" {
			moduleKey = strings.TrimSpace(anyToText(row["id"]))
		}
		if !isExtraAdjustModuleKey(moduleKey) {
			normalized = append(normalized, row)
			continue
		}
		if !found {
			extra = cloneAnyMapForExtraAdjust(row)
			found = true
		}
	}
	normalized = append(normalized, normalizedExtraAdjustModuleRow(extra, found))
	return normalized
}

func normalizedExtraAdjustModuleRow(source map[string]any, exists bool) map[string]any {
	row := map[string]any{}
	if exists {
		row = cloneAnyMapForExtraAdjust(source)
	}
	moduleID := strings.TrimSpace(anyToText(row["id"]))
	if moduleID == "" {
		moduleID = extraAdjustModuleKey
	}
	row["id"] = moduleID
	row["moduleKey"] = extraAdjustModuleKey
	row["moduleName"] = extraAdjustModuleName
	row["weight"] = extraAdjustModuleOrder
	row["calculationMethod"] = "direct_input"
	row["customScript"] = ""
	delete(row, "voteConfig")
	delete(row, "detail")
	return row
}

func anyToText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func cloneAnyMapForExtraAdjust(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
