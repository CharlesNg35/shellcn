import {
  isCanvasCommandType,
  type CanvasCommand,
  type CanvasFrame,
  type CanvasPoint,
  type CanvasRegion,
} from "./types";

export function parseCanvasFrame(raw: string): CanvasFrame {
  const parsed: unknown = JSON.parse(raw);
  if (Array.isArray(parsed)) return { commands: parseCommands(parsed) };
  if (!isRecord(parsed)) return { commands: [] };
  if (Array.isArray(parsed.commands)) {
    return {
      commands: parseCommands(parsed.commands),
      regions: parseRegions(parsed.regions),
    };
  }
  return { commands: parseCommands([parsed]) };
}

function parseCommands(items: unknown[]): CanvasCommand[] {
  return items.filter(isCommand) as CanvasCommand[];
}

function isCommand(value: unknown): value is CanvasCommand {
  return (
    isRecord(value) &&
    typeof value.type === "string" &&
    isCanvasCommandType(value.type)
  );
}

export function parseRegions(value: unknown): CanvasRegion[] | undefined {
  if (!Array.isArray(value)) return undefined;
  const out = value.flatMap((item) => {
    const region = parseRegion(item);
    return region ? [region] : [];
  });
  return out.length ? out : [];
}

function parseRegion(value: unknown): CanvasRegion | undefined {
  if (!isRecord(value) || typeof value.id !== "string") return undefined;
  return {
    id: value.id,
    shape: stringValue(value.shape) as CanvasRegion["shape"],
    x: numberValue(value.x),
    y: numberValue(value.y),
    width: optionalNumber(value.width),
    height: optionalNumber(value.height),
    radius: optionalNumber(value.radius),
    points: parsePoints(value.points),
    d: stringValue(value.d),
    cursor: stringValue(value.cursor),
    label: stringValue(value.label),
    capturePointer: value.capturePointer === true,
  };
}

function parsePoints(value: unknown): CanvasPoint[] | undefined {
  if (!Array.isArray(value)) return undefined;
  const points = value.flatMap((item) => {
    if (!isRecord(item)) return [];
    return [{ x: numberValue(item.x), y: numberValue(item.y) }];
  });
  return points.length ? points : undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function numberValue(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function optionalNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value)
    ? value
    : undefined;
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}
