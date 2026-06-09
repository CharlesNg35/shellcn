import type { CanvasCommand } from "./types";

export function wrapText(
  ctx: CanvasRenderingContext2D,
  text: string,
  width: number,
): string[] {
  const words = text.split(/\s+/).filter(Boolean);
  const lines: string[] = [];
  let current = "";
  for (const word of words) {
    const next = current ? `${current} ${word}` : word;
    if (current && ctx.measureText(next).width > width) {
      lines.push(current);
      current = word;
    } else {
      current = next;
    }
  }
  if (current) lines.push(current);
  return lines.length ? lines : [""];
}

export function fitTextBoxLines(
  ctx: CanvasRenderingContext2D,
  lines: string[],
  width: number,
  maxLines?: number,
  ellipsis?: string,
): string[] {
  const limit = Math.max(0, Math.floor(num(maxLines)));
  if (!limit || lines.length <= limit) return lines;
  const out = lines.slice(0, limit);
  out[out.length - 1] = ellipsizeText(
    ctx,
    out[out.length - 1],
    width,
    ellipsis || "...",
  );
  return out;
}

export function isEnhancedTextBox(
  command: Extract<CanvasCommand, { type: "textBox" }>,
): boolean {
  return Boolean(
    command.background ||
    command.backgroundId ||
    command.height ||
    command.padding ||
    command.maxLines ||
    command.ellipsis ||
    command.verticalAlign ||
    command.radius ||
    command.radii,
  );
}

export function textBoxOffsetY(
  height: number,
  textHeight: number,
  padding: number,
  align?: string,
): number {
  const available = Math.max(0, height - padding * 2 - textHeight);
  if (align === "bottom") return padding + available;
  if (align === "middle") return padding + available / 2;
  return padding;
}

export function textBoxAnchorX(
  x: number,
  width: number,
  align: CanvasTextAlign,
): number {
  if (align === "center") return x + width / 2;
  if (align === "right" || align === "end") return x + width;
  return x;
}

function ellipsizeText(
  ctx: CanvasRenderingContext2D,
  text: string,
  width: number,
  ellipsis: string,
): string {
  if (ctx.measureText(text).width <= width) return text;
  const suffix = ellipsis || "...";
  let out = text;
  while (out.length > 0 && ctx.measureText(out + suffix).width > width) {
    out = out.slice(0, -1);
  }
  return out ? out + suffix : suffix;
}

function num(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}
