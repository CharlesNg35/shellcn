import { describe, it, expect } from "vitest";
import { isVisible, validateField } from "./condition";
import type { Field } from "@/types/projection";

describe("form condition evaluation", () => {
  it("shows/hides via allOf and anyOf", () => {
    const allOf = {
      allOf: [{ field: "auth", op: "eq" as const, value: "password" }],
    };
    expect(isVisible(allOf, { auth: "password" })).toBe(true);
    expect(isVisible(allOf, { auth: "private_key" })).toBe(false);

    const anyOf = {
      anyOf: [
        { field: "auth", op: "eq" as const, value: "private_key" },
        { field: "auth", op: "eq" as const, value: "credential" },
      ],
    };
    expect(isVisible(anyOf, { auth: "credential" })).toBe(true);
    expect(isVisible(anyOf, { auth: "password" })).toBe(false);

    expect(isVisible(undefined, {})).toBe(true);
  });

  it("resolves dotted-path fields and keeps flat keys working", () => {
    const cond = {
      allOf: [{ field: "can.delete", op: "eq" as const, value: true }],
    };
    expect(isVisible(cond, { can: { delete: true } })).toBe(true);
    expect(isVisible(cond, { can: { delete: false } })).toBe(false);
    // missing nested path → rule fails (not a match)
    expect(isVisible(cond, { can: {} })).toBe(false);
    // an exact flat key still wins over path-walking
    expect(isVisible(cond, { "can.delete": true })).toBe(true);
    // unrelated flat condition unaffected
    expect(
      isVisible(
        { allOf: [{ field: "status", op: "eq" as const, value: "Running" }] },
        { status: "Running" },
      ),
    ).toBe(true);
  });

  it("supports empty/notEmpty/in operators", () => {
    expect(
      isVisible({ allOf: [{ field: "tls", op: "notEmpty" }] }, { tls: true }),
    ).toBe(true);
    expect(isVisible({ allOf: [{ field: "x", op: "empty" }] }, { x: "" })).toBe(
      true,
    );
    expect(
      isVisible(
        { allOf: [{ field: "k", op: "in", value: ["a", "b"] }] },
        { k: "b" },
      ),
    ).toBe(true);
  });

  it("supports numeric and contains operators", () => {
    expect(
      isVisible({ allOf: [{ field: "n", op: "gt", value: 3 }] }, { n: 5 }),
    ).toBe(true);
    expect(
      isVisible({ allOf: [{ field: "n", op: "lte", value: 3 }] }, { n: "3" }),
    ).toBe(true);
    expect(
      isVisible(
        { allOf: [{ field: "s", op: "contains", value: "ay" }] },
        { s: "gateway" },
      ),
    ).toBe(true);
    expect(
      isVisible(
        { allOf: [{ field: "list", op: "contains", value: "x" }] },
        { list: ["x", "y"] },
      ),
    ).toBe(true);
  });

  it("composes nested all/any/not groups", () => {
    // (A and B) or (C and not D)
    const cond = {
      any: [
        {
          allOf: [
            { field: "a", op: "eq" as const, value: 1 },
            { field: "b", op: "eq" as const, value: 1 },
          ],
        },
        {
          allOf: [{ field: "c", op: "eq" as const, value: 1 }],
          not: { allOf: [{ field: "d", op: "eq" as const, value: 1 }] },
        },
      ],
    };
    expect(isVisible(cond, { a: 1, b: 1 })).toBe(true);
    expect(isVisible(cond, { c: 1, d: 0 })).toBe(true);
    expect(isVisible(cond, { c: 1, d: 1 })).toBe(false);
    expect(isVisible(cond, { a: 1, b: 0 })).toBe(false);
  });
});

describe("field validation", () => {
  const port: Field = {
    key: "port",
    label: "Port",
    type: "number",
    validators: [
      { type: "min", value: 1 },
      { type: "max", value: 65535 },
    ],
  };

  it("enforces required", () => {
    const host: Field = {
      key: "host",
      label: "Host",
      type: "text",
      required: true,
    };
    expect(validateField(host, "")).toMatch(/required/);
    expect(validateField(host, "h")).toBeNull();
  });

  it("enforces min/max and skips when empty/optional", () => {
    expect(validateField(port, 0)).toMatch(/at least/);
    expect(validateField(port, 70000)).toMatch(/at most/);
    expect(validateField(port, 22)).toBeNull();
    expect(validateField(port, "")).toBeNull();
  });

  it("uses a custom message and regex", () => {
    const f: Field = {
      key: "name",
      label: "Name",
      type: "text",
      validators: [
        { type: "regex", value: "^[a-z]+$", message: "lowercase only" },
      ],
    };
    expect(validateField(f, "ABC")).toBe("lowercase only");
    expect(validateField(f, "abc")).toBeNull();
  });
});
