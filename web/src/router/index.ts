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
      path: "/secure-account",
      name: "secure-account",
      component: () => import("../views/SecureAccountView.vue"),
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
        { path: "users", redirect: { name: "settings" } },
        {
          path: "settings/activity",
          name: "activity",
          component: () => import("../views/MyActivityView.vue"),
        },
        {
          path: "settings/users",
          name: "users",
          component: () => import("../views/UsersView.vue"),
          meta: { admin: true },
        },
        {
          path: "settings/users/:id",
          name: "user-detail",
          component: () => import("../views/UserDetailView.vue"),
          props: true,
          meta: { admin: true },
        },
        {
          path: "profile",
          name: "profile",
          component: () => import("../views/ProfileView.vue"),
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
          path: "recordings",
          name: "recordings",
          component: () => import("../views/RecordingsView.vue"),
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
  // Admin-only routes (e.g. user management) are not reachable by non-admins,
  // even via a direct URL — the backend enforces this too.
  if (to.meta.admin && !auth.isAdmin) {
    return { name: "home" };
  }
  return true;
});

export default router;
