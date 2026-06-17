import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ScopeBar from "./ScopeBar.vue";
import { SCOPE_SEPARATOR, useScopeStore } from "@/stores/scope";
import type { FilterOption, ResourceEvent } from "@/types/projection";

const data = vi.hoisted(() => ({
  fetchPage: vi.fn(),
  watch: vi.fn(),
}));

vi.mock("@/api/dataSource", () => ({
  fetchPage: data.fetchPage,
  watch: data.watch,
}));

function selectOptions(wrapper: ReturnType<typeof mount>): FilterOption[] {
  return wrapper
    .findComponent({ name: "Select" })
    .props("options") as FilterOption[];
}

describe("ScopeBar", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    data.fetchPage.mockReset();
    data.watch.mockReset();
    data.watch.mockReturnValue(vi.fn());
  });

  it("patches route-sourced scope options from watch events", async () => {
    let onEvent: ((event: ResourceEvent) => void) | undefined;
    data.fetchPage.mockResolvedValue({
      items: [{ name: "default" }],
      nextCursor: "",
      total: 1,
    });
    data.watch.mockImplementation((_connectionId, _source, _ctx, cb) => {
      onEvent = cb;
      return vi.fn();
    });

    const wrapper = mount(ScopeBar, {
      props: {
        connectionId: "c1",
        scope: [
          {
            param: "environment",
            label: "Environment",
            control: "select",
            optionsSource: {
              routeId: "sample.scope.options",
              params: { kind: "environment" },
            },
            watchSource: {
              routeId: "sample.scope.watch",
              method: "WS",
              params: { kind: "environment" },
            },
            valueField: "name",
            labelField: "name",
            allLabel: "All environments",
          },
        ],
      },
    });
    await flushPromises();

    expect(selectOptions(wrapper).map((option) => option.value)).toEqual([
      "",
      "default",
    ]);
    expect(data.watch).toHaveBeenCalledWith(
      "c1",
      {
        routeId: "sample.scope.watch",
        method: "WS",
        params: { kind: "environment" },
      },
      {},
      expect.any(Function),
    );

    onEvent?.({
      type: "added",
      ref: { kind: "environment", name: "ops", uid: "env-ops" },
      resource: { name: "ops" },
    });
    await flushPromises();

    expect(selectOptions(wrapper).map((option) => option.value)).toEqual([
      "",
      "default",
      "ops",
    ]);

    onEvent?.({
      type: "deleted",
      ref: { kind: "environment", name: "default", uid: "env-default" },
    });
    await flushPromises();

    expect(selectOptions(wrapper).map((option) => option.value)).toEqual([
      "",
      "ops",
    ]);
  });

  it("uses autocomplete as selection-only input unless custom values are allowed", async () => {
    const wrapper = mount(ScopeBar, {
      props: {
        connectionId: "c1",
        scope: [
          {
            param: "environment",
            label: "Environment",
            control: "autocomplete",
            options: [
              { value: "default", label: "default" },
              { value: "ops", label: "ops" },
            ],
          },
        ],
      },
    });
    await flushPromises();

    const autocomplete = wrapper.findComponent({ name: "AutoComplete" });
    expect(autocomplete.props("forceSelection")).toBe(true);
    expect(autocomplete.props("multiple")).toBe(false);

    await autocomplete.vm.$emit("update:modelValue", "typed");
    expect(useScopeStore().params("c1")).toEqual({});

    await autocomplete.vm.$emit("update:modelValue", {
      value: "ops",
      label: "ops",
    });
    expect(useScopeStore().params("c1")).toEqual({ environment: "ops" });
  });

  it("keeps select overlays aligned to the compact scope control width", async () => {
    const wrapper = mount(ScopeBar, {
      props: {
        connectionId: "c1",
        scope: [
          {
            param: "database",
            label: "Database",
            control: "select",
            options: [
              { value: "0", label: "0" },
              {
                value: "very-long-database-name",
                label: "very-long-database-name-that-should-truncate",
              },
            ],
          },
        ],
      },
    });
    await flushPromises();

    expect(
      wrapper.findComponent({ name: "Select" }).props("overlayStyle"),
    ).toEqual({
      width: "13rem",
      maxWidth: "calc(100vw - 2rem)",
    });
  });

  it("keeps multiselect overlays aligned to the compact scope control width", async () => {
    const wrapper = mount(ScopeBar, {
      props: {
        connectionId: "c1",
        scope: [
          {
            param: "database",
            label: "Database",
            control: "select",
            multiple: true,
            options: [
              { value: "0", label: "0" },
              {
                value: "very-long-database-name",
                label: "very-long-database-name-that-should-truncate",
              },
            ],
          },
        ],
      },
    });
    await flushPromises();

    expect(
      wrapper.findComponent({ name: "MultiSelect" }).props("overlayStyle"),
    ).toEqual({
      width: "13rem",
      maxWidth: "calc(100vw - 2rem)",
    });
  });

  it("supports multiple autocomplete scopes without a separate control name", async () => {
    const wrapper = mount(ScopeBar, {
      props: {
        connectionId: "c1",
        scope: [
          {
            param: "environment",
            label: "Environment",
            control: "autocomplete",
            multiple: true,
            options: [
              { value: "default", label: "default" },
              { value: "ops", label: "ops" },
            ],
          },
        ],
      },
    });
    await flushPromises();

    const autocomplete = wrapper.findComponent({ name: "AutoComplete" });
    expect(autocomplete.props("multiple")).toBe(true);

    await autocomplete.vm.$emit("update:modelValue", [
      { value: "default", label: "default" },
      { value: "ops", label: "ops" },
    ]);
    expect(useScopeStore().params("c1")).toEqual({
      environment: ["default", "ops"].join(SCOPE_SEPARATOR),
    });
  });
});
