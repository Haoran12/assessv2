import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAppStore } from "@/stores/app";
import { resolveUnsavedBeforeLeave } from "@/guards/unsaved";

const MainLayout = () => import("@/layouts/MainLayout.vue");
const LoginView = () => import("@/views/LoginView.vue");
const ChangePasswordView = () => import("@/views/ChangePasswordView.vue");
const SystemUsersView = () => import("@/views/SystemUsersView.vue");
const ForbiddenView = () => import("@/views/ForbiddenView.vue");
const OrganizationView = () => import("@/views/OrganizationView.vue");
const SystemOverviewView = () => import("@/views/SystemOverviewView.vue");
const AssessmentView = () => import("@/views/AssessmentView.vue");
const BackupManageView = () => import("@/views/BackupManageView.vue");
const AuditLogsView = () => import("@/views/AuditLogsView.vue");
const SystemSettingsView = () => import("@/views/SystemSettingsView.vue");

const moduleRoutes: RouteRecordRaw[] = [
  {
    path: "overview",
    name: "overview",
    component: SystemOverviewView,
    meta: { requiresAuth: true },
  },
  {
    path: "dashboard",
    name: "dashboard",
    redirect: "/overview",
  },
  {
    path: "org",
    name: "org",
    component: OrganizationView,
    meta: { requiresAuth: true, permission: "org:view" },
  },
  {
    path: "assessment-management",
    name: "assessment-management",
    component: AssessmentView,
    meta: { requiresAuth: true, permission: "assessment:view" },
  },
  {
    path: "rules",
    name: "rules",
    redirect: "/overview",
    meta: { requiresAuth: true },
  },
  {
    path: "system",
    name: "system",
    redirect: "/system/users",
  },
  {
    path: "system/users",
    name: "system-users",
    component: SystemUsersView,
    meta: { requiresAuth: true, permission: "user:view", requiresRoot: true },
  },
  {
    path: "system/backup",
    name: "system-backup",
    component: BackupManageView,
    meta: { requiresAuth: true, permission: ["backup:view", "backup:org:view"] },
  },
  {
    path: "system/audit",
    name: "system-audit",
    component: AuditLogsView,
    meta: { requiresAuth: true, permission: "audit:view" },
  },
  {
    path: "system/settings",
    name: "system-settings",
    component: SystemSettingsView,
    meta: { requiresAuth: true, permission: "setting:view", requiresRoot: true },
  },
];

const routes: RouteRecordRaw[] = [
  {
    path: "/login",
    name: "login",
    component: LoginView,
    meta: { publicOnly: true },
  },
  {
    path: "/change-password",
    name: "change-password",
    component: ChangePasswordView,
    meta: { requiresAuth: true, allowWhenMustChange: true },
  },
  {
    path: "/403",
    name: "forbidden",
    component: ForbiddenView,
    meta: { requiresAuth: true, allowWhenMustChange: true },
  },
  {
    path: "/",
    component: MainLayout,
    children: [
      {
        path: "",
        redirect: "/overview",
      },
      ...moduleRoutes,
    ],
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach(async (to, from) => {
  const store = useAppStore();
  const requiresAuth = to.matched.some((record) => record.meta.requiresAuth);
  const publicOnly = to.matched.some((record) => record.meta.publicOnly);
  const allowWhenMustChange = to.matched.some((record) => record.meta.allowWhenMustChange);

  if (from.matched.length > 0 && to.fullPath !== from.fullPath) {
    const allowed = await resolveUnsavedBeforeLeave();
    if (!allowed) {
      return false;
    }
  }

  if (!store.initialized) {
    try {
      await store.initializeSession();
    } catch (_error) {
      // Session init failed and store has been cleared.
    }
  }

  if (publicOnly) {
    if (!store.isAuthed) {
      return true;
    }
    return store.mustChangePassword ? "/change-password" : "/dashboard";
  }

  if (requiresAuth && !store.isAuthed) {
    return `/login?redirect=${encodeURIComponent(to.fullPath)}`;
  }

  if (store.isAuthed && store.mustChangePassword && !allowWhenMustChange) {
    return "/change-password";
  }

  const requiresRoot = to.matched.some((record) => record.meta.requiresRoot);
  if (requiresRoot) {
    const isRoot = store.primaryRole === "root" || store.roles.includes("root");
    if (!isRoot) {
      return "/403";
    }
  }

  const permissionMeta = to.meta.permission;
  if (permissionMeta) {
    const required = Array.isArray(permissionMeta)
      ? (permissionMeta as string[])
      : [String(permissionMeta)];
    if (!store.hasAnyPermission(required)) {
      return "/403";
    }
  }

  return true;
});

export default router;
