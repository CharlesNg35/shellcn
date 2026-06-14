<script setup lang="ts">
import { computed, ref } from "vue";
import Dialog from "primevue/dialog";
import Button from "primevue/button";
import Menu from "primevue/menu";
import { useToast } from "primevue/usetoast";
import { fetchDoc, runFormAction } from "@/api/dataSource";
import type { Action, ResourceIdentity, Row } from "@/types/projection";
import { RiskLevel } from "@/types/projection";
import AppIcon from "@/components/AppIcon.vue";
import SchemaForm from "../form/SchemaForm.vue";
import { isVisible } from "../form/condition";
import { useDockStore, type DockItem } from "@/stores/dock";
import { dialogRoot } from "@/primevue/preset";
import { cn } from "@/utils/cn";

const dock = useDockStore();

const props = defineProps<{
  connectionId: string;
  actions: Action[];
  resource?: ResourceIdentity | null;
  record?: Row | null;
  resources?: ResourceIdentity[] | null;
  records?: Row[] | null;
  scope?: Record<string, string> | null;
}>();

interface ActionTarget {
  resource: ResourceIdentity | null;
  record: Row | null;
}

function actionTargets(): ActionTarget[] {
  if (props.records?.length) {
    return props.records.map((record, index) => ({
      resource: props.resources?.[index] ?? record.ref ?? null,
      record,
    }));
  }
  if (props.resources?.length) {
    return props.resources.map((resource) => ({ resource, record: null }));
  }
  if (props.resource || props.record) {
    return [
      {
        resource: props.resource ?? props.record?.ref ?? null,
        record: props.record ?? null,
      },
    ];
  }
  return [];
}

function firstTarget(): ActionTarget {
  return (
    actionTargets()[0] ?? {
      resource: props.resource ?? null,
      record: props.record ?? null,
    }
  );
}

function targetContext(target: ActionTarget) {
  return { resource: target.resource, record: target.record };
}

function recordMatches(
  cond: NonNullable<Action["enabledWhen"]>,
  rec: Row,
): boolean {
  const r = rec as Record<string, unknown>;
  const rules = [...(cond.allOf ?? []), ...(cond.anyOf ?? [])];
  if (rules.some((x) => r[x.field] === undefined)) return true;
  return isVisible(cond, r);
}

function isEnabled(action: Action): boolean {
  const cond = action.enabledWhen;
  if (!cond) return true;
  const recs = props.records?.length
    ? props.records
    : props.record
      ? [props.record]
      : [];
  return recs.every((rec) => recordMatches(cond, rec));
}

function isActionVisible(action: Action): boolean {
  const cond = action.visibleWhen;
  if (!cond) return true;
  const recs = props.records?.length
    ? props.records
    : props.record
      ? [props.record]
      : [];
  if (recs.length === 0) return true;
  return recs.every((rec) => recordMatches(cond, rec));
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
  [RiskLevel.Safe]:
    "border border-surface-300 text-surface-700 hover:bg-surface-100 dark:border-surface-700 dark:text-surface-200 dark:hover:bg-surface-800",
  [RiskLevel.Write]: "bg-primary-600 text-white hover:bg-primary-700",
  [RiskLevel.Destructive]: "bg-rose-600 text-white hover:bg-rose-700",
  [RiskLevel.Privileged]: "bg-amber-600 text-white hover:bg-amber-700",
};

const MAX_INLINE = 5;
const triggerClass =
  "inline-flex min-w-0 items-center justify-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium transition-colors " +
  riskClass.safe;

type RenderUnit =
  | { kind: "button"; action: Action }
  | { kind: "menu"; key: string; label: string; actions: Action[] };

const renderUnits = computed<RenderUnit[]>(() => {
  const groups = new Map<string, Extract<RenderUnit, { kind: "menu" }>>();
  const units: RenderUnit[] = [];
  for (const action of props.actions.filter(isActionVisible)) {
    if (action.group) {
      let unit = groups.get(action.group);
      if (!unit) {
        unit = {
          kind: "menu",
          key: `g:${action.group}`,
          label: action.group,
          actions: [],
        };
        groups.set(action.group, unit);
        units.push(unit);
      }
      unit.actions.push(action);
    } else {
      units.push({ kind: "button", action });
    }
  }
  return units;
});

