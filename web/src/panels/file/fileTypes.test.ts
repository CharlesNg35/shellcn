import { describe, it, expect } from "vitest";
import {
  extensionOf,
  formatBytes,
  formatDate,
  iconFor,
  isPreviewable,
  languageFor,
  sortEntries,
  viewerFor,
} from "./fileTypes";
import type { FileEntry } from "@/types/projection";

describe("file type mapping", () => {
  it("picks a viewer by extension", () => {
    expect(viewerFor("readme.md")).toBe("code");
    expect(viewerFor("photo.PNG")).toBe("image");
    expect(viewerFor("manual.pdf")).toBe("pdf");
    expect(viewerFor("song.mp3")).toBe("audio");
    expect(viewerFor("clip.mp4")).toBe("video");
    expect(viewerFor("archive.tar.gz")).toBe("download");
    expect(viewerFor("Dockerfile")).toBe("code");
    expect(viewerFor(".env")).toBe("code");
    expect(viewerFor("main.tfvars")).toBe("code");
    expect(viewerFor("diagram.svg")).toBe("image");
    expect(viewerFor("movie.ogv")).toBe("video");
    expect(viewerFor("voice.opus")).toBe("audio");
  });

  it("prefers MIME when provided", () => {
    expect(viewerFor("blob", "image/png")).toBe("image");
    expect(viewerFor("data", "application/pdf")).toBe("pdf");
    expect(viewerFor("notes", "text/plain")).toBe("code");
    expect(viewerFor("api", "application/graphql")).toBe("code");
    expect(viewerFor("manifest", "application/manifest+json")).toBe("code");
  });

  it("maps code languages and detects extensions", () => {
    expect(extensionOf("a/b/c.YAML")).toBe("yaml");
    expect(extensionOf("noext")).toBe("");
    expect(languageFor("schema.sql")).toBe("sql");
    expect(languageFor(".env")).toBe("ini");
    expect(languageFor("Dockerfile")).toBe("dockerfile");
    expect(languageFor("deploy.sh")).toBe("shell");
    expect(languageFor("main.tf")).toBe("plaintext");
    expect(languageFor("data.bin")).toBe("plaintext");
  });

  it("flags previewable vs download-only", () => {
    expect(isPreviewable("a.json")).toBe(true);
    expect(isPreviewable("a.zip")).toBe(false);
  });

  it("picks a file icon by extension, category, and special kinds", () => {
    expect(iconFor("any", true)).toBe("folder");
    expect(iconFor("data.json", false)).toBe("file-json");
    expect(iconFor("compose.yaml", false)).toBe("file-cog");
    expect(iconFor("schema.sql", false)).toBe("database");
    expect(iconFor("deploy.sh", false)).toBe("file-terminal");
    expect(iconFor("rows.csv", false)).toBe("file-spreadsheet");
    expect(iconFor("page.html", false)).toBe("code-xml");
    expect(iconFor("Dockerfile", false)).toBe("container");
    expect(iconFor("main.go", false)).toBe("file-code"); // code viewer fallback
    expect(iconFor("photo.png", false)).toBe("file-image");
    expect(iconFor("clip.mp4", false)).toBe("file-video");
    expect(iconFor("bundle.tar.gz", false)).toBe("file-archive");
    expect(iconFor("server.pem", false)).toBe("file-key");
    expect(iconFor("blob.bin", false)).toBe("file"); // unknown → generic
  });

  it("formats sizes", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(2048)).toBe("2.0 KB");
    expect(formatBytes(undefined)).toBe("—");
  });

  it("formats dates and dashes blanks/invalid", () => {
    expect(formatDate(undefined)).toBe("—");
    expect(formatDate("not-a-date")).toBe("—");
    expect(formatDate("2026-05-29T19:30:00Z")).not.toBe("—");
  });

  it("sorts with directories first, then by key and direction", () => {
    const e = (
      name: string,
      isDir: boolean,
      size = 0,
      modTime?: string,
    ): FileEntry => ({ name, path: `/${name}`, isDir, size, modTime });
    const entries = [
      e("b.txt", false, 30, "2026-01-02T00:00:00Z"),
      e("zeta", true),
      e("a.txt", false, 10, "2026-03-01T00:00:00Z"),
      e("alpha", true),
    ];

    const byName = sortEntries(entries, "name", "asc").map((x) => x.name);
    expect(byName).toEqual(["alpha", "zeta", "a.txt", "b.txt"]); // dirs first

    const bySizeDesc = sortEntries(entries, "size", "desc")
      .filter((x) => !x.isDir)
      .map((x) => x.name);
    expect(bySizeDesc).toEqual(["b.txt", "a.txt"]); // 30 before 10

    const byModifiedAsc = sortEntries(entries, "modified", "asc")
      .filter((x) => !x.isDir)
      .map((x) => x.name);
    expect(byModifiedAsc).toEqual(["b.txt", "a.txt"]); // Jan before Mar
  });
});
