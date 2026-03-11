<template>
  <div class="dashboard">
    <h2>项目基础框架已就绪</h2>
    <p>当前版本完成了文档 00 对应的 M2 基础框架搭建，可按模块继续开发业务能力。</p>

    <el-row :gutter="16">
      <el-col v-for="item in cards" :key="item.name" :span="8">
        <el-card class="dashboard-card">
          <template #header>
            <strong>{{ item.name }}</strong>
          </template>
          <p>{{ item.description }}</p>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
const cards = [
  { name: "认证与权限", description: "已提供登录接口和 JWT 中间件骨架。" },
  { name: "模块路由", description: "已创建 org/assessment/rules/scores 等 10 组 API 占位。" },
  { name: "数据层", description: "已接入 SQLite + GORM，并预留 migration 目录。" },
  { name: "前端框架", description: "已接入 Vue3 + TS + Pinia + Element Plus。" },
  { name: "页面骨架", description: "已提供登录页、主布局和模块占位页。" },
  { name: "Tauri 容器", description: "已创建 src-tauri 最小可编译工程。" },
];
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dashboard-card {
  min-height: 150px;
}
</style>

