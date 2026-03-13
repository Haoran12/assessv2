export type AssessmentYearStatus = "preparing" | "active" | "ended";
export type AssessmentPeriodCode = "Q1" | "Q2" | "Q3" | "Q4" | "YEAR_END";
export type AssessmentPeriodStatus = "not_started" | "active" | "ended" | "locked";
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

export interface AssessmentYearItem {
  id: number;
  year: number;
  yearName: string;
  status: AssessmentYearStatus;
  startDate?: string;
  endDate?: string;
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
  startDate?: string;
  endDate?: string;
  createdAt: number;
  updatedAt: number;
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

export interface CreateAssessmentYearPayload {
  year: number;
  yearName?: string;
  description?: string;
  startDate?: string;
  endDate?: string;
  copyFromYearId?: number;
}

export interface CreateAssessmentYearResult {
  year: AssessmentYearItem;
  periods: AssessmentPeriodItem[];
  objectsCount: number;
}
