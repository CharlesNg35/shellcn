import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import Button from "primevue/button";
import { installFetch } from "../test/fetchMock";
import CredentialFormDialog from "./CredentialFormDialog.vue";
import ShareDialog from "./ShareDialog.vue";

const credentialKinds = [
  {
    kind: "ssh_password",
    label: "SSH password",
    fields: [
      {
        key: "username",
        label: "Username",
        type: "text",
        required: true,
        public: true,
      },
      {
        key: "password",
        label: "Password",
        type: "password",
        required: true,
        secret: true,
      },
    ],
    compatibleProtocols: ["ssh", "sftp"],
  },
  {
    kind: "tls_client_cert",
    label: "TLS client certificate",
    fields: [
      {
        key: "certificate",
        label: "Client certificate",
        type: "textarea",
        required: true,
        secret: true,
      },
      {
        key: "private_key",
        label: "Private key",
        type: "textarea",
        required: true,
        secret: true,
      },
    ],
    compatibleProtocols: ["docker"],
  },
  {
    kind: "kubeconfig",
    label: "Kubeconfig",
    fields: [
      {
        key: "context",
        label: "Context / user",
        type: "text",
        public: true,
      },
      {
        key: "kubeconfig",
        label: "Kubeconfig YAML",
        type: "textarea",
        required: true,
        secret: true,
      },
    ],
    compatibleProtocols: ["kubernetes"],
  },
];

beforeEach(() => setActivePinia(createPinia()));
afterEach(() => vi.unstubAllGlobals());

describe("CredentialFormDialog", () => {
  it("rotates an existing secret behind a write-only Replace affordance", async () => {
    let put: Record<string, unknown> | null = null;
    installFetch((url, init) => {
      if (url.includes("/credential-kinds")) return { body: credentialKinds };
      if (url.includes("/credentials/c1") && init?.method === "PUT") {
        put = JSON.parse(String(init.body));
        return { body: { id: "c1" } };
      }
      return { body: {} };
    });

    const wrapper = mount(CredentialFormDialog, {
      props: {
        visible: true,
        credential: {
          id: "c1",
          name: "ops key",
          kind: "ssh_password",
          values: { username: "ops" },
          ownerId: "u-demo",
        },
      },
    });
    await flushPromises();

    // The secret is write-only: a "Replace" affordance, not a populated input.
    expect(document.body.textContent).toContain("Replace");
    expect(document.body.querySelector('input[type="password"]')).toBeFalsy();

    // Saving without replacing keeps the stored secret (blank secret in body).
    const save = wrapper
      .findAllComponents(Button)
      .find((b) => b.text().includes("Save changes"));
    await save?.trigger("click");
    await flushPromises();

    expect(put).toMatchObject({
      name: "ops key",
      kind: "ssh_password",
      values: { username: "ops" },
    });
    expect(put).not.toHaveProperty("protocols");
    expect(wrapper.emitted("saved")).toBeTruthy();
    wrapper.unmount();
  });

  it("requires secret material when creating", async () => {
    const fetchFn = installFetch((url) =>
      url.includes("/credential-kinds")
        ? { body: credentialKinds }
        : { status: 201, body: { id: "new" } },
    );
    const wrapper = mount(CredentialFormDialog, {
      props: { visible: true, credential: null },
    });
    await flushPromises();

    const save = wrapper
      .findAllComponents(Button)
      .find((b) => b.text().includes("Create credential"));
    await save?.trigger("click");
    await flushPromises();

    // No POST happens because name + secret are required and empty.
    const posted = fetchFn.mock.calls.find(
      ([, i]) => (i as RequestInit | undefined)?.method === "POST",
    );
    expect(posted).toBeUndefined();
    expect(document.body.textContent).toContain("required");
    wrapper.unmount();
  });

  it("limits inline creation to the selector's credential kinds", async () => {
    installFetch((url) =>
      url.includes("/credential-kinds")
        ? { body: credentialKinds }
        : { body: [] },
    );
    const wrapper = mount(CredentialFormDialog, {
      props: {
        visible: true,
        selector: { kind: "kubeconfig", protocols: ["kubernetes"] },
        protocol: "kubernetes",
      },
    });
    await flushPromises();

    expect(document.body.textContent).toContain("Kubeconfig");
    expect(document.body.textContent).toContain("kubernetes");
    expect(document.body.textContent).not.toContain("SSH password");
    expect(wrapper.findComponent({ name: "Select" }).exists()).toBe(false);
    wrapper.unmount();
  });

  it("scopes inline creation to the selector kind", async () => {
    installFetch((url) =>
      url.includes("/credential-kinds")
        ? { body: credentialKinds }
        : { body: [] },
    );
    const wrapper = mount(CredentialFormDialog, {
      props: {
        visible: true,
        selector: { kind: "kubeconfig", protocols: ["kubernetes"] },
        protocol: "kubernetes",
      },
    });
    await flushPromises();

    expect(document.body.textContent).toContain("Kubeconfig");
    expect(document.body.textContent).not.toContain("SSH password");
    expect(wrapper.findComponent({ name: "Select" }).exists()).toBe(false);
    wrapper.unmount();
  });

  it("renders only declared credential fields", async () => {
    installFetch((url) =>
      url.includes("/credential-kinds")
        ? { body: credentialKinds }
        : { body: [] },
    );
    const wrapper = mount(CredentialFormDialog, {
      props: {
        visible: true,
        credential: {
          id: "tls",
          name: "docker cert",
          kind: "tls_client_cert",
          ownerId: "u-demo",
        },
      },
    });
    await flushPromises();

    expect(document.body.textContent).not.toContain("Username");
    expect(document.body.textContent).toContain("Client certificate");
    expect(document.body.textContent).toContain("Private key");
    wrapper.unmount();
  });

  it("opens credential sharing from the credential dialog", async () => {
    installFetch((url) => {
      if (url.includes("/credential-kinds")) return { body: credentialKinds };
      if (url.includes("/credentials/c1/grants")) return { body: [] };
      return { body: [] };
    });
    const wrapper = mount(CredentialFormDialog, {
      props: {
        visible: true,
        credential: {
          id: "c1",
          name: "ops key",
          kind: "ssh_password",
          ownerId: "u-demo",
        },
      },
    });
    await flushPromises();

    const share = wrapper
      .findAllComponents(Button)
      .find((b) => b.text().includes("Share"));
    await share?.trigger("click");
    await flushPromises();

    const dialog = wrapper.findComponent(ShareDialog);
    expect(dialog.exists()).toBe(true);
    expect(dialog.props()).toMatchObject({
      resource: "credentials",
      resourceId: "c1",
      resourceName: "ops key",
    });
    wrapper.unmount();
  });
});