const layout = computed<{ visible: RenderUnit[]; overflow: Action[] }>(() => {
  const units = renderUnits.value;
  if (units.length <= MAX_INLINE) return { visible: units, overflow: [] };
  const visible = [...units];
  const overflow: Action[] = [];
  for (
    let i = visible.length - 1;
    i >= 0 && visible.length > MAX_INLINE - 1;
    i--
  ) {
    const u = visible[i];
    if (u.kind === "button") {
      overflow.unshift(u.action);
      visible.splice(i, 1);
    }
  }
  return { visible, overflow };
});

function menuModel(actions: Action[]) {
  return actions.map((action) => ({
    label: action.label,
    action,
    disabled: !isEnabled(action) || busyAction.value === action.id,
    command: () => {
      if (isEnabled(action)) trigger(action);
    },
  }));
}

function menuItemClass(action: Action): string {
  return action.risk === RiskLevel.Destructive
    ? "text-rose-600 dark:text-rose-400"
    : "text-surface-700 dark:text-surface-200";
}

const menus = ref(new Map<string, { toggle: (event: Event) => void }>());
function setMenu(key: string, el: unknown): void {
  if (el) menus.value.set(key, el as { toggle: (event: Event) => void });
  else menus.value.delete(key);
}
function toggleMenu(key: string, event: Event): void {
  menus.value.get(key)?.toggle(event);
}

function dockKey(action: Action): string {
  const target = firstTarget();
  if (target.resource?.uid) return target.resource.uid;
  if (target.record) return JSON.stringify(target.record);
  const params = actionParams(action, target);
  const sig = Object.keys(params)
    .sort()
    .map((k) => `${k}=${params[k]}`)
    .join("&");
  return sig || "connection";
}

