import { config } from "@vue/test-utils";
import { vi } from "vitest";
import PrimeVue from "primevue/config";
import ToastService from "primevue/toastservice";
import ConfirmationService from "primevue/confirmationservice";
import { primeVuePassthrough } from "../primevue/preset";

// jsdom implements neither ResizeObserver (PrimeVue Tabs' ink bar) nor matchMedia
// (Select); stub both so widgets mount under test.
if (!window.ResizeObserver) {
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}

if (!window.matchMedia) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

// Register the PrimeVue plugin (+ Toast/Confirmation services) for every mounted
// component so PrimeVue widgets and useToast/useConfirm resolve in unit tests.
config.global.plugins = [
  [PrimeVue, { unstyled: true, pt: primeVuePassthrough }],
  ToastService,
  ConfirmationService,
];

// Render RouterLink as a plain anchor when no router is installed in a unit test.
config.global.stubs = {
  RouterLink: { props: ["to"], template: "<a><slot /></a>" },
};
