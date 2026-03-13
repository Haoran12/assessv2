package service

import "strings"

const (
	ObjectTypeTeam       = "team"
	ObjectTypeIndividual = "individual"
)

const (
	TeamCategoryGroup                           = "group"
	TeamCategoryGroupLeadership                 = "group_leadership_team"
	TeamCategoryGroupDepartment                 = "group_department"
	TeamCategorySubsidiaryCompany               = "subsidiary_company"
	TeamCategorySubsidiaryCompanyLeadership     = "subsidiary_company_leadership_team"
	TeamCategorySubsidiaryCompanyDepartment     = "subsidiary_company_department"
	IndividualCategoryLeadershipMain            = "leadership_main"
	IndividualCategoryLeadershipDeputy          = "leadership_deputy"
	IndividualCategoryDepartmentMain            = "department_main"
	IndividualCategoryDepartmentDeputy          = "department_deputy"
	IndividualCategoryGeneralManagementPersonnel = "general_management_personnel"
)

var teamCategoryList = []string{
	TeamCategoryGroup,
	TeamCategoryGroupLeadership,
	TeamCategoryGroupDepartment,
	TeamCategorySubsidiaryCompany,
	TeamCategorySubsidiaryCompanyLeadership,
	TeamCategorySubsidiaryCompanyDepartment,
}

var individualCategoryList = []string{
	IndividualCategoryLeadershipMain,
	IndividualCategoryLeadershipDeputy,
	IndividualCategoryDepartmentMain,
	IndividualCategoryDepartmentDeputy,
	IndividualCategoryGeneralManagementPersonnel,
}

var categorySetByObjectType = map[string]map[string]struct{}{
	ObjectTypeTeam:       toSet(teamCategoryList),
	ObjectTypeIndividual: toSet(individualCategoryList),
}

var legacyCategoryAlias = map[string]string{
	"group_dept":    TeamCategoryGroupDepartment,
	"company":       TeamCategorySubsidiaryCompany,
	"company_dept":  TeamCategorySubsidiaryCompanyDepartment,
	"group_leader":  IndividualCategoryLeadershipMain,
	"company_leader": IndividualCategoryLeadershipDeputy,
	"manager_main":  IndividualCategoryDepartmentMain,
	"manager_deputy": IndividualCategoryDepartmentDeputy,
	"staff":         IndividualCategoryGeneralManagementPersonnel,
}

func normalizeObjectType(value string) (string, bool) {
	objectType := strings.TrimSpace(strings.ToLower(value))
	_, ok := categorySetByObjectType[objectType]
	return objectType, ok
}

func normalizeObjectCategory(value string) string {
	category := strings.TrimSpace(strings.ToLower(value))
	if mapped, ok := legacyCategoryAlias[category]; ok {
		return mapped
	}
	return category
}

func isSupportedCategoryForObjectType(objectType, category string) bool {
	categories, ok := categorySetByObjectType[objectType]
	if !ok {
		return false
	}
	_, ok = categories[category]
	return ok
}

func inferObjectTypeByCategory(category string) (string, bool) {
	for objectType, categories := range categorySetByObjectType {
		if _, ok := categories[category]; ok {
			return objectType, true
		}
	}
	return "", false
}

func normalizeIndividualCategoryFromLevelCode(levelCode string) string {
	normalized := normalizeObjectCategory(levelCode)
	switch normalized {
	case IndividualCategoryLeadershipMain,
		IndividualCategoryLeadershipDeputy,
		IndividualCategoryDepartmentMain,
		IndividualCategoryDepartmentDeputy,
		IndividualCategoryGeneralManagementPersonnel:
		return normalized
	default:
		return IndividualCategoryGeneralManagementPersonnel
	}
}

func toSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
