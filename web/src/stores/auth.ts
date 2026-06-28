import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { setCsrfToken } from "../api/client";
import { authApi, type AuthUser, type SessionDTO } from "../api/auth";
import { Role } from "../constants/roles";
import { resetSession } from "./session";

export type { AuthUser };

export const useAuthStore = defineStore("auth", () => {
  const user = ref<AuthUser | null>(null);
  const ready = ref(false);
  // Set to true after sign-in when the user should be nudged to enable 2FA.
  const mfaReminder = ref(false);
  // Challenge token carried between the password and second-factor login steps.
  const pendingMfaToken = ref<string | null>(null);
  let bootstrapPromise: Promise<void> | null = null;

  const isAuthenticated = computed(() => user.value !== null);
  const roles = computed<Role[]>(() => user.value?.roles ?? []);
  const isAdmin = computed(() => roles.value.includes(Role.Admin));
  const twoFactorEnabled = computed(
    () => user.value?.twoFactorEnabled ?? false,
  );
  const awaitingMfa = computed(() => pendingMfaToken.value !== null);
  // Viewers consume only shared resources; operators/admins may create their own.
  const canCreate = computed(
    () =>
      roles.value.includes(Role.Operator) || roles.value.includes(Role.Admin),
  );

  function apply(session: SessionDTO): void {
    user.value = session.user;
    setCsrfToken(session.csrfToken);
    mfaReminder.value = session.mfaReminder;
    pendingMfaToken.value = null;
    ready.value = true;
  }

  function clear(): void {
    user.value = null;
    setCsrfToken("");
    mfaReminder.value = false;
    pendingMfaToken.value = null;
  }

  async function bootstrap(): Promise<void> {
    try {
      apply(await authApi.me());
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

  // Returns mfaRequired; when true the caller collects a code and calls completeMfa.
  async function login(
    username: string,
    password: string,
  ): Promise<{ mfaRequired: boolean }> {
    const result = await authApi.login(username, password);
    if (result.mfaRequired && result.mfaToken) {
      pendingMfaToken.value = result.mfaToken;
      return { mfaRequired: true };
    }
    if (result.session) apply(result.session);
    return { mfaRequired: false };
  }

  async function completeMfa(code: string): Promise<void> {
    if (!pendingMfaToken.value) throw new Error("no pending sign-in");
    const result = await authApi.loginMfa(pendingMfaToken.value, code);
    if (result.session) apply(result.session);
  }

  function cancelMfa(): void {
    pendingMfaToken.value = null;
  }

  async function changePassword(
    currentPassword: string,
    newPassword: string,
  ): Promise<void> {
    apply(await authApi.changePassword(currentPassword, newPassword));
  }

  async function refresh(): Promise<void> {
    apply(await authApi.me());
  }

  function dismissReminder(): void {
    mfaReminder.value = false;
  }

  async function logout(): Promise<void> {
    try {
      await authApi.logout();
    } finally {
      clear();
      resetSession();
    }
  }

  return {
    user,
    ready,
    mfaReminder,
    isAuthenticated,
    isAdmin,
    canCreate,
    twoFactorEnabled,
    awaitingMfa,
    ensureReady,
    bootstrap,
    login,
    completeMfa,
    cancelMfa,
    changePassword,
    refresh,
    dismissReminder,
    logout,
    clear,
  };
});
