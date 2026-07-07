<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { useDebounceFn } from "@vueuse/core";
import Button from "primevue/button";
import DatePicker from "primevue/datepicker";
import IconField from "primevue/iconfield";
import InputIcon from "primevue/inputicon";
import InputText from "primevue/inputtext";
import Popover from "primevue/popover";
import Select from "primevue/select";
import Tooltip from "primevue/tooltip";
import AppIcon from "../AppIcon.vue";
import {
  searchFieldClass,
  searchIconLeftClass,
  searchInputClass,
} from "@/primevue/preset";
import type { AuditFilters } from "@/types/projection";
import {
  activeAuditFilterCount,
  auditResultOptions,
  auditRiskOptions,
  buildAuditFilters,
  createAuditFilterDraft,
  resetAuditFilterDraft,
  syncAuditFilterDraft,
} from "./auditFilters";

type PopoverRef = {
  toggle: (event: Event) => void;
  hide: () => void;
};

const props = withDefaults(
  defineProps<{
    filters?: AuditFilters;
  }>(),
  { filters: () => ({}) },
);

const emit = defineEmits<{
  apply: [filters: AuditFilters];
}>();

const draft = reactive(createAuditFilterDraft());
const filterPopover = ref<PopoverRef | null>(null);
const vTooltip = Tooltip;

const activeCount = computed(() => activeAuditFilterCount(props.filters));
const hiddenFilterCount = computed(
  () =>
    Number(Boolean(props.filters.remoteAddr)) +
    Number(Boolean(props.filters.since || props.filters.until)),
);
const moreFilterLabel = computed(() =>
  hiddenFilterCount.value ? `More (${hiddenFilterCount.value})` : "More",
);
const clearFiltersTooltip = { value: "Clear filters", showDelay: 300 };

function toggleFilters(event: Event): void {
  filterPopover.value?.toggle(event);
}

function applyFilters(closePopover = true): void {
  emit("apply", buildAuditFilters(draft));
  if (closePopover) filterPopover.value?.hide();
}

const applyDebouncedFilters = useDebounceFn(() => applyFilters(false), 300);

function updateEvent(value: string | undefined): void {
  draft.event = value ?? "";
  applyDebouncedFilters();
}

function updateResult(value: string | null): void {
  draft.result = value;
  applyFilters(false);
}

function updateRisk(value: string | null): void {
  draft.risk = value;
  applyFilters(false);
}

function clearFilters(): void {
  resetAuditFilterDraft(draft);
  emit("apply", {});
  filterPopover.value?.hide();
}

watch(
  () => props.filters,
  (filters) => {
    syncAuditFilterDraft(draft, filters);
  },
  { deep: true, immediate: true },
);
</script>

<template>
  <div
    data-testid="audit-filter-bar"
    class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center sm:justify-between"
  >
    <IconField :class="[searchFieldClass, 'sm:w-72 lg:w-80']">
      <InputIcon :class="searchIconLeftClass">
        <AppIcon :icon="{ type: 'lucide', value: 'search' }" :size="14" />
      </InputIcon>
      <InputText
        :model-value="draft.event"
        type="search"
        :class="[searchInputClass, 'h-9']"
        placeholder="Search events"
        aria-label="Search audit events"
        @update:model-value="updateEvent"
        @keydown.enter="applyFilters()"
      />
    </IconField>

    <div class="grid min-w-0 grid-cols-2 gap-2 sm:flex sm:items-center">
      <Select
        :model-value="draft.result"
        :options="auditResultOptions"
        option-label="label"
        option-value="value"
        placeholder="Result"
        show-clear
        size="small"
        class="min-w-0 sm:w-32"
        @update:model-value="updateResult"
      />
      <Select
        :model-value="draft.risk"
        :options="auditRiskOptions"
        option-label="label"
        option-value="value"
        placeholder="Risk"
        show-clear
        size="small"
        class="min-w-0 sm:w-36"
        @update:model-value="updateRisk"
      />
      <Button
        type="button"
        severity="secondary"
        variant="outlined"
        size="small"
        class="min-w-0"
        @click="toggleFilters"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'filter' }" :size="14" />
        {{ moreFilterLabel }}
      </Button>
      <Button
        v-if="activeCount"
        v-tooltip.bottom="clearFiltersTooltip"
        type="button"
        aria-label="Clear filters"
        severity="secondary"
        variant="text"
        rounded
        size="small"
        @click="clearFilters"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="14" />
      </Button>
    </div>
  </div>

  <Popover ref="filterPopover" aria-label="More audit filters">
    <div class="grid w-[min(24rem,calc(100vw-2rem))] gap-4">
      <div class="grid gap-1.5">
        <label
          for="audit-date-range"
          class="text-xs font-medium text-surface-500 dark:text-surface-400"
        >
          Date range
        </label>
        <DatePicker
          v-model="draft.dateRange"
          input-id="audit-date-range"
          selection-mode="range"
          :manual-input="false"
          show-button-bar
          show-icon
          size="small"
          placeholder="Any date"
        />
      </div>

      <div class="grid gap-1.5">
        <label
          for="audit-source-ip"
          class="text-xs font-medium text-surface-500 dark:text-surface-400"
        >
          Source IP
        </label>
        <InputText
          id="audit-source-ip"
          v-model="draft.remoteAddr"
          size="small"
          placeholder="Any source"
          @keydown.enter="applyFilters()"
        />
      </div>

      <div class="flex justify-end gap-2 pt-1">
        <Button
          label="Reset"
          severity="secondary"
          variant="text"
          size="small"
          @click="clearFilters"
        />
        <Button label="Apply" size="small" @click="applyFilters()" />
      </div>
    </div>
  </Popover>
</template>
