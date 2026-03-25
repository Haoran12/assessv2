import axios, { type AxiosError } from "axios";
import type { ApiResponse } from "@/types/api";

const isWailsRuntime = typeof navigator !== "undefined" && navigator.userAgent.toLowerCase().includes("wails");
const baseURL = isWailsRuntime ? "" : import.meta.env.VITE_API_BASE_URL || "";
const TOKEN_KEY = "assessv2_token";
const USER_KEY = "assessv2_user";
const MUST_CHANGE_KEY = "assessv2_must_change_password";
const SESSION_EXPIRED_KEY = "assessv2_session_expired";

interface ApiErrorPayload {
  code?: unknown;
  message?: unknown;
}

function toPositiveInt(value: unknown): number | undefined {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return undefined;
  }
  return Math.floor(parsed);
}

function extractCodeFromText(message: string): number | undefined {
  const text = message.trim();
  if (!text) {
    return undefined;
  }
  const matched = text.match(/request failed(?: with code)?\s*(\d+)/i)
    || text.match(/status code\s*(\d+)/i);
  if (!matched?.[1]) {
    return undefined;
  }
  return toPositiveInt(matched[1]);
}

function stripFailurePrefix(message: string): string {
  return message
    .trim()
    .replace(/^request failed with code\s*\d+\s*:?\s*/i, "")
    .replace(/^request failed with status code\s*\d+\s*:?\s*/i, "")
    .replace(/^request failed\s*\d+\s*:?\s*/i, "")
    .replace(/^permission denied[.:]?\s*/i, "")
    .trim();
}

function fallbackReasonByStatus(status?: number): string {
  switch (status) {
    case 400:
      return "请求参数不正确，请检查后重试";
    case 401:
      return "登录状态已失效，请重新登录";
    case 403:
      return "当前账号没有权限执行该操作，请联系管理员授权";
    case 404:
      return "请求的资源不存在或已被删除";
    case 409:
      return "数据状态已变化，请刷新后重试";
    case 422:
      return "提交内容校验失败，请检查后重试";
    case 500:
    case 502:
    case 503:
    case 504:
      return "服务暂时不可用，请稍后重试";
    default:
      return "请求处理失败，请稍后重试";
  }
}

function normalizeReason(message: string, status?: number): string {
  const stripped = stripFailurePrefix(message);
  const normalized = stripped.replace(/\s+/g, " ").trim();
  const lower = normalized.toLowerCase();

  const missingPermissionMatched = normalized.match(/缺少权限[:：]\s*([^\s。]+)/);
  if (missingPermissionMatched?.[1]) {
    return `当前账号缺少「${missingPermissionMatched[1]}」权限，请联系管理员授权`;
  }

  if (normalized.includes("需要 root 角色") || lower.includes("root role")) {
    return "当前操作仅允许 root 角色执行，请联系管理员处理";
  }

  if (lower.includes("permission denied") || normalized.includes("没有权限")) {
    return "当前账号没有权限执行该操作，请联系管理员授权";
  }

  if (
    lower.includes("missing auth context")
    || lower.includes("missing authorization header")
    || lower.includes("invalid authorization header format")
    || lower.includes("invalid token")
  ) {
    return "登录状态已失效，请重新登录";
  }

  if (lower.includes("invalid username or password")) {
    return "用户名或密码不正确";
  }

  if (lower.includes("account is inactive")) {
    return "账号已停用，请联系管理员";
  }

  if (lower.includes("account is locked")) {
    return "账号已锁定，请联系管理员";
  }

  if (lower.includes("oldpassword is incorrect") || lower.includes("invalid password")) {
    return "当前密码不正确，请重新输入";
  }

  if (lower.includes("username already exists")) {
    return "用户名已存在，请更换后重试";
  }

  if (lower.includes("not found")) {
    return "请求的资源不存在或已被删除";
  }

  if (lower.includes("timeout")) {
    return "请求超时，请检查网络后重试";
  }

  if (lower.includes("network error")) {
    return "网络连接失败，请检查网络后重试";
  }

  if (lower.includes("invalid") && (lower.includes("payload") || lower.includes("param") || lower.includes("id"))) {
    return "请求参数或数据格式不正确，请检查后重试";
  }

  if (lower.includes("failed to ")) {
    return "服务处理失败，请稍后重试";
  }

  if (normalized) {
    return normalized;
  }

  return fallbackReasonByStatus(status);
}

function formatRequestFailedMessage(error: AxiosError<ApiResponse>): string {
  const status = error.response?.status;
  const payload = error.response?.data as ApiErrorPayload | undefined;
  const payloadMessage = typeof payload?.message === "string" ? payload.message : "";
  const payloadCode = toPositiveInt(payload?.code);
  const fallbackMessage = typeof error.message === "string" ? error.message : "";
  const mergedMessage = payloadMessage.trim() || fallbackMessage.trim();

  const code =
    payloadCode
    ?? extractCodeFromText(payloadMessage)
    ?? toPositiveInt(status)
    ?? extractCodeFromText(fallbackMessage);

  const reason = normalizeReason(mergedMessage, status);
  return `Request Failed ${code ?? "NETWORK"}: ${reason}`;
}

export const http = axios.create({
  baseURL,
  timeout: 10000,
});

http.interceptors.request.use((config) => {
  const token = sessionStorage.getItem("assessv2_token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

http.interceptors.response.use(
  (response) => {
    return response;
  },
  (error: AxiosError<ApiResponse>) => {
    const formattedMessage = formatRequestFailedMessage(error);
    error.message = formattedMessage;
    const payload = error.response?.data as ApiErrorPayload | undefined;
    if (payload && typeof payload === "object") {
      payload.message = formattedMessage;
    }

    const status = error.response?.status;
    const requestURL = error.config?.url ?? "";
    const isLoginRequest = requestURL.includes("/api/auth/login");
    if (status === 401 && !isLoginRequest) {
      sessionStorage.removeItem(TOKEN_KEY);
      sessionStorage.removeItem(USER_KEY);
      sessionStorage.removeItem(MUST_CHANGE_KEY);
      sessionStorage.setItem(SESSION_EXPIRED_KEY, "1");
      if (window.location.pathname !== "/login") {
        const redirectPath = encodeURIComponent(window.location.pathname + window.location.search);
        window.location.href = `/login?redirect=${redirectPath}`;
      }
    }
    return Promise.reject(error);
  },
);
