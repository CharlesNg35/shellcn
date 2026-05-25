import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import Select from "primevue/select";
import InputText from "primevue/inputtext";
import { installFetch } from "../test/fetchMock";
import { useConnectionsStore } from "../stores/connections";
import ConnectionFormDialog from "./ConnectionFormDialog.vue";
import SchemaForm from "../panels/form/SchemaForm.vue";
import type { PluginProjection } from "../types/projection";

const projection: PluginProjection = {
  apiVersion: 1,
  name: "tester",
  version: "0",
  title: "Tester",
  description: "",
  icon: { type: "name", value: "box" },
  config: {
    groups: [
      {
        name: "Basic",
        fields: [
          { key: "host", label: "Host", type: "text", required: true },
          {
            key: "password",
            label: "Password",
            type: "password",
            secret: true,
          },
        ],
      },
    ],
  },
  capabilities: [],
  supportedTransports: ["direct"],
  layout: "tabs",
};

beforeEach(() => {
  setActivePinia(createPinia());
});
afterEach(() => vi.unstubAllGlobals());

describe("ConnectionFormDialog", () => {
  it("renders a plugin's config schema and posts a new connection", async () => {
    let posted: Record<string, unknown> | null = null;
    installFetch((url, init) => {
      if (url.endsWith("/api/plugins/tester")) return { body: projection };
      if (url.endsWith("/api/connections") && init?.method === "POST") {
        posted = JSON.parse(String(init.body));
        return { status: 201, body: { id: "conn-new", name: posted?.name } };
      }
      if (url.endsWith("/api/connections")) return { body: [] };
      return { body: [] };
    });

    const conns = useConnectionsStore();
    conns.plugins = [
      { name: "tester", title: "Tester", icon: { type: "name", value: "box" } },
    ];

    const wrapper = mount(ConnectionFormDialog, {
      props: { visible: true },
    });
    await flushPromises();

    // Choosing a protocol fetches and renders its config schema.
    await wrapper.findComponent(Select).vm.$emit("update:modelValue", "tester");
    await flushPromises();
    const form = wrapper.findComponent(SchemaForm);
    expect(form.exists()).toBe(true);
    expect(form.text()).toContain("Host");

    // Name + a valid config submit through the generic form. (Dialog content is
    // teleported, so drive the components directly rather than DOM selectors.)
    wrapper
      .findAllComponents(InputText)[0]
      .vm.$emit("update:modelValue", "db1");
    await flushPromises();
    form.vm.$emit("submit", { host: "10.0.0.1" });
    await flushPromises();

    expect(posted).toMatchObject({
      name: "db1",
      protocol: "tester",
      config: { host: "10.0.0.1" },
    });
    expect(wrapper.emitted("saved")?.[0][0]).toMatchObject({
      id: "conn-new",
      created: true,
    });
  });
});
