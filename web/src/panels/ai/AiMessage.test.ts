import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import AiMessage from "./AiMessage.vue";
import type { AiMessage as ChatMessage } from "../../stores/aiChat";

function message(content: string): ChatMessage {
  return {
    id: "m1",
    role: "assistant",
    content,
    reasoning: "",
    toolCalls: [],
  };
}

function bubbleClasses(content: string): string[] {
  const wrapper = mount(AiMessage, {
    props: { message: message(content), streaming: false },
    global: {
      stubs: {
        AiMarkdown: true,
        AiReasoning: true,
        AiToolBadges: true,
        AppIcon: true,
        Button: true,
        Message: true,
      },
    },
  });

  return wrapper.find('[data-role="assistant"] > div').classes();
}

describe("AiMessage", () => {
  it("caps assistant width but lets bubbles hug their content", () => {
    const tableClasses = bubbleClasses(
      [
        "| Point | Teaching | References |",
        "| --- | --- | --- |",
        "| A | B | C |",
      ].join("\n"),
    );
    expect(tableClasses).toContain("max-w-[88%]");
    expect(tableClasses).not.toContain("w-[88%]");

    const shortClasses = bubbleClasses("Short answer.");
    expect(shortClasses).toContain("max-w-[88%]");
    expect(shortClasses).not.toContain("w-[88%]");
  });
});
