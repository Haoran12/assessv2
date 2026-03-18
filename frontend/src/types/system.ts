import type { OrgScope } from "@/types/auth";

export type UserStatus = "active" | "inactive" | "locked";

export interface UserListItem {
  id: number;
  username: string;
  status: UserStatus;
  mustChangePassword: boolean;
  lastLoginAt?: number;
  lastLoginIp?: string;
  roles: string[];
  roleNames: string[];
  primaryRole: string;
  organizations: OrgScope[];
  createdAt: number;
  updatedAt: number;
}

export interface UserListResponse {
  items: UserListItem[];
  total: number;
  page: number;
  pageSize: number;
}

export interface UserListQuery {
  page: number;
  pageSize: number;
  keyword?: string;
  status?: UserStatus | "";
}

export interface UserGroupItem {
  id: number;
  roleCode: string;
  roleName: string;
  description: string;
  isSystem: boolean;
  createdAt: number;
  updatedAt: number;
}

export interface UserGroupListResponse {
  items: UserGroupItem[];
}

export type UserObjectLinkAccessLevel = "read" | "detail";

export interface UserObjectLinkItem {
  id: number;
  userId: number;
  assessmentObjectId: number;
  assessmentObjectName: string;
  assessmentObjectYear: number;
  assessmentObjectType: string;
  assessmentObjectCategory: string;
  assessmentObjectActive: boolean;
  linkType: string;
  accessLevel: UserObjectLinkAccessLevel;
  isPrimary: boolean;
  effectiveFrom?: number;
  effectiveTo?: number;
  isActive: boolean;
  createdBy?: number;
  createdAt: number;
  updatedBy?: number;
  updatedAt: number;
}

export interface ReplaceUserObjectLinkItem {
  assessmentObjectId: number;
  linkType: string;
  accessLevel: UserObjectLinkAccessLevel;
  isPrimary: boolean;
  effectiveFrom?: number;
  effectiveTo?: number;
  isActive?: boolean;
}

export type BackupType = "manual" | "auto" | "before_import" | "before_restore";

export interface BackupRecordItem {
  id: number;
  backupName: string;
  backupType: BackupType;
  fileSize: number;
  description: string;
  createdBy?: number;
  createdAt: number;
}

export interface BackupListResponse {
  items: BackupRecordItem[];
  total: number;
  page: number;
  pageSize: number;
}

export interface AuditDiffItem {
  field: string;
  before: unknown;
  after: unknown;
}

export interface AuditLogItem {
  id: number;
  userId?: number;
  username: string;
  realName: string;
  actionType: string;
  targetType: string;
  targetId?: number;
  actionDetail: string;
  ipAddress: string;
  userAgent: string;
  createdAt: number;
}

export interface AuditLogListResponse {
  items: AuditLogItem[];
  total: number;
  page: number;
  pageSize: number;
}

export interface AuditLogDetail extends AuditLogItem {
  detail: Record<string, unknown>;
  diffs: AuditDiffItem[];
  canRollback: boolean;
}

export interface SystemSettingItem {
  id: number;
  settingKey: string;
  settingValue: string;
  settingType: "string" | "number" | "boolean" | "json";
  value: unknown;
  description: string;
  isSystem: boolean;
  updatedBy?: number;
  updatedAt: number;
}

export interface SystemSettingsResponse {
  items: SystemSettingItem[];
  basic: Record<string, unknown>;
  assessment: Record<string, unknown>;
  security: Record<string, unknown>;
  backup: Record<string, unknown>;
  other: Record<string, unknown>;
}
