import { describe, it, expect } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import { defineComponent, h } from "vue";
import Button from "primevue/button";
import ConfirmDialog from "primevue/confirmdialog";
import InputText from "primevue/inputtext";
import AiConversationList from "./AiConversationList.vue";
import type { AiConversation } from "@/api/ai";

const conversation = (over: Partial<AiConversation> = {}): AiConversation => ({
  id: "cv-1",
  ownerId: "u1",
  connectionId: "conn-1",
  title: "Current title",
  titleResolved: false,
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
  it("renames inline in the conversation title row", async () => {
    const wrapper = mount(Harness);

    await wrapper.get('[aria-label="Rename"]').trigger("click");
    await flushPromises();
    expect(wrapper.findComponent(InputText).exists()).toBe(true);
    expect(
      wrapper
        .findAllComponents(Button)
        .some((b) => b.text().trim() === "Delete"),
    ).toBe(false);

    wrapper.findComponent(InputText).vm.$emit("update:modelValue", "Renamed");
    await flushPromises();
    await wrapper.get("form").trigger("submit");

    const list = wrapper.findComponent(AiConversationList);
    expect(list.emitted("rename")?.[0]).toEqual(["cv-1", "Renamed"]);
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
