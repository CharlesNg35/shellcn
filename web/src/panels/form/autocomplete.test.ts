import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import AutoComplete from "primevue/autocomplete";
import FormField from "./FormField.vue";
import type { Field } from "@/types/projection";

const typeField: Field = {
  key: "type",
  label: "Type",
  type: "autocomplete",
  placeholder: "bigserial",
  options: [
    { label: "bigint", value: "bigint" },
    { label: "text", value: "text" },
    { label: "timestamptz", value: "timestamptz" },
  ],
};

describe("autocomplete field", () => {
  it("renders an AutoComplete and allows a free-typed custom value", async () => {
    const w = mount(FormField, {
      props: { field: typeField, modelValue: "" },
    });
    const ac = w.findComponent(AutoComplete);
    expect(ac.exists()).toBe(true);

    await w.find("input").setValue("numeric(10,2)");
    expect(w.emitted("update:modelValue")?.at(-1)?.[0]).toBe("numeric(10,2)");
  });

  it("filters suggestions by query and shows all on an empty query", async () => {
    const w = mount(FormField, {
      props: { field: typeField, modelValue: "" },
    });
    const ac = w.findComponent(AutoComplete);
    ac.vm.$emit("complete", { query: "time" });
    await w.vm.$nextTick();
    expect(ac.props("suggestions")).toEqual(["timestamptz"]);

    ac.vm.$emit("complete", { query: "" });
    await w.vm.$nextTick();
    expect(ac.props("suggestions")).toEqual(["bigint", "text", "timestamptz"]);
  });
});
