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
});
