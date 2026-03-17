package service

import (
	"strings"

	"assessv2/backend/internal/model"
)

const (
	ruleBindingOwnerScopeGlobal           = "global"
	ruleBindingOwnerScopeOrganizationType = "organization_type"
	ruleBindingOwnerScopeOrganization     = "organization"
)

func selectRuleBindingForObject(
	bindings []model.RuleBinding,
	objectType string,
	segmentCode string,
	ownerContext objectOwnerContext,
) *model.RuleBinding {
	if len(bindings) == 0 {
		return nil
	}
	normalizedObjectType := strings.ToLower(strings.TrimSpace(objectType))
	normalizedSegmentCode := normalizeSegmentCode(segmentCode)
	if normalizedObjectType == "" || normalizedSegmentCode == "" {
		return nil
	}

	var selected *model.RuleBinding
	selectedSpecificity := -1
	for idx := range bindings {
		binding := &bindings[idx]
		if !binding.IsActive {
			continue
		}
		if strings.ToLower(strings.TrimSpace(binding.ObjectType)) != normalizedObjectType {
			continue
		}
		if normalizeSegmentCode(binding.SegmentCode) != normalizedSegmentCode {
			continue
		}
		specificity, matches := matchBindingOwnerScope(*binding, ownerContext)
		if !matches {
			continue
		}
		if selected == nil || specificity > selectedSpecificity {
			selected = binding
			selectedSpecificity = specificity
			continue
		}
		if specificity < selectedSpecificity {
			continue
		}
		if binding.Priority > selected.Priority {
			selected = binding
			continue
		}
		if binding.Priority < selected.Priority {
			continue
		}
		if binding.ID < selected.ID {
			selected = binding
		}
	}
	return selected
}

func matchBindingOwnerScope(binding model.RuleBinding, ownerContext objectOwnerContext) (int, bool) {
	switch normalizeBindingOwnerScope(binding.OwnerScope) {
	case ruleBindingOwnerScopeOrganization:
		if binding.OwnerOrgID == nil || ownerContext.OwnerOrgID == nil {
			return 0, false
		}
		if *binding.OwnerOrgID != *ownerContext.OwnerOrgID {
			return 0, false
		}
		return 3, true
	case ruleBindingOwnerScopeOrganizationType:
		bindingOwnerType := strings.ToLower(strings.TrimSpace(binding.OwnerOrgType))
		if bindingOwnerType == "" || ownerContext.OwnerOrgType == "" {
			return 0, false
		}
		if bindingOwnerType != ownerContext.OwnerOrgType {
			return 0, false
		}
		return 2, true
	case ruleBindingOwnerScopeGlobal:
		return 1, true
	default:
		return 0, false
	}
}

func normalizeBindingOwnerScope(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ruleBindingOwnerScopeOrganization:
		return ruleBindingOwnerScopeOrganization
	case ruleBindingOwnerScopeOrganizationType:
		return ruleBindingOwnerScopeOrganizationType
	case ruleBindingOwnerScopeGlobal:
		return ruleBindingOwnerScopeGlobal
	default:
		return ruleBindingOwnerScopeGlobal
	}
}
