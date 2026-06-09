import type { CanvasPoint, CanvasRadii } from "./types";

export type RectPathTarget =
  | Pick<
      CanvasRenderingContext2D,
      "closePath" | "lineTo" | "moveTo" | "quadraticCurveTo" | "rect"
    >
  | Path2D;

export function addRectPath(
  path: RectPathTarget,
  x: number,
  y: number,
  width: number,
  height: number,
  radius = 0,
  radii?: CanvasRadii,
): void {
  const corners = resolveRadii(radius, radii, width, height);
  if (
    corners.topLeft ||
    corners.topRight ||
    corners.bottomRight ||
    corners.bottomLeft
  ) {
    path.moveTo(x + corners.topLeft, y);
    path.lineTo(x + width - corners.topRight, y);
    path.quadraticCurveTo(x + width, y, x + width, y + corners.topRight);
    path.lineTo(x + width, y + height - corners.bottomRight);
    path.quadraticCurveTo(
      x + width,
      y + height,
      x + width - corners.bottomRight,
      y + height,
    );
    path.lineTo(x + corners.bottomLeft, y + height);
    path.quadraticCurveTo(x, y + height, x, y + height - corners.bottomLeft);
    path.lineTo(x, y + corners.topLeft);
    path.quadraticCurveTo(x, y, x + corners.topLeft, y);
    path.closePath();
  } else {
    path.rect(x, y, width, height);
  }
}

export function pointInPolygon(
  point: CanvasPoint,
  polygon: CanvasPoint[],
): boolean {
  let inside = false;
  for (let i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
    const xi = polygon[i].x;
    const yi = polygon[i].y;
    const xj = polygon[j].x;
    const yj = polygon[j].y;
    if (
      yi > point.y !== yj > point.y &&
      point.x < ((xj - xi) * (point.y - yi)) / (yj - yi) + xi
    )
      inside = !inside;
  }
  return inside;
}

function resolveRadii(
  radius: number,
  radii: CanvasRadii | undefined,
  width: number,
  height: number,
): Required<CanvasRadii> {
  const fallback = Math.max(0, radius);
  const max = Math.max(0, Math.min(Math.abs(width), Math.abs(height)) / 2);
  return {
    topLeft: clamp(num(radii?.topLeft, fallback), 0, max),
    topRight: clamp(num(radii?.topRight, fallback), 0, max),
    bottomRight: clamp(num(radii?.bottomRight, fallback), 0, max),
    bottomLeft: clamp(num(radii?.bottomLeft, fallback), 0, max),
  };
}

function num(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}
