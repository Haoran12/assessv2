import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAppStore } from "@/stores/app";
import MainLayout from "@/layouts/MainLayout.vue";
import LoginView from "@/views/LoginView.vue";
import DashboardView from "@/views/DashboardView.vue";
import PlaceholderView from "@/views/PlaceholderView.vue";

const moduleRoutes: RouteRecordRaw[] = [
  {
    path: "org",
    name: "org",
    component: PlaceholderView,
    props: { title: "组织架构管理", apiGroup: "/api/org" },
  },
  {
    path: "assessment",
    name: "assessment",
    component: PlaceholderView,
    props: { title: "考核管理", apiGroup: "/api/assessment" },
  },
  {
    path: "rules",
    name: "rules",
    component: PlaceholderView,
    props: { title: "规则配置", apiGroup: "/api/rules" },
  },
  {
    path: "scores",
    name: "scores",
    component: PlaceholderView,
    props: { title: "分数管理", apiGroup: "/api/scores" },
  },
  {
    path: "votes",
    name: "votes",
    component: PlaceholderView,
    props: { title: "投票管理", apiGroup: "/api/votes" },
  },
  {
    path: "calc",
    name: "calc",
    component: PlaceholderView,
    props: { title: "计算引擎", apiGroup: "/api/calc" },
  },
  {
    path: "reports",
    name: "reports",
    component: PlaceholderView,
    props: { title: "报表中心", apiGroup: "/api/reports" },
  },
  {
    path: "backup",
    name: "backup",
    component: PlaceholderView,
    props: { title: "备份审计", apiGroup: "/api/backup" },
  },
  {
    path: "system",
    name: "system",
    component: PlaceholderView,
    props: { title: "系统管理", apiGroup: "/api/system" },
  },
];

const routes: RouteRecordRaw[] = [
  {
    path: "/login",
    name: "login",
    component: LoginView,
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
      },
      ...moduleRoutes,
    ],
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach((to) => {
  const store = useAppStore();
  if (to.path === "/login") {
    return true;
  }
  if (!store.isAuthed) {
    return "/login";
  }
  return true;
});

export default router;

