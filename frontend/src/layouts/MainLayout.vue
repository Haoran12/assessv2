<template>
  <el-container class="app-shell">
    <el-aside width="250px" class="app-sidebar">
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
      <el-header class="app-header">
        <div class="header-left">
          <div v-if="appStore.isAuthed" class="global-context">
            <el-select
              v-model="contextYearId"
              class="context-select"
              placeholder="选择年度"
              :loading="contextStore.loadingYears"
              clearable
            >
              <el-option
                v-for="item in contextStore.years"
                :key="item.id"
                :label="formatAssessmentYearLabel(item)"
                :value="item.id"
              />
            </el-select>
            <el-select
              v-model="contextPeriodCode"
              class="context-select"
              placeholder="选择周期"
              :loading="contextStore.loadingPeriods"
              :disabled="!contextYearId"
            >
              <el-option
                v-for="item in periodOptions"
                :key="item.id"
                :label="periodLabel(item.periodCode, item.periodName)"
                :value="item.periodCode"
              />
            </el-select>
            <el-select v-model="contextObjectCategory" class="context-select" placeholder="考核分类">
              <el-option
                v-for="item in objectCategoryOptions"
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
              {{ appStore.username || "\u672a\u767b\u5f55" }}
              <el-icon class="el-icon--right"><arrow-down /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item disabled>
                  <span class="role-tag">{{ roleLabel(appStore.primaryRole) }}</span>
                </el-dropdown-item>
                <el-dropdown-item divided @click="goToChangePassword">\u4fee\u6539\u5bc6\u7801</el-dropdown-item>
                <el-dropdown-item @click="handleLogout">\u9000\u51fa\u767b\u5f55</el-dropdown-item>
                <el-dropdown-item divided @click="handleExitSystem">\u9000\u51fa\u7cfb\u7edf</el-dropdown-item>
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
import { useUnsavedStore } from "@/stores/unsaved";
import { resolveUnsavedBeforeLeave } from "@/guards/unsaved";
import type { AssessmentPeriodCode, GlobalAssessmentObjectCategory } from "@/types/assessment";
import { formatAssessmentYearLabel, periodDisplayLabel } from "@/utils/assessment";
import { appBrandName } from "@/config/branding";

interface NavItem {
  path: string;
  label: string;
  permission?: string;
  rootOnly?: boolean;
}

interface DesktopAppBridge {
  ExitSystem?: () => Promise<void> | void;
  SetPreferredDataYear?: (year: number) => Promise<void> | void;
  SetCloseGuard?: (enabled: boolean) => Promise<void> | void;
}

const navItems: NavItem[] = [
  { path: "/overview", label: "\u7cfb\u7edf\u6982\u89c8" },
  { path: "/period-management", label: "\u5468\u671f\u7ba1\u7406", permission: "assessment:view" },
  { path: "/org", label: "\u7ec4\u7ec7\u67b6\u6784", permission: "org:view" },
  { path: "/rules/total", label: "\u603b\u5206\u89c4\u5219", permission: "rule:view" },
  { path: "/rules/module", label: "\u6a21\u5757\u89c4\u5219", permission: "rule:view" },
  { path: "/rules/grade", label: "\u7b49\u7b2c\u89c4\u5219", permission: "rule:view" },
  { path: "/system/users", label: "\u7528\u6237\u7ba1\u7406", permission: "user:view", rootOnly: true },
  { path: "/system/backup", label: "\u5907\u4efd\u6062\u590d", permission: "backup:view" },
  { path: "/system/audit", label: "\u5ba1\u8ba1\u65e5\u5fd7", permission: "audit:view" },
  { path: "/system/settings", label: "\u7cfb\u7edf\u8bbe\u7f6e", permission: "setting:view", rootOnly: true },
];

const route = useRoute();
const router = useRouter();
const appStore = useAppStore();
const contextStore = useContextStore();
const unsavedStore = useUnsavedStore();
let closeGuardSyncTimer: number | null = null;
let lastSyncedCloseGuardState: boolean | null = null;
let bypassBeforeUnloadUntil = 0;
let closeRequestHandlerDisposer: (() => void) | null = null;
let handlingCloseRequest = false;

