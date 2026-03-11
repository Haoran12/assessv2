<template>
  <el-container class="app-shell">
    <el-aside width="240px" class="app-sidebar">
      <div class="brand">AssessV2</div>
      <el-menu :default-active="activePath" router>
        <el-menu-item index="/dashboard">首页</el-menu-item>
        <el-menu-item index="/org">组织架构</el-menu-item>
        <el-menu-item index="/assessment">考核管理</el-menu-item>
        <el-menu-item index="/rules">规则配置</el-menu-item>
        <el-menu-item index="/scores">分数管理</el-menu-item>
        <el-menu-item index="/votes">投票管理</el-menu-item>
        <el-menu-item index="/calc">计算引擎</el-menu-item>
        <el-menu-item index="/reports">报表中心</el-menu-item>
        <el-menu-item index="/backup">备份审计</el-menu-item>
        <el-menu-item index="/system">系统管理</el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="app-header">
        <span>集团企业考核系统</span>
        <div class="header-actions">
          <span>{{ appStore.username || "未登录" }}</span>
          <el-button type="danger" link @click="handleLogout">退出</el-button>
        </div>
      </el-header>
      <el-main class="app-main">
        <RouterView />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAppStore } from "@/stores/app";

const route = useRoute();
const router = useRouter();
const appStore = useAppStore();

const activePath = computed(() => route.path);

function handleLogout(): void {
  appStore.logout();
  router.push("/login");
}
</script>

<style scoped>
.app-shell {
  min-height: 100vh;
}

.app-sidebar {
  border-right: 1px solid #e4e7ed;
  background: #fff;
}

.brand {
  padding: 16px;
  font-weight: 600;
  border-bottom: 1px solid #ebeef5;
}

.app-header {
  border-bottom: 1px solid #ebeef5;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #fff;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.app-main {
  background: #f5f7fa;
}
</style>

