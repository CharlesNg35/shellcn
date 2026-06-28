import { describe, it, expect } from "vitest";
import {
  groupConversations,
  relativeTimeLabel,
} from "./conversationGroups";
import type { AiConversation } from "@/api/ai";

const NOW = Date.parse("2026-03-15T12:00:00Z");

function conv(id: string, updatedAt: string): AiConversation {
  return {
    id,
    ownerId: "u1",
    connectionId: "c1",
    title: id,
    titleResolved: false,
    providerId: "",
    model: "",
    createdAt: updatedAt,
    updatedAt,
  };
}

describe("groupConversations", () => {
  it("buckets by recency, newest first, dropping empty groups", () => {
    const groups = groupConversations(
      [
        conv("today", "2026-03-15T08:00:00Z"),
        conv("yesterday", "2026-03-14T09:00:00Z"),
        conv("lastWeek", "2026-03-11T09:00:00Z"),
        conv("lastMonth", "2026-02-20T09:00:00Z"),
        conv("ancient", "2025-12-01T09:00:00Z"),
      ],
      NOW,
    );
    expect(groups.map((g) => g.key)).toEqual([
      "today",
      "yesterday",
      "week",
      "month",
      "older",
    ]);
    expect(groups[0].items[0].id).toBe("today");
  });

  it("orders items within a group newest first", () => {
    const groups = groupConversations(
      [
        conv("early", "2026-03-15T06:00:00Z"),
        conv("late", "2026-03-15T11:00:00Z"),
      ],
      NOW,
    );
    expect(groups[0].items.map((c) => c.id)).toEqual(["late", "early"]);
  });

  it("returns no groups for an empty list", () => {
    expect(groupConversations([], NOW)).toEqual([]);
  });
});

describe("relativeTimeLabel", () => {
  it("formats recent timestamps relatively", () => {
    expect(relativeTimeLabel(conv("a", "2026-03-15T11:59:30Z"), NOW)).toBe(
      "Just now",
    );
    expect(relativeTimeLabel(conv("a", "2026-03-15T11:30:00Z"), NOW)).toBe(
      "30m ago",
    );
    expect(relativeTimeLabel(conv("a", "2026-03-15T09:00:00Z"), NOW)).toBe(
      "3h ago",
    );
    expect(relativeTimeLabel(conv("a", "2026-03-13T12:00:00Z"), NOW)).toBe(
      "2d ago",
    );
  });

  it("falls back to a calendar date beyond a week", () => {
    expect(relativeTimeLabel(conv("a", "2026-02-01T12:00:00Z"), NOW)).toMatch(
      /Feb/,
    );
  });

  it("returns an empty string when the timestamp is missing", () => {
    expect(relativeTimeLabel(conv("a", ""), NOW)).toBe("");
  });
});
