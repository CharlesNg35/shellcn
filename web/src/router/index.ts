import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "../stores/auth";

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: "/login",
      name: "login",
      component: () => import("../views/LoginView.vue"),
    },
    {
      path: "/invite/:token",
      name: "accept-invite",
      component: () => import("../views/AcceptInviteView.vue"),
    },
    {
      path: "/",
      component: () => import("../components/AppShell.vue"),
      children: [
        {
          path: "",
          name: "home",
          component: () => import("../views/HomeView.vue"),
        },
        {
          path: "users",
          name: "users",
          component: () => import("../views/UsersView.vue"),
        },
        {
          path: "c/:id",
          name: "connection",
          component: () => import("../views/ConnectionWorkspace.vue"),
          props: true,
        },
        {
          path: "credentials",
          name: "credentials",
          component: () => import("../views/CredentialsView.vue"),
        },
        {
          path: "settings",
          name: "settings",
          component: () => import("../views/SettingsView.vue"),
        },
      ],
    },
    { path: "/:pathMatch(.*)*", redirect: { name: "home" } },
  ],
});

// Public routes need no session (login + invitation acceptance).
const publicRoutes = new Set(["login", "accept-invite"]);

// Gate every route behind an established session; bootstrap runs once.
router.beforeEach(async (to) => {
  const auth = useAuthStore();
  await auth.ensureReady();
  if (!publicRoutes.has(String(to.name)) && !auth.isAuthenticated) {
    const redirect =
      to.fullPath !== "/" ? { redirect: to.fullPath } : undefined;
    return { name: "login", query: redirect };
  }
  if (to.name === "login" && auth.isAuthenticated) {
    return { name: "home" };
  }
  return true;
});

export default router;
