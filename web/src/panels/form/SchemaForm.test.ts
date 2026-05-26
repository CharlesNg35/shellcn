import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { installFetch } from "../../test/fetchMock";
import SchemaForm from "./SchemaForm.vue";
import type { Schema } from "../../types/projection";

const schema: Schema = {
  groups: [
    {
      name: "Basic",
      fields: [{ key: "host", label: "Host", type: "text", required: true }],
    },
    {
      name: "Auth",
      fields: [
        {
          key: "auth",
          label: "Auth",
          type: "select",
          default: "password",
          options: [
            { label: "Password", value: "password" },
            { label: "Credential", value: "credential" },
          ],
        },
        {
          key: "password",
          label: "Password",
          type: "password",
          secret: true,
          visibleWhen: {
            allOf: [{ field: "auth", op: "eq", value: "password" }],
          },
        },
        {
          key: "credential_id",
          label: "Credential",
          type: "credential_ref",
          credential: { kinds: ["ssh_password"], protocols: ["ssh"] },
          visibleWhen: {
            allOf: [{ field: "auth", op: "eq", value: "credential" }],
          },
        },
      ],
    },
  ],
};

beforeEach(() => {
  installFetch((url) =>
    url.includes("/credentials") ? { body: [] } : { body: {} },
  );
});
afterEach(() => vi.unstubAllGlobals());

