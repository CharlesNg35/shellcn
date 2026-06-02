import { describe, it, expect, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import { defineComponent, h } from "vue";
import Button from "primevue/button";
import ConfirmDialog from "primevue/confirmdialog";
import InputText from "primevue/inputtext";
import AiConversationList from "./AiConversationList.vue";
import type { AiConversation } from "../../api/ai";

const conversation = (over: Partial<AiConversation> = {}): AiConversation => ({
  id: "cv-1",
  ownerId: "u1",
  connectionId: "conn-1",
  title: "Current title",
  autoTitled: false,
  providerId: "",
  model: "gpt-4o",
  createdAt: "",
  updatedAt: "",
  ...over,
});

const Harness = defineComponent({
  setup() {
    return () =>
      h("div", [
        h(AiConversationList, {
          conversations: [conversation()],
          activeId: "cv-1",
          streamingId: null,
          busy: false,
        }),
        h(ConfirmDialog),
      ]);
  },
});

describe("AiConversationList", () => {
  it("renames through a PrimeVue input dialog instead of a browser prompt", async () => {
    const prompt = vi.spyOn(window, "prompt");
    const wrapper = mount(Harness);

    await wrapper.get('[aria-label="Rename"]').trigger("click");
    await flushPromises();
    wrapper.findComponent(InputText).vm.$emit("update:modelValue", "Renamed");
    await flushPromises();
    await wrapper
      .findAllComponents(Button)
      .find((b) => b.text().trim() === "Rename")
      ?.trigger("click");

    const list = wrapper.findComponent(AiConversationList);
    expect(prompt).not.toHaveBeenCalled();
    expect(list.emitted("rename")?.[0]).toEqual(["cv-1", "Renamed"]);
    prompt.mockRestore();
  });

  it("emits the conversation id when delete is confirmed", async () => {
    const wrapper = mount(Harness);

    await wrapper.get('[aria-label="Delete"]').trigger("click");
    await flushPromises();
    await wrapper
      .findAllComponents(Button)
      .find((b) => b.text().trim() === "Delete")
      ?.trigger("click");
    await flushPromises();

    const list = wrapper.findComponent(AiConversationList);
    expect(list.emitted("remove")?.[0]).toEqual(["cv-1"]);
  });
});
