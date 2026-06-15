import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import Message from "primevue/message";
import AppAlert from "./AppAlert.vue";

describe("AppAlert", () => {
  it("maps app alert tones to PrimeVue message severities", () => {
    const wrapper = mount(AppAlert, {
      props: { tone: "danger", title: "Could not connect" },
      slots: { default: "Authentication failed" },
    });

    expect(wrapper.getComponent(Message).props("severity")).toBe("error");
    expect(wrapper.text()).toContain("Could not connect");
    expect(wrapper.text()).toContain("Authentication failed");
  });

  it("forwards close events from closable messages", async () => {
    const wrapper = mount(AppAlert, {
      props: { tone: "warning", title: "Upload blocked", closable: true },
      slots: { default: "File is too large" },
    });

    await wrapper.getComponent(Message).vm.$emit("close", new Event("close"));

    expect(wrapper.emitted("close")).toHaveLength(1);
  });
});
