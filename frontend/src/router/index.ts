import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAppStore } from "@/stores/app";
import MainLayout from "@/layouts/MainLayout.vue";
import LoginView from "@/views/LoginView.vue";
import DashboardView from "@/views/DashboardView.vue";
import PlaceholderView from "@/views/PlaceholderView.vue";
import ChangePasswordView from "@/views/ChangePasswordView.vue";
import SystemUsersView from "@/views/SystemUsersView.vue";
import ForbiddenView from "@/views/ForbiddenView.vue";

const moduleRoutes: RouteRecordRaw[] = [
  {
    path: "org",
    name: "org",
    component: PlaceholderView,
    props: { title: "组织架构", apiGroup: "/api/org" },
    meta: { requiresAuth: true, permission: "org:*" },
  },
  {
    path: "assessment",
    name: "assessment",
    component: PlaceholderView,
    props: { title: "考核管理", apiGroup: "/api/assessment" },
    meta: { requiresAuth: true, permission: "assessment:view" },
  },
  {
    path: "rules",
    name: "rules",
    component: PlaceholderView,
    props: { title: "规则配置", apiGroup: "/api/rules" },
    meta: { requiresAuth: true, permission: "rule:*" },
  },
  {
    path: "scores",
    name: "scores",
    component: PlaceholderView,
    props: { title: "分数管理", apiGroup: "/api/scores" },
    meta: { requiresAuth: true, permission: "score:view" },
  },
  {
    path: "votes",
    name: "votes",
    component: PlaceholderView,
    props: { title: "投票管理", apiGroup: "/api/votes" },
    meta: { requiresAuth: true, permission: "score:*" },
  },
  {
    path: "calc",
    name: "calc",
    component: PlaceholderView,
    props: { title: "计算引擎", apiGroup: "/api/calc" },
    meta: { requiresAuth: true, permission: "score:*" },
  },
  {
    path: "reports",
    name: "reports",
    component: PlaceholderView,
    props: { title: "报表中心", apiGroup: "/api/reports" },
    meta: { requiresAuth: true, permission: "report:view" },
  },
  {
    path: "backup",
    name: "backup",
    component: PlaceholderView,
    props: { title: "备份审计", apiGroup: "/api/backup" },
    meta: { requiresAuth: true, permission: "backup:*" },
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
    meta: { requiresAuth: true, permission: "user:view" },
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
        redirect: "/dashboard",
      },
      {
        path: "dashboard",
        name: "dashboard",
        component: DashboardView,
        meta: { requiresAuth: true },
      },
      ...moduleRoutes,
    ],
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach(async (to) => {
  const store = useAppStore();
  const requiresAuth = to.matched.some((record) => record.meta.requiresAuth);
  const publicOnly = to.matched.some((record) => record.meta.publicOnly);
  const allowWhenMustChange = to.matched.some((record) => record.meta.allowWhenMustChange);

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
