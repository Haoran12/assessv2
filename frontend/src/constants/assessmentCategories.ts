import type {
  AssessmentObjectCategory,
  AssessmentObjectType,
  GlobalAssessmentObjectCategory,
} from "@/types/assessment";

type AssessmentCategoryDefinition = {
  code: AssessmentObjectCategory;
  name: string;
  objectType: AssessmentObjectType;
  sortOrder: number;
};

const DEFAULT_ASSESSMENT_CATEGORY_DEFINITIONS: AssessmentCategoryDefinition[] = [
  { code: "group", name: "集团", objectType: "team", sortOrder: 1 },
  { code: "group_leadership_team", name: "集团领导班子", objectType: "team", sortOrder: 2 },
  { code: "group_department", name: "集团部门", objectType: "team", sortOrder: 3 },
  { code: "subsidiary_company", name: "权属企业", objectType: "team", sortOrder: 4 },
  { code: "subsidiary_company_leadership_team", name: "权属企业领导班子", objectType: "team", sortOrder: 5 },
  { code: "subsidiary_company_department", name: "权属企业部门", objectType: "team", sortOrder: 6 },
  { code: "leadership_main", name: "领导班子正职", objectType: "individual", sortOrder: 101 },
  { code: "leadership_deputy", name: "领导班子副职", objectType: "individual", sortOrder: 102 },
  { code: "department_main", name: "部门正职", objectType: "individual", sortOrder: 103 },
  { code: "department_deputy", name: "部门副职", objectType: "individual", sortOrder: 104 },
  { code: "general_management_personnel", name: "一般管理人员", objectType: "individual", sortOrder: 105 },
];

const ALL_CATEGORY_CODE_SET = new Set<AssessmentObjectCategory>(
  DEFAULT_ASSESSMENT_CATEGORY_DEFINITIONS.map((item) => item.code),
);

let runtimeCategoryDefinitions: AssessmentCategoryDefinition[] = [...DEFAULT_ASSESSMENT_CATEGORY_DEFINITIONS];

function rebuildRuntimeDefinitions(definitions: AssessmentCategoryDefinition[]): void {
  runtimeCategoryDefinitions = [...definitions].sort((a, b) => a.sortOrder - b.sortOrder);
}

function runtimeCategoriesByType(objectType: AssessmentObjectType): AssessmentObjectCategory[] {
  return runtimeCategoryDefinitions
    .filter((item) => item.objectType === objectType)
    .map((item) => item.code);
}

function runtimeCategoryLabelMap(): Record<AssessmentObjectCategory, string> {
  const result = {} as Record<AssessmentObjectCategory, string>;
  for (const item of runtimeCategoryDefinitions) {
    result[item.code] = item.name;
  }
  return result;
}

export function applyAssessmentCategoryDefinitions(
  items: Array<{
    categoryCode: string;
    categoryName: string;
    objectType: string;
    sortOrder: number;
  }>,
): void {
  const normalized: AssessmentCategoryDefinition[] = [];
  for (const item of items) {
    if (!isAssessmentObjectCategory(item.categoryCode)) {
      continue;
    }
    if (item.objectType !== "team" && item.objectType !== "individual") {
      continue;
    }
    normalized.push({
      code: item.categoryCode,
      name: item.categoryName?.trim() || item.categoryCode,
      objectType: item.objectType,
      sortOrder: Number.isFinite(item.sortOrder) ? item.sortOrder : 0,
    });
  }
  if (normalized.length === 0) {
    return;
  }
  rebuildRuntimeDefinitions(normalized);
}

export function assessmentCategoriesByObjectType(objectType: AssessmentObjectType): AssessmentObjectCategory[] {
  return runtimeCategoriesByType(objectType);
}

export function allAssessmentCategories(): AssessmentObjectCategory[] {
  return runtimeCategoryDefinitions.map((item) => item.code);
}

export function isAssessmentObjectCategory(value: string): value is AssessmentObjectCategory {
  return ALL_CATEGORY_CODE_SET.has(value as AssessmentObjectCategory);
}

export function objectTypeByCategory(category: AssessmentObjectCategory): AssessmentObjectType {
  const found = runtimeCategoryDefinitions.find((item) => item.code === category);
  return found?.objectType ?? "individual";
}

export function assessmentCategoryLabel(value: string): string {
  if (!isAssessmentObjectCategory(value)) {
    return value;
  }
  return runtimeCategoryLabelMap()[value] ?? value;
}

export interface GlobalAssessmentCategoryOption {
  value: GlobalAssessmentObjectCategory;
  label: string;
}

export function globalAssessmentCategoryOptions(): GlobalAssessmentCategoryOption[] {
  const options: GlobalAssessmentCategoryOption[] = [{ value: "all", label: "全部分类" }];
  const labelMap = runtimeCategoryLabelMap();
  for (const code of runtimeCategoriesByType("team")) {
    options.push({ value: code, label: `[团体] ${labelMap[code]}` });
  }
  for (const code of runtimeCategoriesByType("individual")) {
    options.push({ value: code, label: `[个人] ${labelMap[code]}` });
  }
  return options;
}
