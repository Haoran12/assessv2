export interface LoginPayload {
  username: string;
  password: string;
}

export interface ChangePasswordPayload {
  oldPassword: string;
  newPassword: string;
}

export interface OrgScope {
  organizationType: string;
  organizationId: number;
  roleInOrg?: string;
  isPrimary: boolean;
}

export interface SessionUser {
  id: number;
  username: string;
  role: string;
  roles: string[];
  permissions: string[];
  organizations: OrgScope[];
}

export interface LoginResponseData {
  token: string;
  tokenType: "Bearer";
  expiresIn: number;
  mustChangePassword: boolean;
  user: SessionUser;
}

export interface ProfileResponseData {
  user: SessionUser;
  mustChangePassword: boolean;
}
