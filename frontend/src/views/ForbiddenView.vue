<template>
  <div class="forbidden-page">
    <el-result
      icon="warning"
      title="403 无权限"
      sub-title="您没有访问该页面的权限"
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
