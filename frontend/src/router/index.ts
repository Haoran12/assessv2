import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAppStore } from "@/stores/app";

const MainLayout = () => import("@/layouts/MainLayout.vue");
const LoginView = () => import("@/views/LoginView.vue");
const PlaceholderView = () => import("@/views/PlaceholderView.vue");
const ChangePasswordView = () => import("@/views/ChangePasswordView.vue");
const SystemUsersView = () => import("@/views/SystemUsersView.vue");
const ForbiddenView = () => import("@/views/ForbiddenView.vue");
const RulesView = () => import("@/views/RulesView.vue");
const OrganizationView = () => import("@/views/OrganizationView.vue");
const ScoreDirectView = () => import("@/views/ScoreDirectView.vue");
const ScoreExtraView = () => import("@/views/ScoreExtraView.vue");
const VoteTaskView = () => import("@/views/VoteTaskView.vue");
const VoteExecuteView = () => import("@/views/VoteExecuteView.vue");
const VoteStatisticsView = () => import("@/views/VoteStatisticsView.vue");
const ResultOverviewView = () => import("@/views/ResultOverviewView.vue");
const SystemOverviewView = () => import("@/views/SystemOverviewView.vue");
const ModuleRulesView = () => import("@/views/ModuleRulesView.vue");
const GradeRulesPlaceholderView = () => import("@/views/GradeRulesPlaceholderView.vue");

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
    path: "assessment",
    name: "assessment",
    redirect: "/overview",
  },
  {
    path: "rules",
    name: "rules",
    redirect: "/rules/total",
  },
  {
    path: "rules/total",
    name: "rules-total",
    component: RulesView,
    meta: { requiresAuth: true, permission: "rule:view" },
  },
  {
    path: "rules/module",
    name: "rules-module",
    component: ModuleRulesView,
    meta: { requiresAuth: true, permission: "rule:view" },
  },
  {
    path: "rules/grade",
    name: "rules-grade",
    component: GradeRulesPlaceholderView,
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
    path: "results",
    name: "results",
    redirect: "/results/overview",
  },
  {
    path: "result",
    name: "result-legacy",
    redirect: "/results/overview",
  },
  {
    path: "results/overview",
    name: "results-overview",
    component: ResultOverviewView,
    meta: { requiresAuth: true, permission: "score:view", useGlobalContext: true },
  },
  {
    path: "calc",
    name: "calc",
    redirect: "/rules/module",
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
