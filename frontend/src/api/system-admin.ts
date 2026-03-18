import { http } from "@/api/http";
import type {
  AuditLogDetail,
  AuditLogListResponse,
  BackupListResponse,
  BackupRecordItem,
  BackupType,
  SystemSettingsResponse,
} from "@/types/system";

interface BackupListQuery {
  page: number;
  pageSize: number;
  type?: BackupType | "";
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
