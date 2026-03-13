import { http } from "@/api/http";
import type {
  AssessmentCategoryItem,
  DepartmentItem,
  EmployeeHistoryItem,
  EmployeeItem,
  OrgTreeNode,
  OrganizationItem,
  PositionLevelItem,
  TransferEmployeePayload,
  UpsertDepartmentPayload,
  UpsertEmployeePayload,
  UpsertOrganizationPayload,
  UpsertPositionLevelPayload,
} from "@/types/org";

interface ListQuery {
  status?: string;
  keyword?: string;
}

interface ListDepartmentQuery extends ListQuery {
  organizationId?: number;
}

interface ListEmployeeQuery extends ListQuery {
  organizationId?: number;
  departmentId?: number;
}

export async function getOrgTree(includeInactive = false): Promise<OrgTreeNode[]> {
  const response = await http.get("/api/org/tree", {
    params: { includeInactive },
  });
  return (response.data?.data?.items ?? []) as OrgTreeNode[];
}

export async function listOrganizations(params: ListQuery): Promise<OrganizationItem[]> {
  const response = await http.get("/api/org/organizations", { params });
  return (response.data?.data?.items ?? []) as OrganizationItem[];
}

export async function createOrganization(payload: UpsertOrganizationPayload): Promise<OrganizationItem> {
  const response = await http.post("/api/org/organizations", payload);
  return response.data?.data as OrganizationItem;
}

export async function updateOrganization(
  organizationId: number,
  payload: UpsertOrganizationPayload,
): Promise<OrganizationItem> {
  const response = await http.put(`/api/org/organizations/${organizationId}`, payload);
  return response.data?.data as OrganizationItem;
}

export async function deleteOrganization(organizationId: number): Promise<void> {
  await http.delete(`/api/org/organizations/${organizationId}`);
}

export async function listDepartments(params: ListDepartmentQuery): Promise<DepartmentItem[]> {
  const response = await http.get("/api/org/departments", { params });
  return (response.data?.data?.items ?? []) as DepartmentItem[];
}

export async function createDepartment(payload: UpsertDepartmentPayload): Promise<DepartmentItem> {
  const response = await http.post("/api/org/departments", payload);
  return response.data?.data as DepartmentItem;
}

export async function updateDepartment(
  departmentId: number,
  payload: UpsertDepartmentPayload,
): Promise<DepartmentItem> {
  const response = await http.put(`/api/org/departments/${departmentId}`, payload);
  return response.data?.data as DepartmentItem;
}

export async function deleteDepartment(departmentId: number): Promise<void> {
  await http.delete(`/api/org/departments/${departmentId}`);
}

export async function listAssessmentCategories(params?: {
  objectType?: "team" | "individual";
  status?: string;
}): Promise<AssessmentCategoryItem[]> {
  const response = await http.get("/api/org/assessment-categories", { params });
  return (response.data?.data?.items ?? []) as AssessmentCategoryItem[];
}

export async function listPositionLevels(status?: string): Promise<PositionLevelItem[]> {
  const response = await http.get("/api/org/position-levels", {
    params: { status: status || undefined },
  });
  return (response.data?.data?.items ?? []) as PositionLevelItem[];
}

export async function createPositionLevel(payload: UpsertPositionLevelPayload): Promise<PositionLevelItem> {
  const response = await http.post("/api/org/position-levels", payload);
  return response.data?.data as PositionLevelItem;
}

export async function updatePositionLevel(
  positionLevelId: number,
  payload: UpsertPositionLevelPayload,
): Promise<PositionLevelItem> {
  const response = await http.put(`/api/org/position-levels/${positionLevelId}`, payload);
  return response.data?.data as PositionLevelItem;
}

export async function deletePositionLevel(positionLevelId: number): Promise<void> {
  await http.delete(`/api/org/position-levels/${positionLevelId}`);
}

export async function listEmployees(params: ListEmployeeQuery): Promise<EmployeeItem[]> {
  const response = await http.get("/api/org/employees", { params });
  return (response.data?.data?.items ?? []) as EmployeeItem[];
}

export async function createEmployee(payload: UpsertEmployeePayload): Promise<EmployeeItem> {
  const response = await http.post("/api/org/employees", payload);
  return response.data?.data as EmployeeItem;
}

export async function updateEmployee(employeeId: number, payload: UpsertEmployeePayload): Promise<EmployeeItem> {
  const response = await http.put(`/api/org/employees/${employeeId}`, payload);
  return response.data?.data as EmployeeItem;
}

export async function deleteEmployee(employeeId: number): Promise<void> {
  await http.delete(`/api/org/employees/${employeeId}`);
}

export async function transferEmployee(employeeId: number, payload: TransferEmployeePayload): Promise<EmployeeItem> {
  const response = await http.post(`/api/org/employees/${employeeId}/transfer`, payload);
  return response.data?.data as EmployeeItem;
}

export async function listEmployeeHistory(employeeId: number): Promise<EmployeeHistoryItem[]> {
  const response = await http.get(`/api/org/employees/${employeeId}/history`);
  return (response.data?.data?.items ?? []) as EmployeeHistoryItem[];
}
