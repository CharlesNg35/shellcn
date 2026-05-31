import { describe, it, expect } from "vitest";
import { collectField } from "./collect";
import type { Field } from "../../types/projection";

describe("collectField", () => {
  it("excludes a hidden nested sub-field", () => {
    const field: Field = {
      key: "opts",
      label: "Options",
      type: "object",
      fields: [
        { key: "mode", label: "Mode", type: "select" },
        {
          key: "extra",
          label: "Extra",
          type: "text",
          visibleWhen: { allOf: [{ field: "mode", op: "eq", value: "adv" }] },
        },
      ],
    };
    expect(collectField(field, { mode: "basic", extra: "x" })).toEqual({
      value: { mode: "basic" },
    });
  });
  it("collects a map field as an object, validating value items", () => {
    const m: Field = {
      key: "config",
      label: "Config",
      type: "map",
      item: { key: "v", label: "Value", type: "text" },
    };
    expect(collectField(m, { "retention.ms": "3600000" })).toEqual({
      value: { "retention.ms": "3600000" },
    });
    expect(collectField(m, {})).toEqual({ value: undefined });
    expect(collectField({ ...m, required: true }, {}).error).toContain(
      "required",
    );
  });

  it("validates array bounds and row required", () => {
    const arr: Field = {
      key: "c",
      label: "C",
      type: "array",
      itemLabel: "Col",
      minItems: 1,
      item: {
        key: "i",
        label: "Col",
        type: "object",
        fields: [{ key: "name", label: "Name", type: "text", required: true }],
      },
    };
    expect(collectField(arr, []).error).toContain("at least 1");
    expect(collectField(arr, [{}]).error).toContain("Col 1");
    expect(collectField(arr, [{ name: "id" }])).toEqual({
      value: [{ name: "id" }],
    });
  });
});
