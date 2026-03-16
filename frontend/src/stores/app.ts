import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { http } from "@/api/http";
import type {
  ChangePasswordPayload,
  LoginPayload,
  LoginResponseData,
  OrgScope,
  ProfileResponseData,
  SessionUser,
} from "@/types/auth";

const TOKEN_KEY = "assessv2_token";
const USER_KEY = "assessv2_user";
const MUST_CHANGE_KEY = "assessv2_must_change_password";

function readStoredUser(): SessionUser | null {
  const text = sessionStorage.getItem(USER_KEY);
  if (!text) {
    return null;
  }
  try {
    return JSON.parse(text) as SessionUser;
  } catch (_error) {
    return null;
  }
}

export const useAppStore = defineStore("app", () => {
  const token = ref(sessionStorage.getItem(TOKEN_KEY) || "");
  const currentUser = ref<SessionUser | null>(readStoredUser());
  const mustChangePassword = ref(sessionStorage.getItem(MUST_CHANGE_KEY) === "true");
  const initialized = ref(false);

  const isAuthed = computed(() => token.value.length > 0);
  const username = computed(() => currentUser.value?.username ?? "");
  const displayName = computed(() => currentUser.value?.realName || currentUser.value?.username || "");
  const roles = computed(() => currentUser.value?.roles ?? []);
  const permissions = computed(() => currentUser.value?.permissions ?? []);
  const orgScopes = computed<OrgScope[]>(() => currentUser.value?.organizations ?? []);
  const primaryRole = computed(() => currentUser.value?.role ?? "");

  function hasPermission(requiredPermission: string): boolean {
    if (!requiredPermission) {
      return true;
    }
    const granted = permissions.value;
    for (const permission of granted) {
      if (permission === "*" || permission === requiredPermission) {
        return true;
      }
      if (permission.endsWith("*")) {
        const prefix = permission.slice(0, -1);
        if (requiredPermission.startsWith(prefix)) {
          return true;
        }
      }
    }
    return false;
  }

  function hasAnyPermission(requiredPermissions: string[]): boolean {
    if (requiredPermissions.length === 0) {
      return true;
    }
    return requiredPermissions.some((item) => hasPermission(item));
  }

  function persistSession(newToken: string, user: SessionUser, mustChange: boolean): void {
    token.value = newToken;
    currentUser.value = user;
    mustChangePassword.value = mustChange;
    sessionStorage.setItem(TOKEN_KEY, newToken);
    sessionStorage.setItem(USER_KEY, JSON.stringify(user));
    sessionStorage.setItem(MUST_CHANGE_KEY, mustChange ? "true" : "false");
  }

  function clearSession(): void {
    token.value = "";
    currentUser.value = null;
    mustChangePassword.value = false;
    initialized.value = true;
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(USER_KEY);
    sessionStorage.removeItem(MUST_CHANGE_KEY);
  }

  async function login(payload: LoginPayload): Promise<LoginResponseData> {
    const response = await http.post("/api/auth/login", payload);
    const data = response.data.data as LoginResponseData;
    persistSession(data.token, data.user, data.mustChangePassword);
    initialized.value = true;
    return data;
  }

  async function fetchProfile(): Promise<ProfileResponseData> {
    const response = await http.get("/api/system/profile");
    const data = response.data.data as ProfileResponseData;
    if (!token.value) {
      throw new Error("token is empty");
    }
    persistSession(token.value, data.user, data.mustChangePassword);
    initialized.value = true;
    return data;
  }

  async function initializeSession(): Promise<void> {
    if (!token.value) {
      initialized.value = true;
      return;
    }
    try {
      await fetchProfile();
    } catch (_error) {
      clearSession();
      throw _error;
    }
  }

  async function changePassword(payload: ChangePasswordPayload): Promise<void> {
    await http.post("/api/auth/change-password", payload);
    mustChangePassword.value = false;
    sessionStorage.setItem(MUST_CHANGE_KEY, "false");
    await fetchProfile();
  }

  async function logout(callAPI = true): Promise<void> {
    if (callAPI && token.value) {
      try {
        await http.post("/api/auth/logout");
      } catch (_error) {
        // Ignore logout API failure and clear local session anyway.
      }
    }
    clearSession();
  }

  return {
    token,
    currentUser,
    mustChangePassword,
    initialized,
    isAuthed,
    username,
    displayName,
    roles,
    permissions,
    orgScopes,
    primaryRole,
    login,
    fetchProfile,
    initializeSession,
    changePassword,
    logout,
    clearSession,
    hasPermission,
    hasAnyPermission,
  };
});
