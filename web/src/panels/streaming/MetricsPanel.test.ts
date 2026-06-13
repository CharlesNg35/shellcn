import { describe, it, expect, beforeEach } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import MetricsPanel from "./MetricsPanel.vue";

beforeEach(() => setActivePinia(createPinia()));

const stubs = {
  StreamStatusBar: true,
  StatCard: { template: '<div class="stat" />' },
  GaugeChart: { template: '<div class="gauge" />' },
  SeriesChart: { template: '<div class="series" />' },
};

function mountPanel(config?: Record<string, unknown>) {
  return mount(MetricsPanel, {
    props: { connectionId: "c1", config },
    global: { stubs },
  });
}

describe("MetricsPanel", () => {
  it("renders nothing plugin-specific without a declared config", () => {
    const w = mountPanel();
    expect(w.findAll(".gauge")).toHaveLength(0);
    expect(w.findAll(".series")).toHaveLength(0);
    expect(w.findAll(".stat")).toHaveLength(0);
    expect(w.text()).toContain("No metrics configured.");
  });

  it("renders usage rows instead of duplicate gauges for the same metric", () => {
    const w = mountPanel({
      stats: [{ key: "conns", label: "Connections" }],
      gauges: [
        { key: "cpu", label: "CPU", max: 100 },
        { key: "memPct", label: "Memory", max: 100 },
        { key: "disk", label: "Disk", max: 100 },
      ],
      usage: [
        {
          key: "memPct",
          label: "Memory usage",
          type: "percent",
          usage: { percentKey: "memPct" },
        },
      ],
      series: [{ key: "cpu", label: "CPU" }],
    });
    expect(w.findAll(".stat")).toHaveLength(1);
    expect(w.findAll(".gauge")).toHaveLength(2);
    expect(w.text()).toContain("Memory usage");
    expect(w.findAll(".series")).toHaveLength(1);
  });

  it("renders only the declared section when a single kind is set", () => {
    const w = mountPanel({ usage: [{ key: "cpu", label: "CPU" }] });
    expect(w.findAll(".stat")).toHaveLength(0);
    expect(w.findAll(".gauge")).toHaveLength(0);
    expect(w.findAll(".series")).toHaveLength(0);
    expect(w.text()).toContain("CPU");
    expect(w.text()).not.toContain("No metrics configured.");
  });
});
