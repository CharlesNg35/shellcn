import { mount, flushPromises } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import Button from "primevue/button";
import MarketTable from "./MarketTable.vue";
import type { MarketEntry } from "@/types/projection";

function entry(overrides: Partial<MarketEntry> = {}): MarketEntry {
  return {
    name: "cassandra",
    displayName: "Cassandra",
    description: "Wide-column database plugin",
    repo: "github.com/CharlesNg35/shellcn-contrib",
    license: "MIT",
    maintainers: ["ShellCN"],
    latest: {
      version: "0.1.0",
      apiVersion: 1,
      protocolVersion: 1,
      platforms: ["linux/amd64"],
      icon: { type: "lucide", value: "database" },
      snapshotUrl:
        "https://raw.githubusercontent.com/CharlesNg35/shellcn-plugin-registry/main/snapshots/cassandra/cassandra-v0.1.0.json",
    },
    compatible: true,
    managed: false,
    updateAvailable: false,
    ...overrides,
  };
}

describe("MarketTable", () => {
  it("filters marketplace entries by search text", async () => {
    const wrapper = mount(MarketTable, {
      props: {
        entries: [
          entry(),
          entry({
            name: "elasticsearch",
            displayName: "Elasticsearch",
            description: "Search engine plugin",
          }),
        ],
        loading: false,
        installing: {},
        uninstalling: {},
      },
    });

    const search = wrapper.get(
      'input[aria-label="Search marketplace plugins"]',
    );
    const iconShell = search.element.nextElementSibling as HTMLElement;
    expect(iconShell.className).toContain("inset-y-0");
    expect(iconShell.className).toContain("items-center");

    await search.setValue("elastic");
    await flushPromises();

    expect(wrapper.text()).toContain("Elasticsearch");
    expect(wrapper.text()).not.toContain("Cassandra");
  });

  it("emits install and uninstall actions for managed entries", async () => {
    const wrapper = mount(MarketTable, {
      props: {
        entries: [
          entry({
            managed: true,
            installedVersion: "0.1.0",
            updateAvailable: true,
          }),
        ],
        loading: false,
        installing: {},
        uninstalling: {},
      },
    });

    const buttons = wrapper.findAllComponents(Button);
    const update = buttons.find((button) => button.props("label") === "Update");
    const uninstall = buttons.find(
      (button) => button.props("label") === "Uninstall",
    );
    expect(update).toBeTruthy();
    expect(uninstall).toBeTruthy();

    await update!.trigger("click");
    await uninstall!.trigger("click");

    expect(wrapper.emitted("install")?.[0][0]).toMatchObject({
      name: "cassandra",
    });
    expect(wrapper.emitted("uninstall")?.[0][0]).toMatchObject({
      name: "cassandra",
    });
  });
});
