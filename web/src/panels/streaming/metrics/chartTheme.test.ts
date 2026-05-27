import { describe, it, expect } from "vitest";
import { seriesColor, fade, axisStyle, metricPalette } from "./chartTheme";

describe("chartTheme", () => {
  it("cycles palette colors by index", () => {
    expect(seriesColor(0)).toBe(metricPalette[0]);
    expect(seriesColor(metricPalette.length)).toBe(metricPalette[0]);
  });

  it("fades a hex color to rgba", () => {
    expect(fade("#6366f1")).toBe("rgba(99, 102, 241, 0.15)");
    expect(fade("#10b981", 0.5)).toBe("rgba(16, 185, 129, 0.5)");
  });

  it("returns theme-appropriate axis colors", () => {
    expect(axisStyle(true).tick).not.toBe(axisStyle(false).tick);
    expect(axisStyle(true).grid).toContain("255,255,255");
  });
});
