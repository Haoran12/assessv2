import axios, { type AxiosError } from "axios";
import type { ApiResponse } from "@/types/api";

const baseURL = import.meta.env.VITE_API_BASE_URL || "";

export const http = axios.create({
  baseURL,
  timeout: 10000,
});

http.interceptors.request.use((config) => {
  const token = localStorage.getItem("assessv2_token");
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
    return Promise.reject(error);
  },
);