describe("SchemaForm", () => {
  it("shows/hides fields by structured condition", async () => {
    const pw = mount(SchemaForm, {
      props: { schema, modelValue: { auth: "password" } },
    });
    await flushPromises();
    expect(pw.find('input[type="password"]').exists()).toBe(true);

    const cred = mount(SchemaForm, {
      props: { schema, modelValue: { auth: "credential" } },
    });
    await flushPromises();
    expect(cred.find('input[type="password"]').exists()).toBe(false);
    expect(cred.text()).toContain("Credential");
  });

  it("blocks submit and surfaces a validation error when required is empty", async () => {
    const w = mount(SchemaForm, { props: { schema, submitLabel: "Save" } });
    await w.find("form").trigger("submit");
    expect(w.emitted("submit")).toBeUndefined();
    expect(w.text()).toContain("required");
  });

  it("emits validated values on submit", async () => {
    const w = mount(SchemaForm, { props: { schema, submitLabel: "Save" } });
    await flushPromises();
    await w.findAll("input")[0].setValue("10.0.0.1");
    await w.find("form").trigger("submit");
    const submitted = w.emitted("submit");
    expect(submitted).toBeTruthy();
    expect((submitted?.[0][0] as Record<string, unknown>).host).toBe(
      "10.0.0.1",
    );
  });

  it("renders multiselect and duration fields without falling back to plain text", async () => {
    const w = mount(SchemaForm, {
      props: {
        schema: {
          groups: [
            {
              name: "Advanced",
              fields: [
                {
                  key: "roles",
                  label: "Roles",
                  type: "multiselect",
                  options: [
                    { label: "Read", value: "read" },
                    { label: "Write", value: "write" },
                  ],
                },
                { key: "ttl", label: "TTL", type: "duration" },
              ],
            },
          ],
        },
        submitLabel: "Save",
      },
    });
    await flushPromises();
    expect(w.findComponent({ name: "MultiSelect" }).exists()).toBe(true);
    expect(w.find('input[placeholder="30s, 5m, 1h"]').exists()).toBe(true);
  });

  it("renders number fields without locale digit grouping", async () => {
    const w = mount(SchemaForm, {
      props: {
        schema: {
          groups: [
            {
              name: "Basic",
              fields: [{ key: "port", label: "Port", type: "number" }],
            },
          ],
        },
        modelValue: { port: 6379 },
      },
    });
    await flushPromises();
    expect(w.findComponent({ name: "InputNumber" }).props("useGrouping")).toBe(
      false,
    );
  });

  it("omits hidden conditional fields from submit payloads", async () => {
    const w = mount(SchemaForm, {
      props: {
        schema,
        submitLabel: "Save",
        modelValue: {
          host: "10.0.0.1",
          auth: "credential",
          credential_id: "cred-1",
          password: "stale-secret",
        },
      },
    });
    await flushPromises();
    await w.find("form").trigger("submit");
    const payload = w.emitted("submit")?.[0][0] as Record<string, unknown>;
    expect(payload.auth).toBe("credential");
    expect(payload.credential_id).toBe("cred-1");
    expect(payload).not.toHaveProperty("password");
  });

  it("uses ambient context for visibility", async () => {
    const contextual: Schema = {
      groups: [
        {
          name: "Target",
          fields: [
            {
              key: "endpoint",
              label: "Endpoint",
              type: "text",
              required: true,
              visibleWhen: {
                allOf: [{ field: "$transport", op: "eq", value: "direct" }],
              },
            },
          ],
        },
      ],
    };
    const w = mount(SchemaForm, {
      props: {
        schema: contextual,
        submitLabel: "Save",
        modelValue: { endpoint: "127.0.0.1:2375" },
        context: { $transport: "agent" },
      },
    });
    await flushPromises();
    expect(w.text()).not.toContain("Endpoint");
    await w.find("form").trigger("submit");
    const payload = w.emitted("submit")?.[0][0] as Record<string, unknown>;
    expect(payload).not.toHaveProperty("endpoint");
  });

  it("keeps secret fields write-only (set/replace, value never in a readable field)", async () => {
    const w = mount(SchemaForm, {
      props: { schema, secretsSet: { password: true } },
    });
    expect(w.text()).toContain("Set");
    expect(w.text()).toContain("Replace");
    // No password input is shown until the user chooses to replace.
    expect(w.find('input[type="password"]').exists()).toBe(false);
  });

  it("does not require an already-set secret left untouched on edit", async () => {
    const requiredSecret: Schema = {
      groups: [
        {
          name: "Auth",
          fields: [
            {
              key: "token",
              label: "Token",
              type: "password",
              secret: true,
              required: true,
            },
          ],
        },
      ],
    };
    const w = mount(SchemaForm, {
      props: { schema: requiredSecret, secretsSet: { token: true } },
    });
    (w.vm as unknown as { submit: () => void }).submit();
    await flushPromises();
    const submitted = w.emitted("submit");
    expect(submitted).toBeTruthy();
    // The kept secret is neither blocked nor resubmitted.
    expect(submitted?.[0][0]).not.toHaveProperty("token");
  });

  it("preserves an unreadable configured credential without submitting its id", async () => {
    const requiredCredential: Schema = {
      groups: [
        {
          name: "Auth",
          fields: [
            {
              key: "credential_id",
              label: "Credential",
              type: "credential_ref",
              required: true,
              credential: { kinds: ["ssh_password"], protocols: ["ssh"] },
            },
          ],
        },
      ],
    };
    const w = mount(SchemaForm, {
      props: {
        schema: requiredCredential,
        credentialStates: {
          credential_id: { state: "set", readable: false },
        },
      },
    });

    (w.vm as unknown as { submit: () => void }).submit();
    await flushPromises();

    const submitted = w.emitted("submit");
    expect(submitted).toBeTruthy();
    expect(submitted?.[0][0]).not.toHaveProperty("credential_id");
    expect(submitted?.[0][1]).toEqual({
      preserveCredentials: ["credential_id"],
    });
  });

  it("requires a new credential after the user chooses to replace a hidden one", async () => {
    const requiredCredential: Schema = {
      groups: [
        {
          name: "Auth",
          fields: [
            {
              key: "credential_id",
              label: "Credential",
              type: "credential_ref",
              required: true,
              credential: { kinds: ["ssh_password"], protocols: ["ssh"] },
            },
          ],
        },
      ],
    };
    const w = mount(SchemaForm, {
      props: {
        schema: requiredCredential,
        credentialStates: {
          credential_id: { state: "set", readable: false },
        },
      },
    });
    await flushPromises();

    await w.find("button").trigger("click");
    (w.vm as unknown as { submit: () => void }).submit();
    await flushPromises();

    expect(w.emitted("submit")).toBeUndefined();
    expect(w.text()).toContain("required");
  });
});
