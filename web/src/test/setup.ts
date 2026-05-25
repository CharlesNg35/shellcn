import { config } from "@vue/test-utils";
import { vi } from "vitest";
import PrimeVue from "primevue/config";
import ToastService from "primevue/toastservice";
import { primeVuePassthrough } from "../primevue/preset";

// jsdom does not implement matchMedia; some PrimeVue widgets (Select) call it.
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

// Register the PrimeVue plugin (+ Toast service) for every mounted component so
// PrimeVue widgets resolve their config in unit tests.
config.global.plugins = [
  [PrimeVue, { unstyled: true, pt: primeVuePassthrough }],
  ToastService,
];
