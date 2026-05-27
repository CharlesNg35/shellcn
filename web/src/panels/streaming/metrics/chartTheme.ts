export const metricPalette = [
  "#6366f1",
  "#10b981",
  "#f59e0b",
  "#0ea5e9",
  "#ec4899",
  "#8b5cf6",
];

export function seriesColor(index: number): string {
  return metricPalette[index % metricPalette.length];
}

export function fade(hex: string, alpha = 0.15): string {
  const n = parseInt(hex.slice(1), 16);
  return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`;
}

export function axisStyle(dark: boolean): { tick: string; grid: string } {
  return dark
    ? { tick: "#a1a1aa", grid: "rgba(255,255,255,0.07)" }
    : { tick: "#71717a", grid: "rgba(0,0,0,0.07)" };
}