const objectCategoryOptions = computed(() => contextStore.categoryOptions);
const activePath = computed(() => route.path);
const visibleMenus = computed(() =>
  navItems.filter((item) => {
    if (item.rootOnly && appStore.primaryRole !== "root" && !appStore.roles.includes("root")) {
      return false;
    }
    return !item.permission || appStore.hasPermission(item.permission);
  }),
);
const periodOptions = computed(() => contextStore.periods);

const contextYearId = computed({
  get: () => contextStore.yearId,
  set: (value) => {
    contextStore.setYear(value).catch(() => {
      ElMessage.error("全局年度切换失败");
    });
  },
});
const contextPeriodCode = computed({
  get: () => contextStore.periodCode,
  set: (value: AssessmentPeriodCode) => contextStore.setPeriodCode(value),
});
const contextObjectCategory = computed({
  get: () => contextStore.objectCategory,
  set: (value: GlobalAssessmentObjectCategory) => contextStore.setObjectCategory(value),
});

watch(
  () => appStore.isAuthed,
  async (authed) => {
    if (!authed) {
      return;
    }
    try {
      await contextStore.ensureInitialized();
    } catch (_error) {
      ElMessage.error("\u5168\u5c40\u4e0a\u4e0b\u6587\u52a0\u8f7d\u5931\u8d25");
    }
  },
  { immediate: true },
);

watch(
  () => contextStore.currentYear?.year,
  (year) => {
    if (!year || !appStore.isAuthed) {
      return;
    }
    void syncPreferredDataYear(year);
  },
  { immediate: true },
);

watch(
  () => unsavedStore.hasUnsavedChanges,
  () => {
    scheduleCloseGuardSync();
  },
  { immediate: true },
);

watch(
  () => appStore.isAuthed,
  () => {
    scheduleCloseGuardSync();
  },
  { immediate: true },
);

onMounted(() => {
  if (typeof window !== "undefined") {
    window.addEventListener("beforeunload", handleBeforeUnload);
  }

  bindDesktopCloseRequestHandler();

  scheduleCloseGuardSync();
});

onBeforeUnmount(() => {
  if (typeof window !== "undefined") {
    window.removeEventListener("beforeunload", handleBeforeUnload);
    if (closeGuardSyncTimer !== null) {
      window.clearTimeout(closeGuardSyncTimer);
      closeGuardSyncTimer = null;
    }
  }

  if (closeRequestHandlerDisposer) {
    closeRequestHandlerDisposer();
    closeRequestHandlerDisposer = null;
  }

  clearDesktopCloseGuard();
});

function periodLabel(code: AssessmentPeriodCode, name?: string): string {
  return periodDisplayLabel(code, name);
}

async function handleLogout(): Promise<void> {
  const allowed = await resolveUnsavedBeforeLeave({
    title: "退出登录提醒",
    message: "检测到当前有未保存改动，退出登录后将丢失。请选择后续操作。",
  });
  if (!allowed) {
    return;
  }

  await appStore.logout();
  unsavedStore.clearAll();
  ElMessage.success("\u5df2\u9000\u51fa\u767b\u5f55");
  await router.push("/login");
}

async function goToChangePassword(): Promise<void> {
  await router.push("/change-password");
}

function isDesktopRuntime(): boolean {
  return typeof navigator !== "undefined" && navigator.userAgent.toLowerCase().includes("wails");
}

function hasPendingExitChanges(): boolean {
  return unsavedStore.hasUnsavedChanges;
}

function getDesktopAppBridge(): DesktopAppBridge | undefined {
  if (typeof window === "undefined" || !isDesktopRuntime()) {
    return undefined;
  }
  const goBridge = (window as Window & { go?: { main?: { App?: DesktopAppBridge } } }).go;
  return goBridge?.main?.App;
}

function scheduleCloseGuardSync(): void {
  if (typeof window === "undefined") {
    return;
  }
  if (closeGuardSyncTimer !== null) {
    window.clearTimeout(closeGuardSyncTimer);
  }
  closeGuardSyncTimer = window.setTimeout(() => {
    closeGuardSyncTimer = null;
    void syncDesktopCloseGuard();
  }, 120);
}

