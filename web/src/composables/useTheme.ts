import { ref } from "vue";

type Theme = "light" | "dark";
const STORAGE_KEY = "shellcn.theme";

function initial(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "light" || stored === "dark") return stored;
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

const theme = ref<Theme>("dark");
const isDark = ref(true);

function apply(next: Theme): void {
  theme.value = next;
  isDark.value = next === "dark";
  document.documentElement.classList.toggle("dark", isDark.value);
  localStorage.setItem(STORAGE_KEY, next);
}

let started = false;

export function useTheme() {
  if (!started) {
    started = true;
    apply(initial());
  }
  return {
    isDark,
    theme,
    toggle: () => apply(isDark.value ? "light" : "dark"),
    set: apply,
  };
}
