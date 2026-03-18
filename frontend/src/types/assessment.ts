export type AssessmentPeriodCode = string;
export type AssessmentObjectType = "team" | "individual";
export type GlobalAssessmentObjectType = AssessmentObjectType | "all";
export type AssessmentObjectCategory =
  | "group"
  | "group_leadership_team"
  | "group_department"
  | "subsidiary_company"
  | "subsidiary_company_leadership_team"
  | "subsidiary_company_department"
  | "leadership_main"
  | "leadership_deputy"
  | "department_main"
  | "department_deputy"
  | "general_management_personnel";
export type GlobalAssessmentObjectCategory = AssessmentObjectCategory | "all";
export type AssessmentPeriodStatus = "preparing" | "active" | "completed";
export type AssessmentYearStatus = "preparing" | "active" | "completed";

export interface AssessmentSessionItem {
  id: number;
  assessmentName: string;
  displayName: string;
  year: number;
  organizationId: number;
  organizationName: string;
  description: string;
  dataDir: string;
  createdAt: number;
  updatedAt: number;
}

export interface AssessmentSessionPeriodItem {
  id: number;
  assessmentId: number;
  periodCode: AssessmentPeriodCode;
  periodName: string;
  sortOrder: number;
  createdAt: number;
  updatedAt: number;
}

export interface AssessmentObjectGroupItem {
  id: number;
  assessmentId: number;
  objectType: AssessmentObjectType;
  groupCode: string;
  groupName: string;
  sortOrder: number;
  isSystem: boolean;
  createdAt: number;
  updatedAt: number;
}

export interface AssessmentSessionObjectItem {
  id: number;
  assessmentId: number;
  objectType: AssessmentObjectType;
  groupCode: string;
  targetType: "organization" | "department" | "employee";
  targetId: number;
  objectName: string;
  parentObjectId?: number;
  sortOrder: number;
  isActive: boolean;
  createdAt: number;
  updatedAt: number;
}

export interface AssessmentObjectCandidateItem {
  targetType: "organization" | "department" | "employee";
  targetId: number;
  objectName: string;
  organizationId: number;
  organizationName: string;
  departmentId?: number;
  departmentName?: string;
  recommendedObjectType: AssessmentObjectType;
  recommendedGroupCode: string;
}

// Legacy compatibility types (deprecated)
export interface AssessmentYearItem {
  id: number;
  year: number;
  status: AssessmentYearStatus;
  description: string;
  createdAt: number;
  updatedAt: number;
}

export interface AssessmentPeriodItem {
  id: number;
  yearId: number;
  periodCode: AssessmentPeriodCode;
  periodName: string;
  status: AssessmentPeriodStatus;
  createdAt: number;
  updatedAt: number;
}

export interface AssessmentPeriodTemplateItem {
  periodCode: AssessmentPeriodCode;
  periodName: string;
  sortOrder: number;
}

export interface AssessmentObjectItem {
  id: number;
  yearId: number;
  objectType: AssessmentObjectType;
  objectCategory: AssessmentObjectCategory;
  targetId: number;
  targetType: "organization" | "department" | "employee" | "leadership_team";
  objectName: string;
  parentObjectId?: number;
  isActive: boolean;
  createdAt: number;
  updatedAt: number;
}

export interface AssessmentSessionDetail {
  session: AssessmentSessionItem;
  periods: AssessmentSessionPeriodItem[];
  objectGroups: AssessmentObjectGroupItem[];
  objectCount: number;
}

export interface CreateAssessmentSessionPayload {
  year: number;
  organizationId: number;
  displayName?: string;
  description?: string;
}

export interface UpdateAssessmentSessionPayload {
  displayName?: string;
  description?: string;
}

export interface UpdateAssessmentPeriodsPayload {
  items: Array<{
    periodCode: string;
    periodName: string;
    sortOrder?: number;
  }>;
}

export interface UpdateAssessmentObjectGroupsPayload {
  items: Array<{
    objectType: AssessmentObjectType;
    groupCode: string;
    groupName: string;
    sortOrder?: number;
  }>;
}

export interface UpdateAssessmentObjectsPayload {
  items: Array<{
    objectType: AssessmentObjectType;
    groupCode: string;
    targetType: "organization" | "department" | "employee";
    targetId: number;
    parentTargetType?: "organization" | "department" | "employee";
    parentTargetId?: number;
    sortOrder?: number;
    isActive: boolean;
  }>;
}
