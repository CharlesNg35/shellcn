import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import InputText from "primevue/inputtext";
import Button from "primevue/button";
import { installFetch } from "../test/fetchMock";
import ConnectionFolderDialog from "./ConnectionFolderDialog.vue";

beforeEach(() => {
  setActivePinia(createPinia());
});
afterEach(() => vi.unstubAllGlobals());

describe("ConnectionFolderDialog", () => {
  it("creates a folder with a curated color token", async () => {
    let posted: Record<string, unknown> | null = null;
    installFetch((url, init) => {
      if (url.endsWith("/api/connection-folders") && init?.method === "POST") {
        posted = JSON.parse(String(init.body));
        return {
          status: 201,
          body: {
            id: "f1",
            name: posted?.name,
            color: posted?.color,
            sortOrder: 0,
          },
        };
      }
      return { body: [] };
    });

    const wrapper = mount(ConnectionFolderDialog, { props: { visible: true } });
    await flushPromises();

    wrapper
      .findComponent(InputText)
      .vm.$emit("update:modelValue", "Production");
    await flushPromises();
    await wrapper
      .findAllComponents(Button)
      .find((b) => b.attributes("title") === "Teal")
      ?.trigger("click");
    await flushPromises();
    await wrapper
      .findAllComponents(Button)
      .find((b) => b.text().includes("Create folder"))
      ?.trigger("click");
    await flushPromises();

    expect(posted).toMatchObject({ name: "Production", color: "teal" });
    expect(wrapper.emitted("saved")?.[0][0]).toMatchObject({
      id: "f1",
      color: "teal",
    });
  });
});
