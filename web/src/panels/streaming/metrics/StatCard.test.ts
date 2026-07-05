import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import StatCard from "./StatCard.vue";

describe("StatCard", () => {
  it("formats bytes and separates scalar units", () => {
    const memory = mount(StatCard, {
      props: { label: "Memory", value: 53_899_264, unit: "bytes" },
    });
    expect(memory.text()).toContain("51.4 MiB");
    expect(memory.text()).not.toContain("53899264bytes");

    const cpu = mount(StatCard, {
      props: { label: "CPU", value: 0.003, unit: "cores" },
    });
    expect(cpu.text()).toContain("0.003");
    expect(cpu.text()).toContain("cores");
    expect(cpu.text()).not.toContain("0.003cores");
  });

  it("formats byte rates", () => {
    const w = mount(StatCard, {
      props: { label: "Net in", value: 2048, unit: "bytes/s" },
    });
    expect(w.text()).toContain("2.0 KiB/s");
  });

  it("formats common backend unit aliases", () => {
    const bytes = mount(StatCard, {
      props: { label: "Disk read", value: 442_304_951_296, unit: "B" },
    });
    expect(bytes.text()).toContain("411.9 GiB");

    const rate = mount(StatCard, {
      props: { label: "Network", value: 2048, unit: "B/s" },
    });
    expect(rate.text()).toContain("2.0 KiB/s");

    const uptime = mount(StatCard, {
      props: { label: "Uptime", value: 3_093_695, unit: "s" },
    });
    expect(uptime.text()).toContain("35d 19h");
  });
});
