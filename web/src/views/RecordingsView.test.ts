import { describe, it, expect, vi, beforeEach } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import { useAuthStore } from "../stores/auth";
import type { RecordingSummary } from "../types/projection";

const list = vi.fn<(...a: unknown[]) => Promise<RecordingSummary[]>>(
  async () => [],
);
vi.mock("../api/recordings", () => ({
  recordingsApi: {
    list: (...args: unknown[]) => list(...args),
    contentUrl: (id: string) => `/api/recordings/${id}/content`,
    remove: vi.fn(),
  },
}));

// useRoute drives the filter (admin per-user drill-down via ?user=).
vi.mock("vue-router", () => ({
  useRoute: () => ({ query: { user: "u9" } }),
  RouterLink: { template: "<a><slot /></a>" },
}));

import RecordingsView from "./RecordingsView.vue";

const rows: RecordingSummary[] = [
  {
    id: "r1",
    userId: "u9",
    connectionId: "c1",
    connectionName: "prod-ssh",
    protocol: "ssh",
    class: "terminal",
    format: "asciicast_v2",
    authoritative: true,
    status: "finalized",
    startedAt: new Date().toISOString(),
    durationMs: 5000,
    size: 2048,
  },
  {
    id: "r2",
    userId: "u9",
    connectionId: "c2",
    connectionName: "kiosk-vnc",
    protocol: "vnc",
    class: "desktop",
    format: "webm_canvas",
    authoritative: false,
    status: "active",
    startedAt: new Date().toISOString(),
    durationMs: 0,
    size: 0,
  },
];

beforeEach(() => {
  setActivePinia(createPinia());
  const auth = useAuthStore();
  auth.user = { id: "admin", username: "admin", roles: ["admin"] };
  list.mockClear();
  list.mockResolvedValue(rows);
});

describe("RecordingsView", () => {
  it("lists recordings with the route filter and shows type + play affordances", async () => {
    const wrapper = mount(RecordingsView);
    await flushPromises();

    expect(list).toHaveBeenCalledWith({ user: "u9" });
    const text = wrapper.text();
    expect(text).toContain("prod-ssh");
    expect(text).toContain("kiosk-vnc");
    expect(text).toContain("User Recordings");
    expect(text).toContain("Terminal");
    expect(text).toContain("Desktop");

    // Only the finalized recording is playable.
    expect(wrapper.findAll('[aria-label="Play recording"]')).toHaveLength(1);
  });

  it("ignores per-user route filters for non-admin users", async () => {
    const auth = useAuthStore();
    auth.user = { id: "op", username: "op", roles: ["operator"] };

    const wrapper = mount(RecordingsView);
    await flushPromises();

    expect(list).toHaveBeenCalledWith({});
    expect(wrapper.text()).toContain("My Recordings");
  });
});
