import { describe, it, expect } from "vitest";
import {
  extensionOf,
  formatBytes,
  isPreviewable,
  languageFor,
  viewerFor,
} from "./fileTypes";

describe("file type mapping", () => {
  it("picks a viewer by extension", () => {
    expect(viewerFor("readme.md")).toBe("code");
    expect(viewerFor("photo.PNG")).toBe("image");
    expect(viewerFor("manual.pdf")).toBe("pdf");
    expect(viewerFor("song.mp3")).toBe("audio");
    expect(viewerFor("clip.mp4")).toBe("video");
    expect(viewerFor("archive.tar.gz")).toBe("download");
    expect(viewerFor("Dockerfile")).toBe("code");
  });

  it("prefers MIME when provided", () => {
    expect(viewerFor("blob", "image/png")).toBe("image");
    expect(viewerFor("data", "application/pdf")).toBe("pdf");
    expect(viewerFor("notes", "text/plain")).toBe("code");
  });

  it("maps code languages and detects extensions", () => {
    expect(extensionOf("a/b/c.YAML")).toBe("yaml");
    expect(extensionOf("noext")).toBe("");
    expect(languageFor("schema.sql")).toBe("sql");
    expect(languageFor("data.bin")).toBe("plaintext");
  });

  it("flags previewable vs download-only", () => {
    expect(isPreviewable("a.json")).toBe(true);
    expect(isPreviewable("a.zip")).toBe(false);
  });

  it("formats sizes", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(2048)).toBe("2.0 KB");
    expect(formatBytes(undefined)).toBe("—");
  });
});
