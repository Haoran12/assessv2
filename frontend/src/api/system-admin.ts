import { http } from "@/api/http";
import type {
  AuditLogDetail,
  AuditLogListResponse,
  BackupListResponse,
  BackupRecordItem,
  OrgPackageItem,
  OrgPackageListResponse,
  BackupType,
  SystemSettingsResponse,
} from "@/types/system";

interface BackupListQuery {
  page: number;
  pageSize: number;
  type?: BackupType | "";
}

interface OrgPackageListQuery {
  page: number;
  pageSize: number;
  rootOrganizationId?: number;
}

interface AuditLogListQuery {
  page: number;
  pageSize: number;
  userId?: number;
  actionType?: string;
  targetType?: string;
  keyword?: string;
  startAt?: number;
  endAt?: number;
}

export async function listBackups(params: BackupListQuery): Promise<BackupListResponse> {
  const response = await http.get("/api/backup/records", { params });
  return response.data?.data as BackupListResponse;
}

export async function createManualBackup(description?: string): Promise<BackupRecordItem> {
  const response = await http.post("/api/backup/records", {
    description: description || "",
  });
  return response.data?.data as BackupRecordItem;
}

export async function downloadBackupFile(backupId: number): Promise<Blob> {
  const response = await http.get(`/api/backup/records/${backupId}/download`, {
    responseType: "blob",
  });
  return response.data as Blob;
}

export async function deleteBackup(backupId: number): Promise<void> {
  await http.delete(`/api/backup/records/${backupId}`);
}

export async function restoreBackup(backupId: number, confirmText: string): Promise<void> {
  await http.post(`/api/backup/records/${backupId}/restore`, { confirmText });
}

export async function listOrgPackages(params: OrgPackageListQuery): Promise<OrgPackageListResponse> {
  const response = await http.get("/api/backup/org-packages", { params });
  return response.data?.data as OrgPackageListResponse;
}

export async function createOrgPackage(payload: {
  rootOrganizationId: number;
  description?: string;
  includeEmployeeHistory?: boolean;
}): Promise<OrgPackageItem> {
  const response = await http.post("/api/backup/org-packages", {
    rootOrganizationId: payload.rootOrganizationId,
    description: payload.description || "",
    includeEmployeeHistory: payload.includeEmployeeHistory ?? true,
  });
  return response.data?.data as OrgPackageItem;
}

export async function downloadOrgPackageFile(backupId: number): Promise<Blob> {
  const response = await http.get(`/api/backup/org-packages/${backupId}/download`, {
    responseType: "blob",
  });
  return response.data as Blob;
}

export async function restoreOrgPackage(
  backupId: number,
  payload: { confirmText: string; mode: "replace_scope"; targetRootOrganizationId: number },
): Promise<void> {
  await http.post(`/api/backup/org-packages/${backupId}/restore`, payload);
}

export async function listAuditLogs(params: AuditLogListQuery): Promise<AuditLogListResponse> {
  const response = await http.get("/api/system/audit-logs", { params });
  return response.data?.data as AuditLogListResponse;
}

export async function getAuditLogDetail(auditId: number): Promise<AuditLogDetail> {
  const response = await http.get(`/api/system/audit-logs/${auditId}`);
  return response.data?.data as AuditLogDetail;
}

export async function rollbackAuditLog(auditId: number): Promise<void> {
  await http.post(`/api/system/audit-logs/${auditId}/rollback`, {});
}

export async function getSystemSettings(): Promise<SystemSettingsResponse> {
  const response = await http.get("/api/system/settings");
  return response.data?.data as SystemSettingsResponse;
}

export async function updateSystemSettings(items: Array<{ settingKey: string; settingValue: unknown }>): Promise<SystemSettingsResponse> {
  const response = await http.put("/api/system/settings", { items });
  return response.data?.data as SystemSettingsResponse;
}
