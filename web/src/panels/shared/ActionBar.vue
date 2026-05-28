<script setup lang="ts">
import { ref } from "vue";
import Dialog from "primevue/dialog";
import Button from "primevue/button";
import { useToast } from "primevue/usetoast";
import { runFormAction } from "../../api/dataSource";
import type {
  Action,
  ResourceRef,
  RiskLevel,
  Row,
} from "../../types/projection";
import AppIcon from "../../components/AppIcon.vue";
import SchemaForm from "../form/SchemaForm.vue";
import { isVisible } from "../form/condition";
import { useDockStore, type DockItem } from "../../stores/dock";
import { dialogRoot } from "../../primevue/preset";
import { cn } from "../../utils/cn";

const dock = useDockStore();

const props = defineProps<{
  connectionId: string;
  actions: Action[];
  resource?: ResourceRef | null;
  record?: Row | null; // the active row, so actions can gate on its fields
  // Default params contributed by the surrounding context (e.g. the params of
  // the list the action sits on). Actions without their own resource inherit
  // these, so an action declared on a scoped view operates within that scope
  // without restating it; an action's explicit params always take precedence.
  scope?: Record<string, string> | null;
}>();

// Enabled unless the condition fails; when the row lacks the fields it needs
// (e.g. only a ref is known) we can't judge, so stay enabled rather than disable.
function isEnabled(action: Action): boolean {
  const cond = action.enabledWhen;
  if (!cond) return true;
  const record = (props.record ?? {}) as Record<string, unknown>;
  const rules = [...(cond.allOf ?? []), ...(cond.anyOf ?? [])];
  if (rules.some((r) => record[r.field] === undefined)) return true;
  return isVisible(cond, record);
}
const emit = defineEmits<{
  done: [action: Action, result?: Record<string, unknown>];
}>();

const toast = useToast();
const pending = ref<Action | null>(null);
const busy = ref(false);
const busyAction = ref<string | null>(null);
const error = ref<string | null>(null);

const riskClass: Record<RiskLevel, string> = {
  safe: "border border-surface-300 text-surface-700 hover:bg-surface-100 dark:border-surface-700 dark:text-surface-200 dark:hover:bg-surface-800",
  write: "bg-primary-600 text-white hover:bg-primary-700",
  destructive: "bg-rose-600 text-white hover:bg-rose-700",
  privileged: "bg-amber-600 text-white hover:bg-amber-700",
};

// Stable identity for the dock tab an action opens, so repeat clicks focus the
// existing tab instead of stacking duplicates. An action tied to a resource
// keys on that resource; otherwise it keys on its resolved params, so the same
// action run against different scopes opens distinct tabs.
function dockKey(action: Action): string {
  if (props.resource?.uid) return props.resource.uid;
  const params = actionParams(action);
  const sig = Object.keys(params)
    .sort()
    .map((k) => `${k}=${params[k]}`)
    .join("&");
  return sig || "connection";
}

function dockItem(action: Action): DockItem {
  return {
    id: `${action.id}:${dockKey(action)}`,
    title: props.resource?.name
      ? `${props.resource.name} · ${action.label}`
      : action.label,
    icon: action.icon,
    panel: action.panel as string,
    source: {
      routeId: action.routeId,
      method: action.method,
      params: actionParams(action),
    },
    config: action.config,
    resource: props.resource,
  };
}

function trigger(action: Action): void {
  error.value = null;
  if (action.panel && action.open === "dock") {
    dock.open(props.connectionId, dockItem(action));
    return;
  }
  if (action.panel && action.open === "dialog") {
    dock.openDialog(props.connectionId, dockItem(action));
    return;
  }
  if (action.open === "url") {
    void openURL(action);
    return;
  }
  if (
    action.requiresConfirm ||
    action.input ||
    action.risk === "destructive" ||
    action.risk === "privileged"
  )
    pending.value = action;
  else void execute(action);
}

