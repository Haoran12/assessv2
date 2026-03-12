<template>
  <el-container class="app-shell">
    <el-aside width="250px" class="app-sidebar">
      <div class="brand">
        <div class="brand-title">AssessV2</div>
        <div class="brand-subtitle">M1-M3 前端交互版</div>
      </div>
      <el-menu :default-active="activePath" router>
        <el-menu-item
          v-for="item in visibleMenus"
          :key="item.path"
          :index="item.path"
        >
          {{ item.label }}
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="app-header">
        <div class="header-left">
          <strong>集团考核管理系统</strong>
        </div>
        <div class="header-right">
          <div v-if="showGlobalContext" class="global-context">
            <el-select
              v-model="contextYearId"
              class="context-select"
              placeholder="年度"
              :loading="contextStore.loadingYears"
              clearable
            >
              <el-option
                v-for="item in contextStore.years"
                :key="item.id"
                :label="`${item.year} - ${item.yearName}`"
                :value="item.id"
              />
            </el-select>
            <el-select v-model="contextPeriodCode" class="context-select" placeholder="周期">
              <el-option v-for="item in periodOptions" :key="item" :label="item" :value="item" />
            </el-select>
          </div>
          <el-dropdown trigger="click">
            <span class="username-trigger" :class="{ 'is-root': appStore.primaryRole === 'root' }">
              {{ appStore.username || "未登录" }}
              <el-icon class="el-icon--right"><arrow-down /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item disabled>
                  <span class="role-tag">{{ roleLabel(appStore.primaryRole) }}</span>
                </el-dropdown-item>
                <el-dropdown-item divided @click="goToChangePassword">
                  修改密码
                </el-dropdown-item>
                <el-dropdown-item @click="handleLogout">
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-main class="app-main">
        <RouterView />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { ArrowDown } from "@element-plus/icons-vue";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import { PERIOD_OPTIONS } from "@/utils/assessment";

interface NavItem {
  path: string;
  label: string;
  permission?: string;
}

const navItems: NavItem[] = [
  { path: "/dashboard", label: "首页" },
  { path: "/org", label: "组织架构", permission: "org:view" },
  { path: "/assessment", label: "年度周期", permission: "assessment:view" },
  { path: "/rules", label: "规则配置", permission: "rule:view" },
  { path: "/scores/direct", label: "直接录入", permission: "score:view" },
  { path: "/scores/extra", label: "加减分", permission: "score:view" },
  { path: "/votes/task", label: "投票任务", permission: "score:view" },
  { path: "/votes/execute", label: "执行投票", permission: "score:view" },
  { path: "/votes/statistics", label: "投票统计", permission: "score:view" },
  { path: "/calc", label: "计算引擎", permission: "score:*" },
  { path: "/reports", label: "报表中心", permission: "report:view" },
  { path: "/backup", label: "备份审计", permission: "backup:*" },
  { path: "/system/users", label: "用户管理", permission: "user:view" },
];

const route = useRoute();
const router = useRouter();
const appStore = useAppStore();
const contextStore = useContextStore();
const periodOptions = PERIOD_OPTIONS;

const activePath = computed(() => route.path);
const visibleMenus = computed(() =>
  navItems.filter((item) => !item.permission || appStore.hasPermission(item.permission)),
);
const showGlobalContext = computed(() => route.matched.some((record) => record.meta.useGlobalContext));
const contextYearId = computed({
  get: () => contextStore.yearId,
  set: (value) => contextStore.setYear(value),
});
const contextPeriodCode = computed({
  get: () => contextStore.periodCode,
  set: (value) => contextStore.setPeriodCode(value),
});

watch(
  () => [showGlobalContext.value, appStore.isAuthed],
  async ([visible, authed]) => {
    if (!visible || !authed) {
      return;
    }
    try {
      await contextStore.ensureInitialized();
    } catch (_error) {
      ElMessage.error("全局上下文加载失败");
    }
  },
  { immediate: true },
);

async function handleLogout(): Promise<void> {
  await appStore.logout();
  ElMessage.success("已退出登录");
  await router.push("/login");
}

async function goToChangePassword(): Promise<void> {
  await router.push("/change-password");
}

function roleLabel(roleCode: string): string {
  switch (roleCode) {
    case "root":
      return "Root 管理员";
    case "viewer":
      return "查看者";
    case "":
      return "未分配角色";
    default:
      return roleCode;
  }
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
  padding: 18px 16px;
  border-bottom: 1px solid #ebeef5;
}

.brand-title {
  font-weight: 700;
  letter-spacing: 0.3px;
}

.brand-subtitle {
  margin-top: 4px;
  color: #6b7280;
  font-size: 12px;
}

.app-header {
  border-bottom: 1px solid #ebeef5;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #fff;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.global-context {
  display: flex;
  align-items: center;
  gap: 8px;
}

.context-select {
  width: 180px;
}

.username-trigger {
  display: flex;
  align-items: center;
  cursor: pointer;
  padding: 8px 12px;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.username-trigger:hover {
  background-color: #f5f7fa;
}

.username-trigger.is-root {
  color: #f56c6c;
  font-weight: 600;
}

.username-trigger.is-root:hover {
  background-color: #fef0f0;
}

.role-tag {
  padding: 2px 8px;
  border-radius: 999px;
  background: #eef2ff;
  color: #4338ca;
  font-size: 12px;
}

.app-main {
  background: #f5f7fa;
}

@media (max-width: 1100px) {
  .header-right {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .context-select {
    width: 150px;
  }
}
</style>
