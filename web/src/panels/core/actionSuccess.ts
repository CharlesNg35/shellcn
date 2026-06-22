import { nextTick, type ComputedRef } from "vue";
import {
  channelKey,
  interpolate,
  resolveParams,
  type ResolveContext,
} from "@/api/dataSource";
import { useNotify } from "@/composables/useNotify";
import { useDockStore } from "@/stores/dock";
import { useStreamChannelsStore } from "@/stores/streamChannels";
import {
  ActionEffectType,
  PanelType,
  type Action,
  type ActionEffect,
  type OpenPanelEffect,
  type Tab as TabDef,
  type TerminalInputEffect,
} from "@/types/projection";

export interface ActionSuccessRuntime {
  connectionId: () => string;
  tabs: ComputedRef<TabDef[]>;
  resolvePanel: (tab: TabDef) => PanelType;
  selectTab: (key: string) => void;
  context?: () => ResolveContext;
}

// resolveTitle interpolates ${response.x}/${resource.x} tokens in a panel title,
// the same context the source params use. Plain titles pass through untouched, and
// an unresolvable token falls back to the raw title rather than failing the open.
function resolveTitle(
  template: string | undefined,
  ctx: ResolveContext,
): string {
  if (!template || !template.includes("${")) return template ?? "";
  try {
    return interpolate(template, ctx);
  } catch {
    return template;
  }
}

export function useActionSuccess(runtime: ActionSuccessRuntime) {
  const notify = useNotify();

  async function run(
    action: Action,
    result?: Record<string, unknown>,
  ): Promise<void> {
    selectSuccessTab(action);

    for (const effect of action.onSuccess?.effects ?? []) {
      await runEffect(action, effect, result);
    }
  }

  function selectSuccessTab(action: Action): void {
    const tabKey = action.onSuccess?.selectTab;
    if (tabKey && runtime.tabs.value.some((tab) => tab.key === tabKey)) {
      runtime.selectTab(tabKey);
    }
  }

  async function runEffect(
    action: Action,
    effect: ActionEffect,
    result?: Record<string, unknown>,
  ): Promise<void> {
    switch (effect.type) {
      case ActionEffectType.TerminalInput:
        await runTerminalInput(action, effect.terminalInput, result);
        break;
      case ActionEffectType.OpenPanel:
        runOpenPanel(effect.openPanel, result);
        break;
    }
  }

  // runOpenPanel opens a dock/dialog panel after the action, resolving the source
  // params against the action result (${response.x}) plus the active context.
  function runOpenPanel(
    effect?: OpenPanelEffect,
    result?: Record<string, unknown>,
  ): void {
    if (!effect?.source) return;
    const ctx: ResolveContext = {
      ...(runtime.context?.() ?? {}),
      response: result ?? null,
    };
    const params = resolveParams(effect.source.params, ctx);
    const id = `${effect.source.routeId}:${Object.values(params).join(":")}`;
    const item = {
      id,
      title: resolveTitle(effect.title, ctx),
      icon: effect.icon,
      panel: effect.panel,
      source: { ...effect.source, params },
      config: effect.config as Record<string, unknown> | undefined,
      resource: ctx.resource ?? null,
      record: ctx.record ?? null,
    };
    const dock = useDockStore();
    if (effect.open === "dialog") {
      dock.openDialog(runtime.connectionId(), item);
    } else {
      dock.open(runtime.connectionId(), item);
    }
  }

  async function runTerminalInput(
    action: Action,
    effect?: TerminalInputEffect,
    result?: Record<string, unknown>,
  ): Promise<void> {
    if (!effect) return;

    const text = terminalInputText(effect, result);
    if (!text) return;

    const tabKey = effect.tab || action.onSuccess?.selectTab;
    const tab = runtime.tabs.value.find(
      (candidate) => candidate.key === tabKey,
    );
    if (!tab) return;

    runtime.selectTab(tab.key);
    await nextTick();

    const key = terminalStreamKey(tab);
    const streams = useStreamChannelsStore();
    if (!key || !(await streams.sendWhenOpen(key, text))) {
      notify.error(
        "Terminal is not ready",
        "Open the terminal and run the action again.",
      );
      return;
    }
  }

  function terminalStreamKey(tab: TabDef): string | null {
    if (!tab.source) return null;

    const base = channelKey(
      runtime.connectionId(),
      tab.source,
      runtime.context?.() ?? {},
    );
    if (runtime.resolvePanel(tab) !== PanelType.TerminalGrid) return base;

    const suffix =
      useStreamChannelsStore().preferredTerminalTarget(base) ?? "pane-1";
    return `${base}:${suffix}`;
  }

  return { run };
}

function terminalInputText(
  effect: TerminalInputEffect,
  result?: Record<string, unknown>,
): string | null {
  const raw =
    effect.text ??
    (effect.resultField && result ? result[effect.resultField] : undefined);
  if (typeof raw !== "string" || raw.length === 0) return null;
  return effect.appendNewline && !raw.endsWith("\n") ? `${raw}\n` : raw;
}