function actionParams(action: Action): Record<string, string> {
  const base = props.scope ? { ...props.scope } : {};
  if (action.params) return { ...base, ...action.params };
  const ref = props.resource;
  if (!ref) return base;
  const params: Record<string, string> = {
    ...base,
    kind: ref.kind,
    name: ref.name,
    uid: ref.uid,
  };
  if (ref.namespace) params.namespace = ref.namespace;
  if (ref.scope) params.scope = ref.scope;
  return params;
}

// open="url": run the route, then open the returned {url} in a new tab.
async function openURL(action: Action): Promise<void> {
  error.value = null;
  busyAction.value = action.id;
  try {
    const result: unknown = await runFormAction(
      props.connectionId,
      action.routeId,
      { resource: props.resource },
      {},
      actionParams(action),
      action.method ?? "GET",
    );
    const raw =
      result && typeof result === "object"
        ? (result as Record<string, unknown>).url
        : undefined;
    if (typeof raw === "string" && raw) {
      window.open(raw, "_blank", "noopener,noreferrer");
    }
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    busyAction.value = null;
  }
}

async function execute(
  action: Action,
  body?: Record<string, unknown>,
): Promise<void> {
  busy.value = true;
  busyAction.value = action.id;
  error.value = null;
  try {
    const result = await runFormAction(
      props.connectionId,
      action.routeId,
      { resource: props.resource },
      body ?? {},
      actionParams(action),
      action.method ?? "POST",
    );
    pending.value = null;
    toast.add({
      severity: "success",
      summary: `${action.label} succeeded.`,
      life: 4000,
    });
    emit("done", action, result);
  } catch (e) {
    error.value = (e as Error).message;
    toast.add({
      severity: "error",
      summary: `${action.label} failed`,
      detail: (e as Error).message,
      life: 6000,
    });
  } finally {
    busy.value = false;
    busyAction.value = null;
  }
}

function onVisible(visible: boolean): void {
  if (!visible) {
    pending.value = null;
    error.value = null;
  }
}
</script>

<template>
  <div class="flex flex-wrap items-center gap-2">
    <Button
      v-for="action in actions"
      :key="action.id"
      type="button"
      :disabled="!isEnabled(action) || busyAction === action.id"
      :title="action.label"
      size="small"
      :pt="{
        root: cn(
          'inline-flex min-w-0 items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium transition-colors disabled:pointer-events-none disabled:opacity-40',
          riskClass[action.risk],
        ),
      }"
      @click="isEnabled(action) && trigger(action)"
    >
      <AppIcon
        :icon="action.icon"
        :size="15"
        :loading="busyAction === action.id"
      />
      {{ action.label }}
    </Button>

    <Dialog
      :visible="!!pending"
      modal
      :header="pending?.label"
      :dismissable-mask="true"
      :pt="{ root: dialogRoot('max-w-2xl') }"
      @update:visible="onVisible"
    >
      <template v-if="pending">
        <p v-if="pending.confirmText" class="mb-4 text-sm text-surface-500">
          {{ pending.confirmText }}
        </p>

        <SchemaForm
          v-if="pending.input"
          :schema="pending.input"
          :submit-label="pending.label"
          :busy="busy"
          @submit="execute(pending, $event)"
        />

        <template v-else>
          <p v-if="error" class="mb-3 text-sm text-red-500">{{ error }}</p>
          <div class="flex justify-end gap-2">
            <Button
              type="button"
              :pt="{
                root: 'rounded-md border border-surface-300 px-3 py-1.5 text-sm dark:border-surface-700',
              }"
              @click="onVisible(false)"
            >
              Cancel
            </Button>
            <Button
              type="button"
              label="Confirm"
              :loading="busy"
              :disabled="busy"
              autofocus
              :pt="{
                root: cn(
                  'inline-flex min-w-0 items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50',
                  pending.risk === 'destructive'
                    ? 'bg-rose-600'
                    : 'bg-primary-500',
                ),
              }"
              @click="execute(pending)"
            />
          </div>
        </template>
      </template>
    </Dialog>
  </div>
</template>
