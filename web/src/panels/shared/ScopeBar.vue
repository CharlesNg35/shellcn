<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import Select from "primevue/select";
import MultiSelect from "primevue/multiselect";
import InputText from "primevue/inputtext";
import ToggleSwitch from "primevue/toggleswitch";
import { fetchPage } from "@/api/dataSource";
import { SCOPE_SEPARATOR, useScopeStore } from "@/stores/scope";
import type { FilterOption, Row, ScopeFilter } from "@/types/projection";
import AppIcon from "@/components/AppIcon.vue";

const props = defineProps<{
  connectionId: string;
  scope: ScopeFilter[];
}>();

const store = useScopeStore();
const options = reactive<Record<string, FilterOption[]>>({});
const loading = ref(false);

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
  set(f, vs.join(SCOPE_SEPARATOR));
}

// Toggle on = the first option's value; off clears the scope.
function onValue(f: ScopeFilter): string {
  return f.options?.[0]?.value ?? loaded(f)[0]?.value ?? "";
}

async function loadOptions(): Promise<void> {
  loading.value = true;
  try {
    for (const f of props.scope) {
      if (f.options) {
        options[f.param] = f.options;
        ensureDefault(f);
        continue;
      }
      if (!f.optionsSource || !f.valueField) continue;
      const valueField = f.valueField;
      const labelField = f.labelField ?? valueField;
      const page = await fetchPage<Row>(props.connectionId, f.optionsSource);
      options[f.param] = page.items
        .map((r) => {
          const row = r as Record<string, unknown>;
          return {
            value: String(row[valueField] ?? ""),
            label: String(row[labelField] ?? row[valueField] ?? ""),
          };
        })
        .filter((o) => o.value);
      ensureDefault(f);
    }
  } finally {
    loading.value = false;
  }
}

watch(
  () => [props.connectionId, props.scope],
  () => {
    store.configure(props.connectionId, props.scope);
    void loadOptions();
  },
  { immediate: true },
);
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
        v-if="f.control === 'toggle'"
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
          v-if="f.control === 'search'"
          :model-value="value(f)"
          :placeholder="f.allLabel ?? f.label"
          :aria-label="f.label"
          class="w-full"
          @update:model-value="set(f, $event ?? '')"
        />

        <MultiSelect
          v-else-if="f.control === 'multiselect'"
          :model-value="members(f)"
          :options="loaded(f)"
          option-label="label"
          option-value="value"
          :placeholder="f.allLabel ?? f.label"
          :loading="loading"
          :aria-label="f.label"
          @update:model-value="setMembers(f, $event)"
        />

        <Select
          v-else
          :model-value="value(f)"
          :options="choices(f)"
          option-label="label"
          option-value="value"
          :placeholder="f.allLabel ?? f.label"
          :loading="loading"
          :aria-label="f.label"
          @update:model-value="set(f, $event ?? '')"
        />
      </div>
    </div>
  </div>
</template>
