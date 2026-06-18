import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import ProtocolPicker from "./ProtocolPicker.vue";
import type { PluginSummary } from "../types/projection";

const shell = {
  key: "shell",
  label: "Shell & terminal",
  icon: { type: "lucide", value: "terminal" },
  order: 10,
} as const;

const databases = {
  key: "databases",
  label: "Databases",
  icon: { type: "lucide", value: "database" },
  order: 60,
} as const;

const plugins: PluginSummary[] = [
  {
    name: "postgres",
    title: "PostgreSQL",
    icon: { type: "lucide", value: "database" },
    category: databases,
  },
  {
    name: "ssh",
    title: "SSH",
    icon: { type: "lucide", value: "terminal" },
    category: shell,
  },
];

describe("ProtocolPicker", () => {
  it("groups plugins by manifest category order", () => {
    const wrapper = mount(ProtocolPicker, {
      props: { modelValue: "", plugins },
    });
    const headers = wrapper.findAll("header").map((h) => h.text());
    expect(headers[0]).toContain("Shell & terminal");
    expect(headers[1]).toContain("Databases");
  });

  it("searches category labels", async () => {
    const wrapper = mount(ProtocolPicker, {
      props: { modelValue: "", plugins },
    });
    await wrapper.find('input[type="search"]').setValue("database");
    expect(wrapper.text()).toContain("PostgreSQL");
    expect(wrapper.text()).not.toContain("SSH");
  });

  it("uses the shared search icon alignment", () => {
    const wrapper = mount(ProtocolPicker, {
      props: { modelValue: "", plugins },
    });

    const iconShell = wrapper.get('input[aria-label="Search protocols"]')
      .element.previousElementSibling as HTMLElement;
    expect(iconShell.className).toContain("inset-y-0");
    expect(iconShell.className).toContain("items-center");
    expect(
      wrapper.get('input[aria-label="Search protocols"]').classes(),
    ).toContain("h-9");
  });

  it("keeps protocol icon tiles light in dark mode for fixed-color SVG icons", () => {
    const wrapper = mount(ProtocolPicker, {
      props: { modelValue: "ssh", plugins },
    });

    const iconTiles = wrapper.findAll(".h-9.w-9");
    expect(iconTiles[0].classes()).toEqual(
      expect.arrayContaining(["dark:bg-primary-100", "dark:text-primary-700"]),
    );
    expect(iconTiles[1].classes()).toEqual(
      expect.arrayContaining(["dark:bg-surface-100", "dark:text-surface-700"]),
    );
  });
});
