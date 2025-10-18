/**
 * Hook to detect if a Radix UI tab is currently active
 */
import { type RefObject } from 'react'

/**
 * Checks if the container is within an active Radix UI tab
 * @param containerRef Reference to the container element
 * @returns True if the tab is active, false otherwise
 */
export function useIsTabActive(containerRef: RefObject<HTMLElement>): boolean {
  const container = containerRef.current
  if (!container) {
    return false
  }

  // Find parent TabsContent element
  let parent = container.parentElement
  while (parent && parent.getAttribute('data-radix-tabs-content') === null) {
    parent = parent.parentElement
  }

  // Check if tab is active (data-state="active")
  return parent?.getAttribute('data-state') === 'active'
}

/**
 * Gets the active state of a tab without hooks (for use in effects)
 */
export function getIsTabActive(container: HTMLElement | null): boolean {
  if (!container) {
    return false
  }

  let parent = container.parentElement
  while (parent && parent.getAttribute('data-radix-tabs-content') === null) {
    parent = parent.parentElement
  }

  return parent?.getAttribute('data-state') === 'active'
}
