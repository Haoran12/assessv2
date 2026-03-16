<template>
  <el-container class="app-shell">
    <el-aside width="250px" class="app-sidebar">
      <div class="brand">
        <div class="brand-title">考核管理系统</div>
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
              placeholder="年度"
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
              placeholder="周期"
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
                <el-dropdown-item divided @click="handleExitSystem">退出系统</el-dropdown-item>
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
import { ElMessage, ElMessageBox } from "element-plus";
import { ArrowDown } from "@element-plus/icons-vue";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import { useUnsavedStore } from "@/stores/unsaved";
import type { AssessmentPeriodCode, GlobalAssessmentObjectCategory } from "@/types/assessment";
import { formatAssessmentYearLabel } from "@/utils/assessment";

interface NavItem {
  path: string;
  label: string;
  permission?: string;
}

interface DesktopAppBridge {
  ExitSystem?: () => Promise<void> | void;
  SetPreferredDataYear?: (year: number) => Promise<void> | void;
  SetCloseGuard?: (enabled: boolean) => Promise<void> | void;
}

const navItems: NavItem[] = [
  { path: "/overview", label: "系统概览" },
  { path: "/org", label: "组织架构", permission: "org:view" },
  { path: "/rules/total", label: "总分规则", permission: "rule:view" },
  { path: "/rules/module", label: "模块规则", permission: "rule:view" },
  { path: "/rules/grade", label: "等级规则", permission: "rule:view" },
  { path: "/system/users", label: "用户管理", permission: "user:view" },
];

const route = useRoute();
const router = useRouter();
const appStore = useAppStore();
const contextStore = useContextStore();
const unsavedStore = useUnsavedStore();
let closeGuardObserver: MutationObserver | null = null;
let closeGuardSyncTimer: number | null = null;
let lastSyncedCloseGuardState: boolean | null = null;
let bypassBeforeUnloadUntil = 0;

const objectCategoryOptions = computed(() => contextStore.categoryOptions);
const activePath = computed(() => route.path);
const visibleMenus = computed(() =>
  navItems.filter((item) => !item.permission || appStore.hasPermission(item.permission)),
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
      ElMessage.error("全局上下文加载失败");
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

  if (typeof document !== "undefined" && typeof MutationObserver !== "undefined") {
    closeGuardObserver = new MutationObserver(() => {
      scheduleCloseGuardSync();
    });
    closeGuardObserver.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ["class", "style"],
    });
  }

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

  if (closeGuardObserver) {
    closeGuardObserver.disconnect();
    closeGuardObserver = null;
  }

  clearDesktopCloseGuard();
});

function periodLabel(code: AssessmentPeriodCode, name?: string): string {
  const text = name?.trim();
  return text ? `${code} - ${text}` : code;
}

async function handleLogout(): Promise<void> {
  await appStore.logout();
  unsavedStore.clearAll();
  ElMessage.success("已退出登录");
  await router.push("/login");
}

async function goToChangePassword(): Promise<void> {
  await router.push("/change-password");
}

function hasOpenEditorDialog(): boolean {
  if (typeof document === "undefined" || typeof window === "undefined") {
    return false;
  }

  const overlays = Array.from(document.querySelectorAll<HTMLElement>(".el-overlay"));
  return overlays.some((overlay) => {
    if (!overlay.querySelector(".el-dialog")) {
      return false;
    }
    if (overlay.querySelector(".el-message-box")) {
      return false;
    }
    const style = window.getComputedStyle(overlay);
    return style.display !== "none" && style.visibility !== "hidden";
  });
}

function isDesktopRuntime(): boolean {
  return typeof navigator !== "undefined" && navigator.userAgent.toLowerCase().includes("wails");
}

function hasPendingExitChanges(): boolean {
  return unsavedStore.hasUnsavedChanges || hasOpenEditorDialog();
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

async function confirmExitIfUnsaved(): Promise<boolean> {
  const hasUnsavedChanges = hasPendingExitChanges();
  if (!hasUnsavedChanges) {
    return true;
  }

  try {
    await ElMessageBox.confirm("检测到存在未保存的数据，退出后将丢失，是否继续？", "退出提醒", {
      type: "warning",
      confirmButtonText: "继续退出",
      cancelButtonText: "取消",
    });
    return true;
  } catch (_error) {
    return false;
  }
}

async function handleExitSystem(): Promise<void> {
  const allowed = await confirmExitIfUnsaved();
  if (!allowed) {
    return;
  }

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
    ElMessage.success("已退出登录");
    await router.push("/login");
  }
}

function roleLabel(roleCode: string): string {
  switch (roleCode) {
    case "root":
      return "Root";
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
