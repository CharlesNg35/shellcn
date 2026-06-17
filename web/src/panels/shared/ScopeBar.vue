<script setup lang="ts">
import { onUnmounted, reactive, ref, watch } from "vue";
import Select from "primevue/select";
import MultiSelect from "primevue/multiselect";
import AutoComplete from "primevue/autocomplete";
import InputText from "primevue/inputtext";
import ToggleSwitch from "primevue/toggleswitch";
import { fetchPage, watch as watchResource } from "@/api/dataSource";
import { SCOPE_SEPARATOR, useScopeStore } from "@/stores/scope";
import type {
  FilterOption,
  ResourceEvent,
  Row,
  ScopeFilter,
} from "@/types/projection";
import { EventType, ScopeControl } from "@/types/projection";
import AppIcon from "@/components/AppIcon.vue";

const props = defineProps<{
  connectionId: string;
  scope: ScopeFilter[];
}>();

const store = useScopeStore();
const options = reactive<Record<string, FilterOption[]>>({});
const suggestions = reactive<Record<string, FilterOption[]>>({});
const loading = ref(false);
const scopeControlOverlayStyle = {
  width: "13rem",
  maxWidth: "calc(100vw - 2rem)",
};
let stops: (() => void)[] = [];
let loadVersion = 0;

function loaded(f: ScopeFilter): FilterOption[] {
  return options[f.param] ?? [];
}

// The clear-to-default choice exists only when the manifest labels it.
function choices(f: ScopeFilter): FilterOption[] {
  return f.allLabel
    ? [{ value: "", label: f.allLabel }, ...loaded(f)]
    : loaded(f);
}

function value(f: ScopeFilter): string {
  return store.params(props.connectionId)[f.param] ?? "";
}

function set(f: ScopeFilter, v: string): void {
  store.set(props.connectionId, f.param, v);
}

function multiple(f: ScopeFilter): boolean {
  return f.multiple === true;
}

function searchable(f: ScopeFilter): boolean {
  return f.searchable !== false;
}

function ensureDefault(f: ScopeFilter): void {
  if (!f.defaultValue || value(f)) return;
  if (choices(f).some((option) => option.value === f.defaultValue)) {
    set(f, f.defaultValue);
  }
}

// Members ride in one param string, joined by the shared scope separator.
function members(f: ScopeFilter): string[] {
  const v = value(f);
  return v ? v.split(SCOPE_SEPARATOR).filter(Boolean) : [];
}

function setMembers(f: ScopeFilter, vs: string[]): void {
  set(f, [...new Set(vs)].join(SCOPE_SEPARATOR));
}

// Toggle on = the first option's value; off clears the scope.
function onValue(f: ScopeFilter): string {
  return f.options?.[0]?.value ?? loaded(f)[0]?.value ?? "";
}

function optionFromRow(f: ScopeFilter, row: unknown): FilterOption | null {
  if (!f.valueField || !row || typeof row !== "object") return null;
  const record = row as Record<string, unknown>;
  const value = String(record[f.valueField] ?? "");
  if (!value) return null;
  const labelField = f.labelField ?? f.valueField;
  return {
    value,
    label: String(record[labelField] ?? record[f.valueField] ?? value),
  };
}

function setOptions(f: ScopeFilter, rows: Row[]): void {
  options[f.param] = rows
    .map((row) => optionFromRow(f, row))
    .filter((option): option is FilterOption => Boolean(option));
  suggestions[f.param] = choicesForControl(f);
  ensureDefault(f);
}

async function loadOptions(): Promise<void> {
  const version = ++loadVersion;
  loading.value = true;
  try {
    for (const f of props.scope) {
      if (f.options) {
        if (version !== loadVersion) return;
        options[f.param] = f.options;
        suggestions[f.param] = choicesForControl(f);
        ensureDefault(f);
        continue;
      }
      if (!f.optionsSource || !f.valueField) continue;
      const page = await fetchPage<Row>(props.connectionId, f.optionsSource);
      if (version !== loadVersion) return;
      setOptions(f, page.items);
    }
  } finally {
    if (version === loadVersion) loading.value = false;
  }
}

function applyOptionEvent(f: ScopeFilter, ev: ResourceEvent): boolean {
  const value =
    optionFromRow(f, ev.resource)?.value ??
    (ev.ref.name ? String(ev.ref.name) : "");
  if (!value) return false;
  const current = loaded(f);
  const index = current.findIndex((option) => option.value === value);
  const type = String(ev.type).toLowerCase();
  if (type === EventType.Deleted) {
    if (index === -1) return true;
    options[f.param] = current.filter((_, i) => i !== index);
    if (multiple(f)) {
      const next = members(f).filter((member) => member !== value);
      if (next.length !== members(f).length) setMembers(f, next);
    } else if (value === valueForFilter(f)) {
      set(f, "");
    }
    return true;
  }
  const option = optionFromRow(f, ev.resource);
  if (!option) return false;
  if (index === -1) options[f.param] = [...current, option];
  else
    options[f.param] = current.map((existing, i) =>
      i === index ? option : existing,
    );
  ensureDefault(f);
  return true;
}

