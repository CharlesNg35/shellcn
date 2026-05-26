import { describe, it, expect, vi, beforeEach } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";

const create = vi.fn<
  (src: unknown, el: unknown, opts?: unknown) => { dispose: () => void }
>(() => ({ dispose: vi.fn() }));
vi.mock("asciinema-player", () => ({ create }));
vi.mock("asciinema-player/dist/bundle/asciinema-player.css", () => ({}));

import CastPlayer from "./CastPlayer.vue";

beforeEach(() => create.mockClear());

describe("CastPlayer", () => {
  it("mounts the asciinema player against the recording source", async () => {
    mount(CastPlayer, { props: { src: "/api/recordings/r1/content" } });
    await flushPromises();

    expect(create).toHaveBeenCalledTimes(1);
    const [src, el, opts] = create.mock.calls[0];
    expect(src).toBe("/api/recordings/r1/content");
    expect(el).toBeInstanceOf(HTMLElement);
    expect(opts).toMatchObject({ fit: "width", idleTimeLimit: 2, speed: 1 });
  });

  it("disposes the player on unmount", async () => {
    const dispose = vi.fn();
    create.mockReturnValueOnce({ dispose });
    const wrapper = mount(CastPlayer, {
      props: { src: "/api/recordings/r1/content" },
    });
    await flushPromises();
    wrapper.unmount();
    expect(dispose).toHaveBeenCalled();
  });
});
