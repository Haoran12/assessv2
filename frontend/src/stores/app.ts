import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { http } from "@/api/http";

interface LoginPayload {
  username: string;
  password: string;
}

export const useAppStore = defineStore("app", () => {
  const token = ref(localStorage.getItem("assessv2_token") || "");
  const username = ref(localStorage.getItem("assessv2_user") || "");

  const isAuthed = computed(() => token.value.length > 0);

  async function login(payload: LoginPayload): Promise<void> {
    const response = await http.post("/api/auth/login", payload);
    token.value = response.data.data.token;
    username.value = response.data.data.user.username;
    localStorage.setItem("assessv2_token", token.value);
    localStorage.setItem("assessv2_user", username.value);
  }

  function logout(): void {
    token.value = "";
    username.value = "";
    localStorage.removeItem("assessv2_token");
    localStorage.removeItem("assessv2_user");
  }

  return {
    token,
    username,
    isAuthed,
    login,
    logout,
  };
});

