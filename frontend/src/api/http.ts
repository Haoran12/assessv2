import axios, { type AxiosError } from "axios";
import type { ApiResponse } from "@/types/api";

const isWailsRuntime = typeof navigator !== "undefined" && navigator.userAgent.toLowerCase().includes("wails");
const baseURL = isWailsRuntime ? "" : import.meta.env.VITE_API_BASE_URL || "";
const TOKEN_KEY = "assessv2_token";
const USER_KEY = "assessv2_user";
const MUST_CHANGE_KEY = "assessv2_must_change_password";
const SESSION_EXPIRED_KEY = "assessv2_session_expired";

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
