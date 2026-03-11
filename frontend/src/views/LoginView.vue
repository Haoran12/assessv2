<template>
  <div class="login-page">
    <el-card class="login-card">
      <template #header>
        <strong>AssessV2 登录</strong>
      </template>
      <el-alert
        title="初始化账号：root / #2026@hdwl"
        type="info"
        :closable="false"
        style="margin-bottom: 16px"
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
          />
        </el-form-item>
        <el-button type="primary" :loading="loading" @click="handleLogin">
          登录
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { useAppStore } from "@/stores/app";

const router = useRouter();
const appStore = useAppStore();

const loading = ref(false);
const form = reactive({
  username: "root",
  password: "#2026@hdwl",
});

async function handleLogin(): Promise<void> {
  loading.value = true;
  try {
    await appStore.login(form);
    ElMessage.success("登录成功");
    await router.push("/dashboard");
  } catch (_error) {
    ElMessage.error("登录失败，请检查用户名和密码");
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
  background: linear-gradient(160deg, #f5f7fa, #e4ecff);
}

.login-card {
  width: 360px;
}
</style>
