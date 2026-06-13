import { describe, expect, it } from "vitest";
import { decodeRedirectTarget, encodeRedirectTarget } from "./redirect";

describe("redirect target encoding", () => {
  it("keeps raw slashes out of generated login query values", () => {
    const encoded = encodeRedirectTarget(
      "/c/8117a967-9114-4c95-a861-215197e791a3?v=list:deployment&vc=8117a967-9114-4c95-a861-215197e791a3",
    );

    expect(encoded).not.toContain("/");
    expect(decodeRedirectTarget(encoded)).toBe(
      "/c/8117a967-9114-4c95-a861-215197e791a3?v=list:deployment&vc=8117a967-9114-4c95-a861-215197e791a3",
    );
  });

  it("accepts existing raw or percent-encoded local redirects", () => {
    expect(decodeRedirectTarget("/settings")).toBe("/settings");
    expect(decodeRedirectTarget("%2Fsettings%2Fprofile")).toBe(
      "/settings/profile",
    );
  });

  it("rejects non-local redirects", () => {
    expect(decodeRedirectTarget("https://example.com")).toBe("/");
    expect(decodeRedirectTarget("//example.com")).toBe("/");
    expect(encodeRedirectTarget("https://example.com")).toBe("u:%2F");
  });
});
