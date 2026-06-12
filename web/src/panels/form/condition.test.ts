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
