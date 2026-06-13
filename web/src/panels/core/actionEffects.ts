import { nextTick, type ComputedRef } from "vue";
import { channelKey } from "@/api/dataSource";
import { useNotify } from "@/composables/useNotify";
import { useStreamChannelsStore } from "@/stores/streamChannels";
import {
  ActionEffectType,
  PanelType,
  type Action,
  type ActionEffect,
  type Tab as TabDef,
  type TerminalInputEffect,
} from "@/types/projection";

export interface ActionEffectRuntime {
  connectionId: () => string;
  tabs: ComputedRef<TabDef[]>;
  resolvePanel: (tab: TabDef) => PanelType;
  selectTab: (key: string) => void;
}

export function useActionEffects(runtime: ActionEffectRuntime) {
  const notify = useNotify();
  const streams = useStreamChannelsStore();

  async function run(
    action: Action,
    result?: Record<string, unknown>,
  ): Promise<void> {
    for (const effect of action.onSuccess?.effects ?? []) {
      await runEffect(action, effect, result);
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
    if (!key || !(await waitForOpenStreamKey(key))) {
      notify.error(
        "Terminal is not ready",
        "Open the terminal and run the action again.",
      );
      return;
    }

    streams.send(key, text);
  }

  function terminalStreamKey(tab: TabDef): string | null {
    if (!tab.source) return null;

    const base = channelKey(runtime.connectionId(), tab.source);
    if (runtime.resolvePanel(tab) !== PanelType.TerminalGrid) return base;

    const suffix = streams.preferredTerminalTarget(base) ?? "pane-1";
    return `${base}:${suffix}`;
  }

  async function waitForOpenStreamKey(key: string): Promise<boolean> {
    for (let i = 0; i < 40; i += 1) {
      if (streams.status(key) === "open") return true;
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    return streams.status(key) === "open";
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
