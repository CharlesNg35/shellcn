import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { installFetch } from "../../test/fetchMock";
import CredentialFormDialog from "../../components/CredentialFormDialog.vue";
import CredentialSelect from "./CredentialSelect.vue";

const credentialKinds = [
  {
    kind: "ssh_private_key",
    label: "SSH private key",
    secretLabel: "Private key",
    secretMultiline: true,
    identityLabel: "Username",
    compatibleProtocols: ["ssh", "sftp"],
  },
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
    compatibleProtocols: ["postgresql"],
  },
  {
    kind: "db_password",
    label: "Database password",
    secretLabel: "Password",
    identityLabel: "Database user",
    compatibleProtocols: ["postgresql"],
  },
];

beforeEach(() => {
  setActivePinia(createPinia());
  installFetch((url) => {
    if (url.includes("/credential-kinds")) return { body: credentialKinds };
    if (url.includes("/credentials")) return { body: [] };
    return { body: {} };
  });
});
afterEach(() => vi.unstubAllGlobals());

describe("CredentialSelect", () => {
  it("loads selectable credentials using the manifest selector kinds and protocol", async () => {
    const calls: string[] = [];
    vi.unstubAllGlobals();
    installFetch((url) => {
      calls.push(url);
      if (url.includes("/credential-kinds")) return { body: credentialKinds };
      if (url.includes("/credentials")) return { body: [] };
      return { body: {} };
    });

    mount(CredentialSelect, {
      props: {
        protocol: "ssh",
        selector: {
          kinds: ["ssh_private_key", "ssh_password"],
          protocols: ["ssh"],
        },
      },
    });
    await flushPromises();

    const credentialCall = calls.find((url) => url.includes("/credentials?"));
    expect(credentialCall).toContain("kind=ssh_private_key%2Cssh_password");
    expect(credentialCall).toContain("protocol=ssh");
  });

  it("reloads selectable credentials when the selected protocol changes", async () => {
    const calls: string[] = [];
    vi.unstubAllGlobals();
    installFetch((url) => {
      calls.push(url);
      if (url.includes("/credentials")) return { body: [] };
      return { body: {} };
    });

    const wrapper = mount(CredentialSelect, {
      props: {
        protocol: "ssh",
        selector: {
          kinds: ["ssh_password"],
          protocols: ["ssh", "sftp"],
        },
      },
    });
    await flushPromises();
    await wrapper.setProps({ protocol: "sftp" });
    await flushPromises();

    const credentialCalls = calls.filter((url) =>
      url.includes("/credentials?"),
    );
    expect(credentialCalls.at(-1)).toContain("protocol=sftp");
  });

  it("opens one create dialog scoped to the manifest selector", async () => {
    const wrapper = mount(CredentialSelect, {
      props: {
        protocol: "ssh",
        selector: {
          kinds: ["ssh_private_key", "ssh_password"],
          protocols: ["ssh"],
        },
      },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("New credential");
    expect(wrapper.text()).not.toContain("New ssh private key");
    expect(wrapper.text()).not.toContain("New ssh password");

    await wrapper.find("button").trigger("click");
    await flushPromises();

    const dialog = wrapper.findComponent(CredentialFormDialog);
    expect(dialog.props("selector")).toMatchObject({
      kinds: ["ssh_private_key", "ssh_password"],
      protocols: ["ssh"],
    });
    expect(dialog.props("protocol")).toBe("ssh");
  });

  it("labels mixed-kind credential choices with credential kind display names", async () => {
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.includes("/credential-kinds")) return { body: credentialKinds };
      if (url.includes("/credentials")) {
        return {
          body: [
            {
              id: "db-pw",
              name: "database prod",
              kind: "db_password",
              identity: "app",
            },
            {
              id: "db-cert",
              name: "database cert",
              kind: "tls_client_cert",
            },
          ],
        };
      }
      return { body: {} };
    });

    const wrapper = mount(CredentialSelect, {
      props: {
        protocol: "postgresql",
        selector: {
          kinds: ["db_password", "tls_client_cert"],
          protocols: ["postgresql"],
        },
      },
    });
    await flushPromises();

    const select = wrapper.findComponent({ name: "Select" });
    expect(select.props("options")).toEqual([
      {
        value: "db-pw",
        label: "database prod · Database password (app)",
      },
      {
        value: "db-cert",
        label: "database cert · TLS client certificate",
      },
    ]);
  });

  it("shows an unreadable configured credential as replace-only", async () => {
    const wrapper = mount(CredentialSelect, {
      props: {
        protocol: "ssh",
        selector: {
          kinds: ["ssh_password"],
          protocols: ["ssh"],
        },
        state: { state: "set", readable: false },
      },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("Credential configured");
    expect(wrapper.text()).toContain("Replace");
    expect(wrapper.findComponent({ name: "Select" }).exists()).toBe(false);

    await wrapper.find("button").trigger("click");
    await flushPromises();

    expect(wrapper.emitted("update:modelValue")?.[0]).toEqual([""]);
    expect(wrapper.findComponent({ name: "Select" }).exists()).toBe(true);
  });
});
