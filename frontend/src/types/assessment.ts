export type AssessmentYearStatus = "preparing" | "active" | "ended";
export type AssessmentPeriodCode = "Q1" | "Q2" | "Q3" | "Q4" | "YEAR_END";
export type AssessmentPeriodStatus = "not_started" | "active" | "ended" | "locked";
export type AssessmentObjectType = "team" | "individual";

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
  objectCategory: string;
  targetId: number;
  targetType: "organization" | "department" | "employee";
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
