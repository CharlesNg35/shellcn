import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import ArrayField from "./ArrayField.vue";
import type { Field } from "../../types/projection";

const objectItem: Field = {
  key: "columns",
  label: "Columns",
  type: "array",
  itemLabel: "Column",
  item: {
    key: "col",
    label: "Column",
    type: "object",
    fields: [
      { key: "name", label: "Name", type: "text" },
      { key: "nullable", label: "Nullable", type: "toggle", default: false },
    ],
  },
};

function addBtn(w: ReturnType<typeof mount>) {
  return w.findAll("button").find((b) => b.text().includes("Add Column"));
}

describe("ArrayField", () => {
  it("appends a default-seeded row on add", async () => {
    const w = mount(ArrayField, {
      props: { field: objectItem, modelValue: [] },
    });
    await addBtn(w)!.trigger("click");
    const emitted = w.emitted("update:modelValue")?.at(-1)?.[0];
    expect(emitted).toEqual([{ nullable: false }]);
  });

  it("removes a row by index", async () => {
    const w = mount(ArrayField, {
      props: { field: objectItem, modelValue: [{ name: "a" }, { name: "b" }] },
    });
    const remove = w
      .findAll("button")
      .find((b) => b.attributes("aria-label") === "Remove Column 1")!;
    await remove.trigger("click");
    expect(w.emitted("update:modelValue")?.at(-1)?.[0]).toEqual([
      { name: "b" },
    ]);
  });

  it("disables remove at minItems and add at maxItems", async () => {
    const bounded: Field = { ...objectItem, minItems: 1, maxItems: 2 };
    const w = mount(ArrayField, {
      props: { field: bounded, modelValue: [{ name: "a" }, { name: "b" }] },
    });
    expect((addBtn(w)!.element as HTMLButtonElement).disabled).toBe(true);
    const remove = w
      .findAll("button")
      .find((b) => b.attributes("aria-label") === "Remove Column 1")!;
    expect((remove.element as HTMLButtonElement).disabled).toBe(false);
  });

  it("updates a row's sub-field value", async () => {
    const w = mount(ArrayField, {
      props: { field: objectItem, modelValue: [{ name: "" }] },
    });
    await w.find('input[type="text"]').setValue("id");
    expect(w.emitted("update:modelValue")?.at(-1)?.[0]).toEqual([
      { name: "id" },
    ]);
  });
});
