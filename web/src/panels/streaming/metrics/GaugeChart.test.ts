import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import GaugeChart from "./GaugeChart.vue";

function mountGauge(props: Record<string, unknown>) {
  return mount(GaugeChart, { props, global: { stubs: { Chart: true } } });
}

describe("GaugeChart", () => {
  it("shows a rounded percentage of max", () => {
    const w = mountGauge({ label: "CPU", value: 42.6, max: 100, unit: "%" });
    expect(w.text()).toContain("43");
    expect(w.text()).toContain("%");
    expect(w.text()).toContain("CPU");
  });

  it("shows the raw value + unit for non-percentage gauges", () => {
    const w = mountGauge({ label: "Memory", value: 6, max: 16, unit: "GiB" });
    expect(w.text()).toContain("6");
    expect(w.text()).toContain("GiB");
  });

  it("renders a dash when there is no value yet", () => {
    const w = mountGauge({ label: "CPU", value: null });
    expect(w.text()).toContain("—");
  });
});
