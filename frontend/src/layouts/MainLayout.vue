<template>
  <el-container class="app-shell">
    <el-aside width="220px" class="app-sidebar">
      <div class="brand">
        <div class="brand-title">{{ appBrandName }}</div>
      </div>
      <el-menu :default-active="activePath" router>
        <el-menu-item v-for="item in visibleMenus" :key="item.path" :index="item.path">
          {{ item.label }}
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container class="app-content-shell">
      <el-header :class="['app-header', { 'app-header-root': isRootUser }]">
        <div class="header-left">
          <div v-if="appStore.isAuthed" class="global-context">
            <el-select
              v-model="contextSessionId"
              class="context-select context-session"
              placeholder="考核场次"
              :loading="contextStore.loadingSessions"
            >
              <el-option
                v-for="item in contextStore.sessions"
                :key="item.id"
                :label="`${item.displayName} (${item.organizationName})`"
                :value="item.id"
              />
            </el-select>
            <el-select
              v-model="contextPeriodCode"
              class="context-select"
              placeholder="周期"
              :loading="contextStore.loadingDetail"
              :disabled="!contextSessionId"
            >
              <el-option
                v-for="item in contextStore.periods"
                :key="item.id"
                :label="item.periodName"
                :value="item.periodCode"
              />
            </el-select>
            <el-select
              v-model="contextObjectGroupCode"
              class="context-select"
              placeholder="考核对象类型"
              :loading="contextStore.loadingDetail"
              :disabled="!contextSessionId"
            >
              <el-option
                v-for="item in contextStore.objectGroupOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </div>
        </div>
        <div class="header-right">
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
                <el-dropdown-item divided @click="goToChangePassword">修改密码</el-dropdown-item>
                <el-dropdown-item @click="handleLogout">退出登录</el-dropdown-item>
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
import { computed, onBeforeUnmount, onMounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { ArrowDown } from "@element-plus/icons-vue";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import { appBrandName } from "@/config/branding";
import { resolveUnsavedBeforeLeave } from "@/guards/unsaved";

interface NavItem {
  path: string;
  label: string;
  permission?: string | string[];
  rootOnly?: boolean;
}

const navItems: NavItem[] = [
  { path: "/overview", label: "考核主页" },
  { path: "/assessment-management", label: "考核管理", permission: "assessment:view" },
  { path: "/org", label: "组织架构", permission: "org:view" },
  { path: "/system/users", label: "用户管理", permission: "user:view", rootOnly: true },
  { path: "/system/backup", label: "备份恢复", permission: ["backup:view", "backup:org:view"] },
  { path: "/system/audit", label: "审计日志", permission: "audit:view" },
  { path: "/system/settings", label: "系统设置", permission: "setting:view", rootOnly: true },
];

const route = useRoute();
const router = useRouter();
const appStore = useAppStore();
const contextStore = useContextStore();

const activePath = computed(() => route.path);
const isRootUser = computed(() => appStore.primaryRole === "root" || appStore.roles.includes("root"));
const visibleMenus = computed(() =>
  navItems.filter((item) => {
    if (item.rootOnly && appStore.primaryRole !== "root" && !appStore.roles.includes("root")) {
      return false;
    }
    if (!item.permission) {
      return true;
    }
    const required = Array.isArray(item.permission) ? item.permission : [item.permission];
    return appStore.hasAnyPermission(required);
  }),
);

const contextSessionId = computed({
  get: () => contextStore.sessionId,
  set: (value) => {
    contextStore.setSession(value).catch(() => {
      ElMessage.error("切换考核场次失败");
    });
  },
});

const contextPeriodCode = computed({
  get: () => contextStore.periodCode,
  set: (value: string) => contextStore.setPeriodCode(value),
});

const contextObjectGroupCode = computed({
  get: () => contextStore.objectGroupCode,
  set: (value: string) => contextStore.setObjectGroupCode(value),
});

function isSystemWindowActive(): boolean {
  return document.visibilityState === "visible" && document.hasFocus();
}

function switchObjectGroupByDirection(direction: 1 | -1): void {
  const options = contextStore.objectGroupOptions;
  if (!contextStore.sessionId || options.length <= 1 || contextStore.loadingDetail) {
    return;
  }
  const currentValue = String(contextStore.objectGroupCode || "").trim();
  const currentIndex = options.findIndex((item) => item.value === currentValue);
  const baseIndex = currentIndex >= 0 ? currentIndex : 0;
  let nextIndex = baseIndex + direction;
  if (nextIndex < 0) {
    nextIndex = options.length - 1;
  } else if (nextIndex >= options.length) {
    nextIndex = 0;
  }
  const nextValue = options[nextIndex]?.value;
  if (!nextValue || nextValue === currentValue) {
    return;
  }
  contextStore.setObjectGroupCode(nextValue);
}

function handleGlobalObjectGroupWheel(event: WheelEvent): void {
  if (!event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) {
    return;
  }
  if (!isSystemWindowActive()) {
    return;
  }
  if (Math.abs(event.deltaY) < Number.EPSILON) {
    return;
  }
  event.preventDefault();
  switchObjectGroupByDirection(event.deltaY > 0 ? 1 : -1);
}

watch(
  () => appStore.isAuthed,
  async (authed) => {
    if (!authed) {
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

onMounted(() => {
  window.addEventListener("wheel", handleGlobalObjectGroupWheel, { passive: false });
});

onBeforeUnmount(() => {
  window.removeEventListener("wheel", handleGlobalObjectGroupWheel);
});

async function handleLogout(): Promise<void> {
  const canLeave = await resolveUnsavedBeforeLeave();
  if (!canLeave) {
    return;
  }
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
      return "Root";
    case "assessment_admin":
      return "Admin";
    case "":
      return "未分配角色";
    default:
      return roleCode;
  }
}
</script>

<style scoped>
.app-shell {
  height: 100vh;
  overflow: hidden;
}

.app-sidebar {
  height: 100vh;
  overflow-y: auto;
  overflow-x: hidden;
  flex-shrink: 0;
  border-right: 1px solid #e4e7ed;
  background: #fff;
}

.app-content-shell {
  min-width: 0;
  min-height: 0;
  height: 100vh;
  overflow: hidden;
}

.brand {
  padding: 18px 16px;
  border-bottom: 1px solid #ebeef5;
}

.brand-title {
  font-weight: 700;
  letter-spacing: 0.3px;
}

.app-header {
  border-bottom: 1px solid #ebeef5;
  display: flex;
  align-items: center;
  gap: 12px;
  background: #fff;
}

.app-header.app-header-root {
  background: #fff1f0;
  border-bottom-color: #f5c2c7;
}

.header-left {
  min-width: 0;
  display: flex;
  align-items: center;
  flex: 1;
}

.header-right {
  margin-left: auto;
  display: flex;
  align-items: center;
}

.global-context {
  display: flex;
  align-items: center;
  gap: 8px;
}

.context-select {
  width: 160px;
}

.context-session {
  width: 250px;
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
  min-height: 0;
  overflow: auto;
  background: #f5f7fa;
  padding: 12px;
}

.app-main :deep(.el-card__body) {
  padding: 14px;
}

@media (max-width: 1280px) {
  .header-left {
    justify-content: flex-end;
  }

  .global-context {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .context-select {
    width: 132px;
  }

  .context-session {
    width: 210px;
  }
}
</style>

