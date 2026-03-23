package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"assessv2/backend/internal/model"
)

const auditDetailVersionV2 = "v2"

var auditSensitiveFieldTokens = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"credential",
	"private_key",
	"apikey",
	"accesskey",
	"refresh_token",
}

func buildAuditRecord(
	userID *uint,
	actionType string,
	targetType string,
	targetID *uint,
	detail map[string]any,
	ipAddress string,
	userAgent string,
) model.AuditLog {
	normalizedDetail, eventCode, summary, changes := normalizeAuditDetail(actionType, targetType, targetID, detail)
	detailBytes, _ := json.Marshal(normalizedDetail)
	changeCount := len(changes)
	return model.AuditLog{
		UserID:       userID,
		ActionType:   actionType,
		TargetType:   targetType,
		TargetID:     targetID,
		EventCode:    eventCode,
		Summary:      summary,
		ChangeCount:  changeCount,
		HasDiff:      changeCount > 0,
		ActionDetail: string(detailBytes),
		IPAddress:    normalizeIPAddress(ipAddress),
		UserAgent:    userAgent,
	}
}

func decodeActionDetail(raw string, actionType string, targetType string, targetID *uint) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		detail, _, _, _ := normalizeAuditDetail(actionType, targetType, targetID, map[string]any{})
		return detail
	}

	decoded := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		detail, _, _, _ := normalizeAuditDetail(actionType, targetType, targetID, map[string]any{
			"_raw": raw,
		})
		return detail
	}
	detail, _, _, _ := normalizeAuditDetail(actionType, targetType, targetID, decoded)
	return detail
}

func normalizeAuditDetail(
	actionType string,
	targetType string,
	targetID *uint,
	detail map[string]any,
) (map[string]any, string, string, []AuditDiffItem) {
	payload := cloneAnyMap(detail)

	eventCode := extractString(payload, "eventCode")
	if eventCode == "" {
		eventCode = extractString(payload, "event")
	}
	if eventCode == "" {
		eventCode = strings.ToLower(strings.TrimSpace(actionType)) + "." + strings.ToLower(strings.TrimSpace(targetType))
		eventCode = strings.Trim(eventCode, ".")
	}
	if eventCode == "" {
		eventCode = "unknown"
	}
	payload["eventCode"] = eventCode
	payload["version"] = auditDetailVersionV2

	before := pickMap(payload, "before")
	after := pickMap(payload, "after")
	before = pickMap(map[string]any{"before": maskSensitiveDataByPath(before, "")}, "before")
	after = pickMap(map[string]any{"after": maskSensitiveDataByPath(after, "")}, "after")
	if _, exists := payload["before"]; exists {
		payload["before"] = before
	}
	if _, exists := payload["after"]; exists {
		payload["after"] = after
	}

	fieldLabels := map[string]string{}
	for key, value := range pickMap(payload, "fieldLabels") {
		text := strings.TrimSpace(fmt.Sprintf("%v", value))
		if text == "" {
			continue
		}
		fieldLabels[key] = text
	}

	changes := parseAuditDiffItems(payload["changes"])
	if len(changes) == 0 && (len(before) > 0 || len(after) > 0) {
		changes = buildAuditDiffItems(before, after, fieldLabels)
	}
	for index := range changes {
		if changes[index].Label == "" {
			if label, exists := fieldLabels[changes[index].Field]; exists && strings.TrimSpace(label) != "" {
				changes[index].Label = strings.TrimSpace(label)
			} else {
				changes[index].Label = deriveFieldLabel(changes[index].Field)
			}
		}
		changes[index].Before = maskSensitiveDataByPath(changes[index].Before, changes[index].Field)
		changes[index].After = maskSensitiveDataByPath(changes[index].After, changes[index].Field)
		if strings.TrimSpace(changes[index].ChangeType) == "" {
			switch {
			case changes[index].Before == nil && changes[index].After != nil:
				changes[index].ChangeType = "added"
			case changes[index].Before != nil && changes[index].After == nil:
				changes[index].ChangeType = "removed"
			default:
				changes[index].ChangeType = "updated"
			}
		}
	}
	if len(changes) > 0 {
		payload["changes"] = encodeAuditDiffItems(changes)
	}

	summary := extractString(payload, "summary")
	if summary == "" {
		summary = buildAuditSummary(actionType, targetType, targetID, eventCode, changes)
	}
	payload["summary"] = summary

	return payload, eventCode, summary, changes
}

func buildAuditSummary(
	actionType string,
	targetType string,
	targetID *uint,
	eventCode string,
	changes []AuditDiffItem,
) string {
	target := strings.TrimSpace(targetType)
	if target == "" {
		target = "record"
	}
	targetRef := target
	if targetID != nil && *targetID > 0 {
		targetRef = target + "#" + strconv.FormatUint(uint64(*targetID), 10)
	}
	if len(changes) > 0 {
		return fmt.Sprintf("%s %s (%d fields changed)", strings.ToUpper(strings.TrimSpace(actionType)), targetRef, len(changes))
	}
	if strings.TrimSpace(eventCode) != "" {
		return eventCode
	}
	return strings.TrimSpace(actionType) + " " + targetRef
}

