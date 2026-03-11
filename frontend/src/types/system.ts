import type { OrgScope } from "@/types/auth";

export type UserStatus = "active" | "inactive" | "locked";

export interface UserListItem {
  id: number;
  username: string;
  realName: string;
  status: UserStatus;
  mustChangePassword: boolean;
  lastLoginAt?: number;
  lastLoginIp?: string;
  roles: string[];
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
