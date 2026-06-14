const encodedRedirectPrefix = "u:";

function normalizeRedirectTarget(value: string): string {
  if (!value.startsWith("/") || value.startsWith("//")) return "/";
  for (let i = 0; i < value.length; i += 1) {
    const code = value.charCodeAt(i);
    if (code < 32 || code === 127) return "/";
  }
  return value;
}

export function encodeRedirectTarget(value: string): string {
  return `${encodedRedirectPrefix}${encodeURIComponent(
    normalizeRedirectTarget(value),
  )}`;
}

export function decodeRedirectTarget(value: unknown): string {
  if (typeof value !== "string") return "/";
  if (value.startsWith(encodedRedirectPrefix)) {
    try {
      return normalizeRedirectTarget(
        decodeURIComponent(value.slice(encodedRedirectPrefix.length)),
      );
    } catch {
      return "/";
    }
  }
  try {
    return normalizeRedirectTarget(decodeURIComponent(value));
  } catch {
    return normalizeRedirectTarget(value);
  }
}
