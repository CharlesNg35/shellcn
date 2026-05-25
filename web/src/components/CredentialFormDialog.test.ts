import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import Button from "primevue/button";
import { installFetch } from "../test/fetchMock";
import CredentialFormDialog from "./CredentialFormDialog.vue";

const credentialKinds = [
  {
    kind: "ssh_password",
    label: "SSH password",
    secretLabel: "Password",
    identityLabel: "Username",
    compatibleProtocols: ["ssh", "sftp"],
  },
  {
    kind: "tls_client_cert",
    label: "TLS client certificate",
    secretLabel: "Certificate and private key",
    secretMultiline: true,
    compatibleProtocols: ["docker"],
  },
  {
    kind: "kubeconfig",
    label: "Kubeconfig",
    secretLabel: "Kubeconfig YAML",
    secretMultiline: true,
    identityLabel: "Context / user",
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
      secret: "",
    });
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
        selector: { kinds: ["kubeconfig"], protocols: ["kubernetes"] },
        protocol: "kubernetes",
        lockedKind: "kubeconfig",
      },
    });
    await flushPromises();

    expect(document.body.textContent).toContain("Kubeconfig");
    expect(document.body.textContent).not.toContain("SSH password");
    expect(wrapper.findComponent({ name: "Select" }).exists()).toBe(false);
    wrapper.unmount();
  });

  it("locks inline creation to the passed kind even when the field accepts multiple kinds", async () => {
    installFetch((url) =>
      url.includes("/credential-kinds")
        ? { body: credentialKinds }
        : { body: [] },
    );
    const wrapper = mount(CredentialFormDialog, {
      props: {
        visible: true,
        selector: {
          kinds: ["ssh_password", "kubeconfig"],
          protocols: ["ssh", "kubernetes"],
        },
        protocol: "kubernetes",
        lockedKind: "kubeconfig",
      },
    });
    await flushPromises();

    expect(document.body.textContent).toContain("Kubeconfig");
    expect(document.body.textContent).not.toContain("SSH password");
    expect(wrapper.findComponent({ name: "Select" }).exists()).toBe(false);
    wrapper.unmount();
  });

  it("hides identity metadata for credential kinds that do not use it", async () => {
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
    expect(document.body.textContent).toContain("Certificate and private key");
    wrapper.unmount();
  });
});
