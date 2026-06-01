import { computed, watch, type Ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useWorkspaceStore } from "../stores/workspace";
import { parseView, serializeView } from "../stores/workspaceUrl";
import { Layout, type PluginProjection } from "../types/projection";

interface Options {
  connectionId: () => string;
  projection: Ref<PluginProjection | null>;
}

// Keeps only the active workspace view in the URL. Sidebar tree disclosure state
// is local UI state and must not be restored or rewritten by browser history.
export function useWorkspaceUrlSync({ connectionId, projection }: Options) {
  const route = useRoute();
  const router = useRouter();
  const ws = useWorkspaceStore();

  const activeLocator = computed<string | undefined>(() => {
    const proj = projection.value;
    const id = connectionId();
    if (!proj) return undefined;
    if (proj.layout === Layout.Tabs) return ws.view(id).activeTab || undefined;
    if (proj.layout === Layout.SidebarTree) {
      const active = ws.activeView(id);
      return active ? serializeView(active) : undefined;
    }
    return undefined;
  });

  function applyLocator(v?: string): void {
    const proj = projection.value;
    const id = connectionId();
    if (!proj) return;
    if (proj.layout === Layout.Tabs) {
      if (v && proj.tabs?.some((tab) => tab.key === v)) {
        ws.setActiveTab(id, v);
      }
      return;
    }
    if (proj.layout !== Layout.SidebarTree || !v) return;
    const parsed = parseView(v, proj.resources ?? [], proj.tree ?? []);
    if (!parsed) return;
    if (ws.view(id).views.some((open) => open.id === parsed.id)) {
      ws.activateView(id, parsed.id);
    } else {
      ws.openPreviewView(id, parsed);
    }
  }

  function restoreFromUrl(): void {
    const v = typeof route.query.v === "string" ? route.query.v : undefined;
    applyLocator(v);
  }

  let prevViewIds = new Set<string>();

  watch(activeLocator, (loc) => {
    const proj = projection.value;
    if (!proj) return;

    const id = connectionId();
    const current =
      typeof route.query.v === "string" ? route.query.v : undefined;
    const openIds = new Set(ws.view(id).views.map((view) => view.id));
    if (loc === current) {
      prevViewIds = openIds;
      return;
    }

    const query = { ...route.query };
    if (loc) query.v = loc;
    else delete query.v;

    const activeId = ws.activeView(id)?.id;
    const isNewView =
      proj.layout === Layout.SidebarTree &&
      current !== undefined &&
      Boolean(activeId) &&
      !prevViewIds.has(activeId as string);
    prevViewIds = openIds;
    void router[isNewView ? "push" : "replace"]({ query });
  });

  watch(
    () => route.query.v,
    (raw) => {
      const v = typeof raw === "string" ? raw : undefined;
      if (v === activeLocator.value) return;
      applyLocator(v);
    },
  );

  return { restoreFromUrl };
}
