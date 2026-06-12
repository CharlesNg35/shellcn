import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import InputText from "primevue/inputtext";
import Checkbox from "primevue/checkbox";
import Button from "primevue/button";
import { installFetch } from "@/test/fetchMock";
import TwoFactorEnroll from "./TwoFactorEnroll.vue";

const session = {
  user: {
    id: "u1",
    username: "alice",
    roles: ["operator"],
    twoFactorEnabled: true,
  },
  csrfToken: "csrf",
  mfaReminder: false,
};

beforeEach(() => setActivePinia(createPinia()));
afterEach(() => vi.unstubAllGlobals());

describe("TwoFactorEnroll", () => {
  it("walks setup → confirm → recovery codes and emits enabled", async () => {
    installFetch((url, init) => {
      if (url.endsWith("/api/auth/totp/setup")) {
        return {
          body: {
            secret: "JBSWY3DPEHPK3PXP",
            otpauthUrl: "otpauth://totp/ShellCN:alice?secret=JBSWY3DPEHPK3PXP",
            qr: "data:image/png;base64,AAAA",
          },
        };
      }
      if (url.endsWith("/api/auth/totp/enable") && init?.method === "POST") {
        return { body: { recoveryCodes: ["aaaa-bbbb", "cccc-dddd"] } };
      }
      if (url.endsWith("/api/auth/me")) return { body: session };
      return { status: 404, body: {} };
    });

    const wrapper = mount(TwoFactorEnroll);
    await flushPromises();

    // Scan step: the QR and the manual key are shown.
    expect(wrapper.find("img").attributes("src")).toContain("data:image/png");
    expect(wrapper.text()).toContain("JBSWY3DPEHPK3PXP");

    // Enter a code and confirm.
    await wrapper.findComponent(InputText).setValue("123456");
    await wrapper
      .findAllComponents(Button)
      .find((b) => b.text().includes("Enable 2FA"))
      ?.trigger("click");
    await flushPromises();

    // Recovery step shows the codes.
    expect(wrapper.text()).toContain("aaaa-bbbb");
    const done = wrapper
      .findAllComponents(Button)
      .find((b) => b.text().trim() === "Done");

    // Done only fires after the user acknowledges they've saved the codes.
    await done?.trigger("click");
    expect(wrapper.emitted("enabled")).toBeFalsy();

    await wrapper.findComponent(Checkbox).setValue(true);
    await done?.trigger("click");
    expect(wrapper.emitted("enabled")).toBeTruthy();
  });
});
