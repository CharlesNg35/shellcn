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

declare module "primevue/toasteventbus" {
  type ToastEventHandler = (payload: unknown) => void;
  interface ToastEventBus {
    emit(event: string, payload?: unknown): void;
    on(event: string, handler: ToastEventHandler): void;
    off(event: string, handler: ToastEventHandler): void;
  }
  const bus: ToastEventBus;
  export default bus;
}
