declare module 'asciinema-player' {
  export interface AsciinemaPlayer {
    dispose?: () => void
  }

  export function create(
    source: string,
    element: HTMLElement,
    options?: Record<string, unknown>
  ): AsciinemaPlayer
}

declare module 'pako' {
  export function ungzip(
    data: Uint8Array,
    options?: { to?: 'string' | undefined }
  ): string | Uint8Array
}
