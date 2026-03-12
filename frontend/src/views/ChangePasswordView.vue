<template>
  <div class="page-wrap">
    <el-card class="card">
      <template #header>
        <strong>修改密码</strong>
      </template>

      <el-alert
        v-if="appStore.mustChangePassword"
        type="warning"
        :closable="false"
        title="首次登录请先修改密码"
        style="margin-bottom: 16px"
      />

      <el-form :model="form" label-position="top" @submit.prevent>
        <el-form-item label="当前密码">
          <el-input
            v-model="form.oldPassword"
            type="password"
            show-password
            placeholder="请输入当前密码"
            @keyup.enter="handleChangePassword"
          />
        </el-form-item>

        <el-form-item label="新密码">
          <el-input
            v-model="form.newPassword"
            type="password"
            show-password
            placeholder="至少 8 位"
            @keyup.enter="handleChangePassword"
          />
        </el-form-item>

        <el-form-item label="确认新密码">
          <el-input
            v-model="form.confirmPassword"
            type="password"
            show-password
            placeholder="请再次输入新密码"
            @keyup.enter="handleChangePassword"
          />
        </el-form-item>

        <div class="actions">
          <el-button
            v-if="!appStore.mustChangePassword"
            @click="goBack"
          >
            取消
          </el-button>
          <el-button type="primary" :loading="loading" @click="handleChangePassword">
            确认修改
          </el-button>
        </div>
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
  oldPassword: "",
  newPassword: "",
  confirmPassword: "",
});

async function handleChangePassword(): Promise<void> {
  if (!form.oldPassword || !form.newPassword) {
    ElMessage.warning("请填写完整信息");
    return;
  }
  if (form.newPassword.length < 8) {
    ElMessage.warning("新密码长度不能少于 8 位");
    return;
  }
  if (form.newPassword !== form.confirmPassword) {
    ElMessage.warning("两次输入的新密码不一致");
    return;
  }
  if (form.oldPassword === form.newPassword) {
    ElMessage.warning("新密码不能与当前密码相同");
    return;
  }

  loading.value = true;
  try {
    await appStore.changePassword({
      oldPassword: form.oldPassword,
      newPassword: form.newPassword,
    });
    ElMessage.success("密码修改成功");
    await router.push("/dashboard");
  } catch (_error) {
    ElMessage.error("密码修改失败，请核对当前密码");
  } finally {
    loading.value = false;
  }
}

async function goBack(): Promise<void> {
  await router.push("/dashboard");
}
</script>

<style scoped>
.page-wrap {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 20px;
}

.card {
  width: 460px;
  max-width: 100%;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
