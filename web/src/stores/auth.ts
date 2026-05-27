import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { api, setCsrfToken } from "../api/client";

export interface AuthUser {
  id: string;
  username: string;
  displayName?: string;
  email?: string;
  roles: string[];
  protected?: boolean;
}

interface SessionDTO {
  user: AuthUser;
  csrfToken: string;
}

export const useAuthStore = defineStore("auth", () => {
  const user = ref<AuthUser | null>(null);
  const ready = ref(false);
  let bootstrapPromise: Promise<void> | null = null;

  const isAuthenticated = computed(() => user.value !== null);
  const isAdmin = computed(() => user.value?.roles.includes("admin") ?? false);

  function apply(session: SessionDTO): void {
    user.value = session.user;
    setCsrfToken(session.csrfToken);
  }

  function clear(): void {
    user.value = null;
    setCsrfToken("");
  }

  async function bootstrap(): Promise<void> {
    try {
      apply(await api.get<SessionDTO>("/auth/me"));
    } catch {
      clear();
    } finally {
      ready.value = true;
    }
  }

  // Bootstraps exactly once, awaited by the router guard.
  function ensureReady(): Promise<void> {
    if (!bootstrapPromise) bootstrapPromise = bootstrap();
    return bootstrapPromise;
  }

  async function login(username: string, password: string): Promise<void> {
    apply(await api.post<SessionDTO>("/auth/login", { username, password }));
    ready.value = true;
  }

  async function changePassword(
    currentPassword: string,
    newPassword: string,
  ): Promise<void> {
    apply(
      await api.post<SessionDTO>("/auth/me/password", {
        currentPassword,
        newPassword,
      }),
    );
    ready.value = true;
  }

  async function logout(): Promise<void> {
    try {
      await api.post("/auth/logout");
    } finally {
      clear();
    }
  }

  return {
    user,
    ready,
    isAuthenticated,
    isAdmin,
    ensureReady,
    bootstrap,
    login,
    changePassword,
    logout,
    clear,
  };
});