async function syncDesktopCloseGuard(force = false): Promise<void> {
  const appBridge = getDesktopAppBridge();
  if (!appBridge?.SetCloseGuard) {
    return;
  }

  const enabled = hasPendingExitChanges();
  if (!force && lastSyncedCloseGuardState === enabled) {
    return;
  }

  lastSyncedCloseGuardState = enabled;
  try {
    await appBridge.SetCloseGuard(enabled);
  } catch (_error) {
    // Ignore close guard sync failures.
  }
}

function clearDesktopCloseGuard(): void {
  const appBridge = getDesktopAppBridge();
  if (!appBridge?.SetCloseGuard) {
    return;
  }

  lastSyncedCloseGuardState = false;
  void appBridge.SetCloseGuard(false);
}

function handleBeforeUnload(event: BeforeUnloadEvent): void {
  if (!isDesktopRuntime()) {
    return;
  }
  if (Date.now() < bypassBeforeUnloadUntil) {
    return;
  }
  if (!hasPendingExitChanges()) {
    return;
  }

  event.preventDefault();
  event.returnValue = "";
}

async function tryExitDesktopRuntime(): Promise<boolean> {
  if (typeof window === "undefined" || !isDesktopRuntime()) {
    return false;
  }

  const appBridge = getDesktopAppBridge();
  if (appBridge?.ExitSystem) {
    await appBridge.ExitSystem();
    return true;
  }

  const runtimeBridge = (window as Window & { runtime?: { Quit?: () => void } }).runtime;
  if (runtimeBridge?.Quit) {
    runtimeBridge.Quit();
    return true;
  }

  return false;
}

async function syncPreferredDataYear(year: number): Promise<void> {
  const appBridge = getDesktopAppBridge();
  if (!appBridge?.SetPreferredDataYear) {
    return;
  }

  try {
    await appBridge.SetPreferredDataYear(year);
  } catch (_error) {
    // Ignore preference sync failures.
  }
}

function bindDesktopCloseRequestHandler(): void {
  if (typeof window === "undefined" || !isDesktopRuntime()) {
    return;
  }

  const runtimeBridge = (window as Window & {
    runtime?: {
      EventsOn?: (eventName: string, callback: () => void) => (() => void) | void;
    };
  }).runtime;
  if (!runtimeBridge?.EventsOn) {
    return;
  }

  const dispose = runtimeBridge.EventsOn("app:close-requested", () => {
    void handleDesktopCloseRequest();
  });
  if (typeof dispose === "function") {
    closeRequestHandlerDisposer = dispose;
  }
}

async function exitSystemInternal(): Promise<void> {
  try {
    await appStore.logout();
  } catch (_error) {
    // Best effort cleanup before quit.
  }
  unsavedStore.clearAll();

  bypassBeforeUnloadUntil = Date.now() + 3000;
  const exited = await tryExitDesktopRuntime();
  if (!exited) {
    bypassBeforeUnloadUntil = 0;
    ElMessage.success("\u5df2\u9000\u51fa\u767b\u5f55");
    await router.push("/login");
  }
}

async function handleDesktopCloseRequest(): Promise<void> {
  if (handlingCloseRequest) {
    return;
  }
  handlingCloseRequest = true;
  try {
    const allowed = await resolveUnsavedBeforeLeave({
      title: "关闭软件提醒",
      message: "检测到当前有未保存改动，关闭后将丢失。请选择后续操作。",
    });
    if (!allowed) {
      return;
    }
    await exitSystemInternal();
  } finally {
    handlingCloseRequest = false;
  }
}

async function handleExitSystem(): Promise<void> {
  const allowed = await resolveUnsavedBeforeLeave({
    title: "退出系统提醒",
    message: "检测到当前有未保存改动，退出后将丢失。请选择后续操作。",
  });
  if (!allowed) {
    return;
  }

  await exitSystemInternal();
}

function roleLabel(roleCode: string): string {
  switch (roleCode) {
    case "root":
      return "Root";
    case "viewer":
      return "\u67e5\u770b\u8005";
    case "":
      return "\u672a\u5206\u914d\u89d2\u8272";
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

.brand-subtitle {
  margin-top: 4px;
  color: #6b7280;
  font-size: 12px;
}

.app-header {
  border-bottom: 1px solid #ebeef5;
  display: flex;
  align-items: center;
  gap: 12px;
  background: #fff;
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
  width: 170px;
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
    width: 145px;
  }
}
</style>
