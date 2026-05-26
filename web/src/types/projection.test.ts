import { describe, it, expect } from "vitest";
import projectionSrc from "./projection.ts?raw";
import type {
  CredentialSummary,
  Icon,
  IconType,
  PanelType,
  PluginProjection,
} from "./projection";

describe("projection contract", () => {
  it("accepts a representative projection (type-level)", () => {
    const projection: PluginProjection = {
      apiVersion: 1,
      name: "ssh",
      version: "0.1.0",
      title: "SSH",
      description: "Secure Shell",
      icon: { type: "lucide", value: "terminal" },
      config: {
        groups: [
          {
            name: "Auth",
            fields: [
              {
                key: "credentialId",
                label: "Saved credential",
                type: "credential_ref",
                credential: {
                  kinds: ["ssh_private_key", "ssh_password"],
                  protocols: ["ssh"],
                },
              },
              {
                key: "password",
                label: "Password",
                type: "password",
                secret: true,
                visibleWhen: {
                  allOf: [{ field: "auth", op: "eq", value: "password" }],
                },
              },
            ],
          },
        ],
      },
      capabilities: ["terminal", "filesystem"],
      supportedTransports: ["direct"],
      layout: "tabs",
      tabs: [
        {
          key: "shell",
          label: "Terminal",
          panel: "terminal",
          source: { routeId: "ssh.shell", method: "WS" },
        },
      ],
      streams: [{ id: "ssh.shell", kind: "terminal", routeId: "ssh.shell" }],
    };
    expect(projection.name).toBe("ssh");
  });

  it("narrows Icon by its discriminant", () => {
    const render = (icon: Icon): string => {
      switch (icon.type) {
        case "lucide":
          return `glyph:${icon.value}`;
        case "url":
        case "base64":
          return `img:${icon.value}`;
        case "emoji":
          return icon.value;
        case "svg":
          return `svg:${icon.value}`;
        default: {
          const _exhaustive: never = icon.type;
          return _exhaustive;
        }
      }
    };
    const cases: Array<[IconType, string]> = [
      ["lucide", "glyph:db"],
      ["url", "img:https://x/i.svg"],
      ["base64", "img:data:image/svg+xml;base64,AAA"],
      ["emoji", "🐳"],
      ["svg", "svg:<svg/>"],
    ];
    expect(render({ type: "lucide", value: "db" })).toBe("glyph:db");
    expect(render({ type: "url", value: "https://x/i.svg" })).toBe(
      "img:https://x/i.svg",
    );
    expect(
      render({ type: "base64", value: "data:image/svg+xml;base64,AAA" }),
    ).toBe("img:data:image/svg+xml;base64,AAA");
    expect(render({ type: "emoji", value: "🐳" })).toBe("🐳");
    expect(render({ type: "svg", value: "<svg/>" })).toBe("svg:<svg/>");
    expect(cases).toHaveLength(5);
  });

  it("models reusable credential summaries without secret values", () => {
    const credential: CredentialSummary = {
      id: "cred-prod-key",
      name: "Production deploy key",
      kind: "ssh_private_key",
      identity: "deploy",
      protocols: ["ssh"],
    };

    expect(Object.keys(credential)).not.toContain("value");
    expect(Object.keys(credential)).not.toContain("secret");
  });

  it("permits an unrecognized PanelType without a type error", () => {
    const known: PanelType = "table";
    const specialized: PanelType = "graph";
    const unknown: PanelType = "something-a-plugin-invented";
    expect([known, specialized, unknown]).toContain("table");
  });

  it("contains no server-only field names", () => {
    const code = projectionSrc
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .replace(/\/\/.*$/gm, "");
    const forbidden = [
      "Handler",
      "handle:",
      "permission",
      "auditEvent",
      "mountPath",
      "rawPath",
    ];
    for (const token of forbidden) {
      expect(
        code.includes(token),
        `projection.ts must not leak "${token}"`,
      ).toBe(false);
    }
  });
});