function choicesForControl(f: ScopeFilter): FilterOption[] {
  return multiple(f) ? loaded(f) : choices(f);
}

function selectedOption(f: ScopeFilter): FilterOption | null {
  const v = value(f);
  if (!v) return null;
  return (
    choicesForControl(f).find((option) => option.value === v) ??
    (f.allowCustom ? { value: v, label: v } : null)
  );
}

function selectedOptions(f: ScopeFilter): FilterOption[] {
  return members(f)
    .map(
      (member) =>
        loaded(f).find((option) => option.value === member) ??
        (f.allowCustom ? { value: member, label: member } : null),
    )
    .filter((option): option is FilterOption => Boolean(option));
}

function optionValue(f: ScopeFilter, raw: unknown): string | null {
  if (raw == null) return "";
  if (typeof raw === "string") return f.allowCustom ? raw.trim() : null;
  if (typeof raw !== "object") return null;
  const value = (raw as Partial<FilterOption>).value;
  return typeof value === "string" ? value : null;
}

function autoCompleteValue(
  f: ScopeFilter,
): FilterOption | FilterOption[] | null {
  return multiple(f) ? selectedOptions(f) : selectedOption(f);
}

function setAutoCompleteValue(f: ScopeFilter, raw: unknown): void {
  if (multiple(f)) {
    if (!Array.isArray(raw)) return;
    const next = raw
      .map((item) => optionValue(f, item))
      .filter((item): item is string => item !== null && item !== "");
    setMembers(f, next);
    return;
  }
  const next = optionValue(f, raw);
  if (next !== null) set(f, next);
}

function complete(f: ScopeFilter, event: { query?: string }): void {
  const query = (event.query ?? "").trim().toLowerCase();
  const source = choicesForControl(f);
  suggestions[f.param] = query
    ? source.filter((option) =>
        `${option.label ?? option.value} ${option.value}`
          .toLowerCase()
          .includes(query),
      )
    : source;
}

function valueForFilter(f: ScopeFilter): string {
  return value(f);
}

function stopWatches(): void {
  for (const stop of stops) stop();
  stops = [];
}

function startWatches(): void {
  stopWatches();
  for (const f of props.scope) {
    if (!f.watchSource) continue;
    stops.push(
      watchResource(props.connectionId, f.watchSource, {}, (ev) => {
        if (!applyOptionEvent(f, ev)) void loadOptions();
      }),
    );
  }
}

watch(
  () => [props.connectionId, props.scope],
  () => {
    store.configure(props.connectionId, props.scope);
    void loadOptions();
    startWatches();
  },
  { immediate: true },
);

onUnmounted(stopWatches);
</script>

<template>
  <div class="flex items-center gap-2">
    <div v-for="f in scope" :key="f.param" class="flex items-center gap-1.5">
      <AppIcon
        v-if="f.icon"
        :icon="f.icon"
        :size="15"
        class="shrink-0 text-surface-400"
      />

      <label
        v-if="f.control === ScopeControl.Toggle"
        class="flex items-center gap-1.5 text-sm text-surface-600 dark:text-surface-300"
      >
        {{ f.options?.[0]?.label ?? f.label }}
        <ToggleSwitch
          :model-value="value(f) !== '' && value(f) === onValue(f)"
          :aria-label="f.label"
          @update:model-value="set(f, $event ? onValue(f) : '')"
        />
      </label>

      <div v-else class="w-52">
        <InputText
          v-if="f.control === ScopeControl.Search"
          :model-value="value(f)"
          :placeholder="f.allLabel ?? f.label"
          :aria-label="f.label"
          class="w-full"
          @update:model-value="set(f, $event ?? '')"
        />

        <MultiSelect
          v-else-if="multiple(f) && f.control !== ScopeControl.AutoComplete"
          :model-value="members(f)"
          :options="loaded(f)"
          option-label="label"
          option-value="value"
          :filter="searchable(f)"
          :placeholder="f.allLabel ?? f.label"
          :loading="loading"
          :overlay-style="scopeControlOverlayStyle"
          :aria-label="f.label"
          @update:model-value="setMembers(f, $event)"
        />

        <AutoComplete
          v-else-if="f.control === ScopeControl.AutoComplete"
          :model-value="autoCompleteValue(f)"
          :suggestions="suggestions[f.param] ?? choicesForControl(f)"
          option-label="label"
          dropdown
          complete-on-focus
          :multiple="multiple(f)"
          :force-selection="!f.allowCustom"
          :placeholder="f.allLabel ?? f.label"
          :loading="loading"
          :aria-label="f.label"
          @complete="complete(f, $event)"
          @update:model-value="setAutoCompleteValue(f, $event)"
        />

        <Select
          v-else
          :model-value="value(f)"
          :options="choices(f)"
          option-label="label"
          option-value="value"
          :filter="searchable(f)"
          :placeholder="f.allLabel ?? f.label"
          :loading="loading"
          :overlay-style="scopeControlOverlayStyle"
          :aria-label="f.label"
          @update:model-value="set(f, $event ?? '')"
        />
      </div>
    </div>
  </div>
</template>
