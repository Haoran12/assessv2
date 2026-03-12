import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAppStore } from "@/stores/app";
import MainLayout from "@/layouts/MainLayout.vue";
import LoginView from "@/views/LoginView.vue";
import DashboardView from "@/views/DashboardView.vue";
import PlaceholderView from "@/views/PlaceholderView.vue";
import ChangePasswordView from "@/views/ChangePasswordView.vue";
import SystemUsersView from "@/views/SystemUsersView.vue";
import ForbiddenView from "@/views/ForbiddenView.vue";
import RulesView from "@/views/RulesView.vue";
import OrganizationView from "@/views/OrganizationView.vue";
import AssessmentView from "@/views/AssessmentView.vue";
import ScoreDirectView from "@/views/ScoreDirectView.vue";
import ScoreExtraView from "@/views/ScoreExtraView.vue";
import VoteTaskView from "@/views/VoteTaskView.vue";
import VoteExecuteView from "@/views/VoteExecuteView.vue";
import VoteStatisticsView from "@/views/VoteStatisticsView.vue";

const moduleRoutes: RouteRecordRaw[] = [
  {
    path: "org",
    name: "org",
    component: OrganizationView,
    meta: { requiresAuth: true, permission: "org:view" },
  },
  {
    path: "assessment",
    name: "assessment",
    component: AssessmentView,
    meta: { requiresAuth: true, permission: "assessment:view" },
  },
  {
    path: "rules",
    name: "rules",
    component: RulesView,
    meta: { requiresAuth: true, permission: "rule:view" },
  },
  {
    path: "scores",
    name: "scores",
    redirect: "/scores/direct",
  },
  {
    path: "score",
    name: "score-legacy",
    redirect: "/scores/direct",
  },
  {
    path: "score/direct",
    name: "score-legacy-direct",
    redirect: "/scores/direct",
  },
  {
    path: "score/extra",
    name: "score-legacy-extra",
    redirect: "/scores/extra",
  },
  {
    path: "scores/direct",
    name: "scores-direct",
    component: ScoreDirectView,
    meta: { requiresAuth: true, permission: "score:view", useGlobalContext: true },
  },
  {
    path: "scores/extra",
    name: "scores-extra",
    component: ScoreExtraView,
    meta: { requiresAuth: true, permission: "score:view", useGlobalContext: true },
  },
  {
    path: "votes",
    name: "votes",
    redirect: "/votes/task",
  },
  {
    path: "vote",
    name: "vote-legacy",
    redirect: "/votes/task",
  },
  {
    path: "vote/task",
    name: "vote-legacy-task",
    redirect: "/votes/task",
  },
  {
    path: "vote/execute",
    name: "vote-legacy-execute",
    redirect: "/votes/execute",
  },
  {
    path: "vote/statistics",
    name: "vote-legacy-statistics",
    redirect: "/votes/statistics",
  },
  {
    path: "votes/task",
    name: "votes-task",
    component: VoteTaskView,
    meta: { requiresAuth: true, permission: "score:view", useGlobalContext: true },
  },
  {
    path: "votes/execute",
    name: "votes-execute",
    component: VoteExecuteView,
    meta: { requiresAuth: true, permission: "score:view", useGlobalContext: true },
  },
  {
    path: "votes/statistics",
    name: "votes-statistics",
    component: VoteStatisticsView,
    meta: { requiresAuth: true, permission: "score:view", useGlobalContext: true },
  },
  {
    path: "calc",
    name: "calc",
    component: PlaceholderView,
    props: { title: "Calc", apiGroup: "/api/calc" },
    meta: { requiresAuth: true, permission: "score:*" },
  },
  {
    path: "reports",
    name: "reports",
    component: PlaceholderView,
    props: { title: "Reports", apiGroup: "/api/reports" },
    meta: { requiresAuth: true, permission: "report:view" },
  },
  {
    path: "backup",
    name: "backup",
    component: PlaceholderView,
    props: { title: "Backup", apiGroup: "/api/backup" },
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