func diffActionDetail(detail map[string]any) []AuditDiffItem {
	items := parseAuditDiffItems(detail["changes"])
	if len(items) > 0 {
		return items
	}
	before := pickMap(detail, "before")
	after := pickMap(detail, "after")
	if len(before) == 0 && len(after) == 0 {
		return []AuditDiffItem{}
	}
	return buildAuditDiffItems(before, after, map[string]string{})
}

func buildAuditDiffItems(before map[string]any, after map[string]any, labels map[string]string) []AuditDiffItem {
	beforeFlat := flattenAuditMap(before)
	afterFlat := flattenAuditMap(after)

	fieldSet := map[string]struct{}{}
	for key := range beforeFlat {
		fieldSet[key] = struct{}{}
	}
	for key := range afterFlat {
		fieldSet[key] = struct{}{}
	}
	fields := make([]string, 0, len(fieldSet))
	for key := range fieldSet {
		fields = append(fields, key)
	}
	sort.Strings(fields)

	items := make([]AuditDiffItem, 0, len(fields))
	for _, key := range fields {
		beforeValue, beforeExists := beforeFlat[key]
		afterValue, afterExists := afterFlat[key]
		if !beforeExists && !afterExists {
			continue
		}
		changeType := "updated"
		switch {
		case !beforeExists && afterExists:
			changeType = "added"
		case beforeExists && !afterExists:
			changeType = "removed"
		default:
			if auditValuesEqual(beforeValue, afterValue) {
				continue
			}
		}
		label := labels[key]
		if strings.TrimSpace(label) == "" {
			label = deriveFieldLabel(key)
		}
		items = append(items, AuditDiffItem{
			Field:      key,
			Label:      label,
			Before:     beforeValue,
			After:      afterValue,
			ChangeType: changeType,
		})
	}
	return items
}

func parseAuditDiffItems(raw any) []AuditDiffItem {
	array, ok := raw.([]any)
	if !ok {
		return []AuditDiffItem{}
	}
	items := make([]AuditDiffItem, 0, len(array))
	for _, entry := range array {
		object, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		field := strings.TrimSpace(fmt.Sprintf("%v", object["field"]))
		if field == "" {
			continue
		}
		label := strings.TrimSpace(fmt.Sprintf("%v", object["label"]))
		changeType := strings.TrimSpace(fmt.Sprintf("%v", object["changeType"]))
		beforeValue, _ := object["before"]
		afterValue, _ := object["after"]
		items = append(items, AuditDiffItem{
			Field:      field,
			Label:      label,
			Before:     beforeValue,
			After:      afterValue,
			ChangeType: changeType,
		})
	}
	return items
}

func encodeAuditDiffItems(items []AuditDiffItem) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"field":      item.Field,
			"label":      item.Label,
			"before":     item.Before,
			"after":      item.After,
			"changeType": item.ChangeType,
		})
	}
	return result
}

func flattenAuditMap(input map[string]any) map[string]any {
	result := map[string]any{}
	flattenAuditValue("", input, result)
	delete(result, "")
	return result
}

func flattenAuditValue(path string, value any, out map[string]any) {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			if path != "" {
				out[path] = map[string]any{}
			}
			return
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next := key
			if path != "" {
				next = path + "." + key
			}
			flattenAuditValue(next, typed[key], out)
		}
	case []any:
		if len(typed) == 0 {
			if path != "" {
				out[path] = []any{}
			}
			return
		}
		for index, item := range typed {
			next := fmt.Sprintf("%s[%d]", path, index)
			if path == "" {
				next = fmt.Sprintf("[%d]", index)
			}
			flattenAuditValue(next, item, out)
		}
	default:
		out[path] = typed
	}
}

func auditValuesEqual(left any, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right)
	}
	return string(leftJSON) == string(rightJSON)
}

func deriveFieldLabel(path string) string {
	text := strings.TrimSpace(path)
	if text == "" {
		return ""
	}
	lastDot := strings.LastIndex(text, ".")
	if lastDot >= 0 && lastDot < len(text)-1 {
		text = text[lastDot+1:]
	}
	leftBracket := strings.Index(text, "[")
	if leftBracket >= 0 {
		text = text[:leftBracket]
	}
	text = strings.ReplaceAll(text, "_", " ")
	text = strings.TrimSpace(text)
	if text == "" {
		return path
	}
	return text
}

func maskSensitiveDataByPath(value any, path string) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			nextPath := key
			if strings.TrimSpace(path) != "" {
				nextPath = path + "." + key
			}
			result[key] = maskSensitiveDataByPath(item, nextPath)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			nextPath := path + "[]"
			result = append(result, maskSensitiveDataByPath(item, nextPath))
		}
		return result
	default:
		if !isSensitiveAuditPath(path) {
			return typed
		}
		switch text := typed.(type) {
		case string:
			if strings.TrimSpace(text) == "" {
				return ""
			}
		}
		return "***"
	}
}

func isSensitiveAuditPath(path string) bool {
	text := strings.ToLower(strings.TrimSpace(path))
	if text == "" {
		return false
	}
	for _, token := range auditSensitiveFieldTokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	buffer, err := json.Marshal(input)
	if err != nil {
		result := make(map[string]any, len(input))
		for key, value := range input {
			result[key] = value
		}
		return result
	}
	result := map[string]any{}
	if err := json.Unmarshal(buffer, &result); err != nil {
		result = make(map[string]any, len(input))
		for key, value := range input {
			result[key] = value
		}
		return result
	}
	return result
}
