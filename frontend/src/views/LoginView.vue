<template>
  <div class="login-page">
    <el-card class="login-card">
      <template #header>
        <strong>AssessV2 登录</strong>
      </template>

      <el-alert
        title="默认账号：root / #2026@hdwl"
        type="info"
        :closable="false"
        style="margin-bottom: 12px"
      />

      <el-alert
        v-if="sessionExpired"
        title="登录已过期，请重新登录"
        type="warning"
        :closable="false"
        style="margin-bottom: 12px"
      />

      <el-form :model="form" label-position="top" @submit.prevent>
        <el-form-item label="用户名">
          <el-input v-model="form.username" placeholder="请输入用户名" />
        </el-form-item>

        <el-form-item label="密码">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="请输入密码"
            show-password
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-button type="primary" :loading="loading" style="width: 100%" @click="handleLogin">
          登录
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import type { AxiosError } from "axios";
import { useAppStore } from "@/stores/app";

const SESSION_EXPIRED_KEY = "assessv2_session_expired";

const route = useRoute();
const router = useRouter();
const appStore = useAppStore();

const loading = ref(false);
const sessionExpired = ref(sessionStorage.getItem(SESSION_EXPIRED_KEY) === "1");
const form = reactive({
  username: "root",
  password: "#2026@hdwl",
});

if (sessionExpired.value) {
  sessionStorage.removeItem(SESSION_EXPIRED_KEY);
}

async function handleLogin(): Promise<void> {
  loading.value = true;
  try {
    const result = await appStore.login(form);
    ElMessage.success("登录成功");

    if (result.mustChangePassword) {
      await router.push("/change-password");
      return;
    }

    const redirectRaw = typeof route.query.redirect === "string" ? route.query.redirect : "";
    const redirect = redirectRaw.startsWith("/") ? redirectRaw : "/dashboard";
    await router.push(redirect || "/dashboard");
  } catch (error) {
    const err = error as AxiosError<{ message?: string }>;
    const message = err.response?.data?.message;
    if (!err.response) {
      ElMessage.error("登录失败：后端服务不可用");
    } else {
      ElMessage.error(message || "登录失败，请检查用户名或密码");
    }
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  background: linear-gradient(160deg, #f6f8fc, #dce8ff);
}

.login-card {
  width: 380px;
}
</style>
