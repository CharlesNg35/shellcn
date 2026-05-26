import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import Select from "primevue/select";
import InputText from "primevue/inputtext";
import { installFetch } from "../test/fetchMock";
import { useConnectionsStore } from "../stores/connections";
import ConnectionFormDialog from "./ConnectionFormDialog.vue";
import ProtocolPicker from "./ProtocolPicker.vue";
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
    await wrapper
      .findComponent(ProtocolPicker)
      .vm.$emit("update:modelValue", "tester");
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

  it("shows recording options only for plugins that declare support", async () => {
    let posted: Record<string, unknown> | null = null;
    const recordable: PluginProjection = {
      ...projection,
      recording: [
        {
          class: "terminal",
          formats: ["asciicast_v2"],
          authoritative: true,
          inputCapture: false,
        },
      ],
    };
    installFetch((url, init) => {
      if (url.endsWith("/api/plugins/tester")) return { body: recordable };
      if (url.endsWith("/api/connections") && init?.method === "POST") {
        posted = JSON.parse(String(init.body));
        return { status: 201, body: { id: "conn-rec", name: posted?.name } };
      }
      return { body: [] };
    });

    const conns = useConnectionsStore();
    conns.plugins = [
      { name: "tester", title: "Tester", icon: { type: "name", value: "box" } },
    ];

    const wrapper = mount(ConnectionFormDialog, { props: { visible: true } });
    await flushPromises();
    await wrapper
      .findComponent(ProtocolPicker)
      .vm.$emit("update:modelValue", "tester");
    await flushPromises();

    // Dialog content is teleported, so assert via the component tree: the
    // protocol is a card picker (not a Select) and a single transport hides that
    // select, leaving just the one recording-policy select.
    const selects = wrapper.findAllComponents(Select);
    expect(selects).toHaveLength(1);
    const recordingSelect = selects[selects.length - 1];
    recordingSelect.vm.$emit("update:modelValue", "auto");
    await flushPromises();

    wrapper.findAllComponents(InputText)[0].vm.$emit("update:modelValue", "r1");
    await flushPromises();
    wrapper.findComponent(SchemaForm).vm.$emit("submit", { host: "10.0.0.1" });
    await flushPromises();

    expect(posted).toMatchObject({ recording: { terminal: "auto" } });
  });

  it("omits the recording section when the plugin declares no support", async () => {
    installFetch((url) => {
      if (url.endsWith("/api/plugins/tester")) return { body: projection };
      return { body: [] };
    });
    const conns = useConnectionsStore();
    conns.plugins = [
      { name: "tester", title: "Tester", icon: { type: "name", value: "box" } },
    ];
    const wrapper = mount(ConnectionFormDialog, { props: { visible: true } });
    await flushPromises();
    await wrapper
      .findComponent(ProtocolPicker)
      .vm.$emit("update:modelValue", "tester");
    await flushPromises();
    // Protocol is a card picker and there's no recording select for an
    // unsupported plugin, so no Select is rendered at all.
    expect(wrapper.findAllComponents(Select)).toHaveLength(0);
  });

  it("preserves unreadable credential refs when editing a shared connection", async () => {
    let updated: Record<string, unknown> | null = null;
    const withCredential: PluginProjection = {
      ...projection,
      config: {
        groups: [
          {
            name: "Basic",
            fields: [
              { key: "host", label: "Host", type: "text", required: true },
              {
                key: "credential_id",
                label: "Credential",
                type: "credential_ref",
                required: true,
                credential: { kinds: ["ssh_password"], protocols: ["tester"] },
              },
            ],
          },
        ],
      },
    };
    installFetch((url, init) => {
      if (url.endsWith("/api/connections/c1") && init?.method === "PUT") {
        updated = JSON.parse(String(init.body));
        return { body: { id: "c1", name: "shared" } };
      }
      if (url.endsWith("/api/connections/c1")) {
        return {
          body: {
            id: "c1",
            name: "shared",
            protocol: "tester",
            transport: "direct",
            config: { host: "10.0.0.1" },
            secrets: {},
            credentials: {
              credential_id: { state: "set", readable: false },
            },
          },
        };
      }
      if (url.endsWith("/api/plugins/tester")) return { body: withCredential };
      if (url.includes("/api/credentials")) return { body: [] };
      return { body: [] };
    });

    const conns = useConnectionsStore();
    conns.plugins = [
      { name: "tester", title: "Tester", icon: { type: "name", value: "box" } },
    ];

    const wrapper = mount(ConnectionFormDialog, {
      props: { visible: true, connectionId: "c1" },
    });
    await flushPromises();

    const form = wrapper.findComponent(SchemaForm);
    form.vm.$emit(
      "submit",
      { host: "10.0.0.2" },
      { preserveCredentials: ["credential_id"] },
    );
    await flushPromises();

    expect(updated).toMatchObject({
      config: { host: "10.0.0.2" },
      preserveCredentials: ["credential_id"],
    });
  });
});