function dockItem(action: Action): DockItem {
  const target = firstTarget();
  const ref = target.resource;
  return {
    id: `${action.id}:${dockKey(action)}`,
    title: ref?.name ? `${ref.name} · ${action.label}` : action.label,
    icon: action.icon,
    panel: action.panel as string,
    source: {
      routeId: action.routeId,
      method: action.method,
      params: actionParams(action, target),
    },
    config: action.config,
    resource: ref,
    record: target.record,
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
  if (
    action.requiresConfirm ||
    action.input ||
    action.risk === RiskLevel.Destructive ||
    action.risk === RiskLevel.Privileged
  ) {
    pending.value = action;
    return;
  }
  if (action.open === "url") {
    void openURL(action);
    return;
  }
  void execute(action);
}

function actionParams(
  action: Action,
  target: ActionTarget = firstTarget(),
): Record<string, string> {
  const base = props.scope ? { ...props.scope } : {};
  if (action.params) return { ...base, ...action.params };
  const ref = target.resource;
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

function formParams(body?: Record<string, unknown>): Record<string, string> {
  const params: Record<string, string> = {};
  for (const [key, value] of Object.entries(body ?? {})) {
    if (value === undefined || value === null || value === "") continue;
    if (
      typeof value === "string" ||
      typeof value === "number" ||
      typeof value === "boolean"
    ) {
      params[key] = String(value);
    }
  }
  return params;
}

async function openURL(
  action: Action,
  body?: Record<string, unknown>,
): Promise<void> {
  error.value = null;
  busy.value = true;
  busyAction.value = action.id;
  const target = firstTarget();
  const params = { ...actionParams(action, target), ...formParams(body) };
  try {
    const result: unknown =
      (action.method ?? "GET") === "GET"
        ? await fetchDoc(
            props.connectionId,
            { routeId: action.routeId, params },
            targetContext(target),
          )
        : await runFormAction(
            props.connectionId,
            action.routeId,
            targetContext(target),
            {},
            params,
            action.method ?? "POST",
          );
    const raw =
      result && typeof result === "object"
        ? (result as Record<string, unknown>).url
        : undefined;
    if (typeof raw === "string" && raw) {
      window.open(raw, "_blank", "noopener,noreferrer");
    }
    pending.value = null;
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

function submitPending(action: Action, body?: Record<string, unknown>): void {
  if (action.open === "url") void openURL(action, body);
  else void execute(action, body);
}

async function execute(
  action: Action,
  body?: Record<string, unknown>,
): Promise<void> {
  busy.value = true;
  busyAction.value = action.id;
  error.value = null;
  const targets = actionTargets();
  try {
    if (targets.length > 1) {
      for (const target of targets) {
        await runFormAction(
          props.connectionId,
          action.routeId,
          targetContext(target),
          body ?? {},
          actionParams(action, target),
          action.method ?? "POST",
        );
      }
      pending.value = null;
      toast.add({
        severity: "success",
        summary: `${action.label}: ${targets.length} items`,
        life: 4000,
      });
      emit("done", action);
    } else {
      const target = targets[0] ?? firstTarget();
      const result = await runFormAction(
        props.connectionId,
        action.routeId,
        targetContext(target),
        body ?? {},
        actionParams(action, target),
        action.method ?? "POST",
      );
      pending.value = null;
      toast.add({
        severity: "success",
        summary: `${action.label} succeeded.`,
        life: 4000,
      });
      emit("done", action, result);
    }
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
    <template
      v-for="unit in layout.visible"
      :key="unit.kind === 'menu' ? unit.key : unit.action.id"
    >
      <Button
        v-if="unit.kind === 'button'"
        type="button"
        :disabled="!isEnabled(unit.action) || busyAction === unit.action.id"
        :title="unit.action.label"
        :aria-label="unit.action.label"
        size="small"
        :pt="{
          root: cn(
            'inline-flex min-w-0 items-center justify-center gap-1.5 rounded-md text-xs font-medium transition-colors disabled:pointer-events-none disabled:opacity-40',
            unit.action.iconOnly ? 'p-1.5' : 'px-2.5 py-1',
            riskClass[unit.action.risk],
          ),
        }"
        @click="isEnabled(unit.action) && trigger(unit.action)"
      >
        <AppIcon
          :icon="unit.action.icon"
          :size="unit.action.iconOnly ? 16 : 15"
          :loading="busyAction === unit.action.id"
        />
        <span v-if="!unit.action.iconOnly">{{ unit.action.label }}</span>
      </Button>

      <template v-else>
        <Button
          type="button"
          :title="unit.label"
          :aria-label="unit.label"
          aria-haspopup="true"
          size="small"
          :pt="{ root: cn(triggerClass) }"
          @click="toggleMenu(unit.key, $event)"
        >
          <span>{{ unit.label }}</span>
          <AppIcon
            :icon="{ type: 'lucide', value: 'chevron-down' }"
            :size="14"
          />
        </Button>
        <Menu
          :ref="(el) => setMenu(unit.key, el)"
          :model="menuModel(unit.actions)"
          popup
        >
          <template #item="{ item, props: ip }">
            <a
              v-bind="ip.action"
              class="flex items-center gap-2 px-3 py-1.5 text-xs"
              :class="[
                menuItemClass(item.action),
                item.disabled ? 'pointer-events-none opacity-40' : '',
              ]"
            >
              <AppIcon
                v-if="item.action.icon"
                :icon="item.action.icon"
                :size="15"
              />
              <span>{{ item.label }}</span>
            </a>
          </template>
        </Menu>
      </template>
    </template>

    <template v-if="layout.overflow.length">
      <Button
        type="button"
        title="More"
        aria-label="More actions"
        aria-haspopup="true"
        size="small"
        :pt="{ root: cn(triggerClass) }"
        @click="toggleMenu('__more__', $event)"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'ellipsis' }" :size="16" />
        <AppIcon :icon="{ type: 'lucide', value: 'chevron-down' }" :size="14" />
      </Button>
      <Menu
        :ref="(el) => setMenu('__more__', el)"
        :model="menuModel(layout.overflow)"
        popup
      >
        <template #item="{ item, props: ip }">
          <a
            v-bind="ip.action"
            class="flex items-center gap-2 px-3 py-1.5 text-xs"
            :class="[
              menuItemClass(item.action),
              item.disabled ? 'pointer-events-none opacity-40' : '',
            ]"
          >
            <AppIcon
              v-if="item.action.icon"
              :icon="item.action.icon"
              :size="15"
            />
            <span>{{ item.label }}</span>
          </a>
        </template>
      </Menu>
    </template>

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
        <p v-if="error" class="mb-3 text-sm text-red-500">{{ error }}</p>

        <SchemaForm
          v-if="pending.input"
          :schema="pending.input"
          :submit-label="pending.label"
          :busy="busy"
          :connection-id="connectionId"
          :resource="firstTarget().resource"
          :record="firstTarget().record"
          @submit="submitPending(pending, $event)"
        />

        <template v-else>
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
                  pending.risk === RiskLevel.Destructive
                    ? 'bg-rose-600'
                    : 'bg-primary-500',
                ),
              }"
              @click="submitPending(pending)"
            />
          </div>
        </template>
      </template>
    </Dialog>
  </div>
</template>
