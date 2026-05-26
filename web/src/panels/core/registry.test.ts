import { describe, it, expect } from "vitest";
import { resolvePanel } from "./registry";
import type { KnownPanelType } from "../../types/projection";

describe("panel registry", () => {
  it("maps every known panel type to a component", () => {
    const types: KnownPanelType[] = [
      "table",
      "form",
      "enroll",
      "file_browser",
      "document",
      "terminal",
      "log_stream",
      "metrics",
      "code_editor",
      "query_editor",
      "remote_desktop",
      "graph",
      "trace",
      "kv",
      "http_client",
    ];
    for (const t of types) {
      expect(resolvePanel(t), `panel ${t}`).toBeTruthy();
    }
  });

  it("returns undefined for an unknown panel type (renderer falls back)", () => {
    expect(resolvePanel("totally-made-up")).toBeUndefined();
  });
});
