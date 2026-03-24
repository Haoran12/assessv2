<template>
  <div class="forbidden-page">
    <el-result
      icon="warning"
      title="Request failed with Code 403: Permission Denied"
      sub-title="当前用户没有权限访问该页面，请联系管理员分配角色或授权。"
    >
      <template #extra>
        <el-button type="primary" @click="goHome">返回首页</el-button>
      </template>
    </el-result>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";

const router = useRouter();

async function goHome(): Promise<void> {
  await router.push("/dashboard");
}
</script>

<style scoped>
.forbidden-page {
  min-height: calc(100vh - 120px);
  display: grid;
  place-items: center;
}
</style>
