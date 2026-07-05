import { describe, expect, it } from "vitest";
import { ColumnType } from "@/types/projection";
import { formatValue } from "./objectDetailFormat";

describe("objectDetailFormat", () => {
  it("formats duration values from seconds", () => {
    expect(formatValue(3_093_695, ColumnType.Duration)).toBe("35d 19h");
  });
});
