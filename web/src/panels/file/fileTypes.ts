// Maps a file (by extension/MIME) to a built-in preview viewer. Data-driven and
// extensible — a new viewer is a one-time core addition, reused by every
// filesystem plugin.

export type ViewerKind =
  | "code"
  | "image"
  | "pdf"
  | "audio"
  | "video"
  | "download";

const EXT: Record<string, ViewerKind> = {};
const add = (kind: ViewerKind, exts: string[]) =>
  exts.forEach((e) => (EXT[e] = kind));

add("code", [
  "txt",
  "log",
  "md",
  "markdown",
  "mdx",
  "json",
  "jsonc",
  "jsonl",
  "yaml",
  "yml",
  "toml",
  "ini",
  "conf",
  "config",
  "env",
  "properties",
  "editorconfig",
  "gitignore",
  "sh",
  "bash",
  "zsh",
  "fish",
  "ps1",
  "py",
  "go",
  "ts",
  "tsx",
  "js",
  "jsx",
  "sql",
  "rb",
  "rs",
  "java",
  "c",
  "h",
  "cpp",
  "cs",
  "php",
  "xml",
  "html",
  "css",
  "scss",
  "less",
  "csv",
  "tsv",
  "graphql",
  "gql",
  "proto",
  "tf",
  "hcl",
  "tfvars",
  "nginx",
  "service",
  "socket",
  "timer",
  "dockerfile",
]);
add("image", [
  "png",
  "jpg",
  "jpeg",
  "gif",
  "webp",
  "svg",
  "bmp",
  "ico",
  "avif",
]);
add("pdf", ["pdf"]);
add("audio", ["mp3", "wav", "ogg", "oga", "flac", "m4a", "aac", "opus"]);
add("video", ["mp4", "webm", "ogv", "mov", "mkv", "m4v", "avi"]);

const CODE_LANG: Record<string, string> = {
  yml: "yaml",
  yaml: "yaml",
  json: "json",
  jsonc: "json",
  jsonl: "json",
  sql: "sql",
  sh: "shell",
  bash: "shell",
  zsh: "shell",
  fish: "shell",
  ps1: "powershell",
  toml: "toml",
  ini: "ini",
  conf: "ini",
  config: "ini",
  env: "ini",
  properties: "properties",
  editorconfig: "properties",
  gitignore: "properties",
  service: "properties",
  socket: "properties",
  timer: "properties",
  dockerfile: "dockerfile",
  nginx: "nginx",
  xml: "xml",
  html: "xml",
};

export function extensionOf(name: string): string {
  const base = name.split("/").pop() ?? name;
  if (base.toLowerCase() === "dockerfile") return "dockerfile";
  if (base.startsWith(".") && base.indexOf(".", 1) === -1) {
    return base.slice(1).toLowerCase();
  }
  const dot = base.lastIndexOf(".");
  return dot > 0 ? base.slice(dot + 1).toLowerCase() : "";
}

export function viewerFor(name: string, mime?: string): ViewerKind {
  if (mime) {
    if (mime.startsWith("image/")) return "image";
    if (mime.startsWith("audio/")) return "audio";
    if (mime.startsWith("video/")) return "video";
    if (mime === "application/pdf" || mime === "application/x-pdf")
      return "pdf";
    if (
      mime.startsWith("text/") ||
      mime.includes("json") ||
      mime.includes("xml") ||
      mime.includes("yaml") ||
      mime.includes("toml") ||
      mime === "application/javascript" ||
      mime === "application/x-sh" ||
      mime === "application/graphql"
    ) {
      return "code";
    }
  }
  return EXT[extensionOf(name)] ?? "download";
}

export function languageFor(name: string): string {
  return CODE_LANG[extensionOf(name)] ?? "plaintext";
}

// Lucide icon per file kind. Specific extensions win; otherwise the preview
// viewer category picks a sensible default. Folders are handled by the caller.
const ICON_BY_EXT: Record<string, string> = {
  json: "file-json",
  jsonc: "file-json",
  jsonl: "file-json",
  yaml: "file-cog",
  yml: "file-cog",
  toml: "file-cog",
  ini: "file-cog",
  conf: "file-cog",
  config: "file-cog",
  env: "file-cog",
  properties: "file-cog",
  editorconfig: "file-cog",
  service: "file-cog",
  socket: "file-cog",
  timer: "file-cog",
  xml: "code-xml",
  html: "code-xml",
  sh: "file-terminal",
  bash: "file-terminal",
  zsh: "file-terminal",
  fish: "file-terminal",
  ps1: "file-terminal",
  sql: "database",
  csv: "file-spreadsheet",
  tsv: "file-spreadsheet",
  txt: "file-text",
  log: "file-text",
  md: "file-text",
  markdown: "file-text",
  mdx: "file-text",
  dockerfile: "container",
};

const ICON_BY_VIEWER: Record<ViewerKind, string> = {
  code: "file-code",
  image: "file-image",
  pdf: "file-text",
  audio: "file-audio",
  video: "file-video",
  download: "file",
};

const ARCHIVE_EXT = new Set([
  "zip",
  "tar",
  "gz",
  "tgz",
  "bz2",
  "tbz2",
  "xz",
  "7z",
  "rar",
  "zst",
  "lz",
  "lzma",
]);
const KEY_EXT = new Set([
  "key",
  "pem",
  "crt",
  "cer",
  "pub",
  "p12",
  "pfx",
  "keystore",
  "asc",
  "gpg",
]);

export function iconFor(name: string, isDir: boolean): string {
  if (isDir) return "folder";
  const ext = extensionOf(name);
  if (ICON_BY_EXT[ext]) return ICON_BY_EXT[ext];
  if (ARCHIVE_EXT.has(ext)) return "file-archive";
  if (KEY_EXT.has(ext)) return "file-key";
  return ICON_BY_VIEWER[viewerFor(name)];
}

export function isPreviewable(name: string, mime?: string): boolean {
  return viewerFor(name, mime) !== "download";
}

export function formatBytes(bytes?: number): string {
  if (bytes === undefined) return "—";
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let v = bytes / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(1)} ${units[i]}`;
}
