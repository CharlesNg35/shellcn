/**
 * Terminal utility functions
 */
import type { SshTerminalEvent } from '@/types/ssh'

/**
 * Clamps a number between min and max values
 */
export function clampNumber(value: number, min: number, max: number): number {
  if (!Number.isFinite(value)) {
    return min
  }
  if (value < min) {
    return min
  }
  if (value > max) {
    return max
  }
  return value
}

/**
 * Checks if an SSH terminal event is stream data (stdout/stderr)
 */
export function isStreamData(event: SshTerminalEvent): boolean {
  const name = event.event.toLowerCase()
  return name === 'stdout' || name === 'stderr'
}

/**
 * Normalizes terminal dimensions
 */
export function normalizeTerminalDimensions(
  cols: number,
  rows: number
): { cols: number; rows: number } {
  return {
    cols: Math.max(1, Math.floor(cols)),
    rows: Math.max(1, Math.floor(rows)),
  }
}
