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
  md: "markdown",
  markdown: "markdown",
  yml: "yaml",
  yaml: "yaml",
  json: "json",
  sql: "sql",
  sh: "shell",
  bash: "shell",
  py: "python",
  go: "go",
  ts: "typescript",
  tsx: "typescript",
  js: "javascript",
  jsx: "javascript",
  css: "css",
  scss: "scss",
  less: "less",
  html: "html",
  xml: "xml",
  csv: "csv",
  tsv: "csv",
  toml: "toml",
  ini: "ini",
  conf: "ini",
  env: "ini",
  graphql: "graphql",
  gql: "graphql",
  proto: "protobuf",
  tf: "hcl",
  hcl: "hcl",
  tfvars: "hcl",
  dockerfile: "dockerfile",
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
