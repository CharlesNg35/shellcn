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

// Per-user drill-down moved to UserRecordings; RecordingsView only reads
// ?connection= and shows My/All. A stray ?user= must be ignored here.
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
  it("shows only the viewer's own recordings, even for an admin", async () => {
    const wrapper = mount(RecordingsView);
    await flushPromises();

    // ?user= is ignored: recordings are private to their creator.
    expect(list).toHaveBeenCalledWith({});
    const text = wrapper.text();
    expect(text).toContain("prod-ssh");
    expect(text).toContain("kiosk-vnc");
    expect(text).toContain("My Recordings");
    expect(text).toContain("Terminal");
    expect(text).toContain("Desktop");

    // Only the finalized recording is playable.
    expect(wrapper.findAll('[aria-label="Play recording"]')).toHaveLength(1);
  });

  it("scopes the list to the viewer for non-admins too", async () => {
    const auth = useAuthStore();
    auth.user = { id: "op", username: "op", roles: ["operator"] };

    const wrapper = mount(RecordingsView);
    await flushPromises();

    expect(list).toHaveBeenCalledWith({});
    expect(wrapper.text()).toContain("My Recordings");
  });
});
