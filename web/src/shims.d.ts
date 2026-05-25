declare module "@novnc/novnc" {
  const RFB: unknown;
  export default RFB;
}

declare module "asciinema-player" {
  export interface Player {
    dispose(): void;
  }
  export function create(
    src: unknown,
    el: HTMLElement,
    opts?: Record<string, unknown>,
  ): Player;
}

declare module "asciinema-player/dist/bundle/asciinema-player.css";
