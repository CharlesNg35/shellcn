import { describe, it, expect } from "vitest";
import { panelRegistry, resolvePanel } from "./registry";
import { KNOWN_PANEL_TYPES } from "../../types/projection";

describe("panel registry", () => {
  it("maps every known panel type to a component", () => {
    for (const t of KNOWN_PANEL_TYPES) {
      expect(resolvePanel(t), `panel ${t}`).toBeTruthy();
    }
  });

  it("does not register panels missing from the projection type contract", () => {
    expect(Object.keys(panelRegistry).sort()).toEqual(
      [...KNOWN_PANEL_TYPES].sort(),
    );
  });

  it("returns undefined for an unknown panel type (renderer falls back)", () => {
    expect(resolvePanel("totally-made-up")).toBeUndefined();
  });
});
