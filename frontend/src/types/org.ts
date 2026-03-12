export type OrgStatus = "active" | "inactive";
export type EmployeeStatus = "active" | "inactive";
export type OrganizationType = "group" | "company";
export type TransferType = "transfer" | "promotion" | "demotion" | "position_change";

export interface OrgTreeNode {
  id: number;
  nodeType: "organization" | "department" | "employee";
  name: string;
  status: string;
  organizationId?: number;
  departmentId?: number;
  parentId?: number;
  sortOrder: number;
  children?: OrgTreeNode[];
}

export interface OrganizationItem {
  id: number;
  orgName: string;
  orgType: OrganizationType;
  parentId?: number;
  leaderId?: number;
  sortOrder: number;
  status: OrgStatus;
  createdAt: number;
  updatedAt: number;
}

export interface DepartmentItem {
  id: number;
  deptName: string;
  organizationId: number;
  parentDeptId?: number;
  leaderId?: number;
  sortOrder: number;
  status: OrgStatus;
  createdAt: number;
  updatedAt: number;
}

export interface PositionLevelItem {
  id: number;
  levelCode: string;
  levelName: string;
  description: string;
  isSystem: boolean;
  isForAssessment: boolean;
  sortOrder: number;
  status: OrgStatus;
}

export interface EmployeeItem {
  id: number;
  empName: string;
  organizationId: number;
  departmentId?: number;
  positionLevelId: number;
  positionTitle: string;
  hireDate?: string;
  status: EmployeeStatus;
  createdAt: number;
  updatedAt: number;
}

export interface EmployeeHistoryItem {
  id: number;
  employeeId: number;
  changeType: TransferType;
  oldOrganizationId?: number;
  newOrganizationId?: number;
  oldDepartmentId?: number;
  newDepartmentId?: number;
  oldPositionLevelId?: number;
  newPositionLevelId?: number;
  oldPositionTitle: string;
  newPositionTitle: string;
  changeReason: string;
  effectiveDate: string;
  createdAt: number;
}

export interface UpsertOrganizationPayload {
  orgName: string;
  orgType: OrganizationType;
  parentId?: number;
  leaderId?: number;
  sortOrder: number;
  status: OrgStatus;
}

export interface UpsertDepartmentPayload {
  deptName: string;
  organizationId: number;
  parentDeptId?: number;
  leaderId?: number;
  sortOrder: number;
  status: OrgStatus;
}

export interface UpsertEmployeePayload {
  empName: string;
  organizationId: number;
  departmentId?: number;
  positionLevelId: number;
  positionTitle: string;
  hireDate?: string;
  status: EmployeeStatus;
}

export interface UpsertPositionLevelPayload {
  levelCode: string;
  levelName: string;
  description?: string;
  isForAssessment: boolean;
  sortOrder: number;
  status: OrgStatus;
}

export interface TransferEmployeePayload {
  changeType: TransferType;
  newOrganizationId?: number;
  newDepartmentId?: number;
  newPositionLevelId?: number;
  newPositionTitle?: string;
  changeReason: string;
  effectiveDate: string;
}
