import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import MapField from "./MapField.vue";
import type { Field } from "@/types/projection";

const configField: Field = {
  key: "config",
  label: "Config",
  type: "map",
  keyPlaceholder: "retention.ms",
  item: { key: "v", label: "Value", type: "text" },
};

function addBtn(w: ReturnType<typeof mount>) {
  return w.findAll("button").find((b) => b.text().includes("Add entry"));
}

describe("MapField", () => {
  it("renders existing entries from the object value", () => {
    const w = mount(MapField, {
      props: { field: configField, modelValue: { "retention.ms": "1000" } },
    });
    const inputs = w.findAll("input");
    expect((inputs[0].element as HTMLInputElement).value).toBe("retention.ms");
    expect((inputs[1].element as HTMLInputElement).value).toBe("1000");
  });

  it("adds a row and emits an object once key and value are set", async () => {
    const w = mount(MapField, {
      props: { field: configField, modelValue: {} },
    });
    await addBtn(w)!.trigger("click");
    const inputs = w.findAll("input");
    await inputs[0].setValue("cleanup.policy");
    await inputs[1].setValue("compact");
    expect(w.emitted("update:modelValue")?.at(-1)?.[0]).toEqual({
      "cleanup.policy": "compact",
    });
  });

  it("removes an entry", async () => {
    const w = mount(MapField, {
      props: { field: configField, modelValue: { a: "1", b: "2" } },
    });
    const remove = w
      .findAll("button")
      .find((b) => b.attributes("aria-label") === "Remove entry 1")!;
    await remove.trigger("click");
    expect(w.emitted("update:modelValue")?.at(-1)?.[0]).toEqual({ b: "2" });
  });
});
