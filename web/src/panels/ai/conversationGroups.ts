import type { AiConversation } from "@/api/ai";

export interface ConversationGroup {
  key: string;
  label: string;
  items: AiConversation[];
}

const DAY = 86_400_000;

const BUCKETS: {
  key: string;
  label: string;
  within: (age: number) => boolean;
}[] = [
  { key: "today", label: "Today", within: (age) => age <= 0 },
  { key: "yesterday", label: "Yesterday", within: (age) => age <= DAY },
  { key: "week", label: "Previous 7 days", within: (age) => age <= 7 * DAY },
  { key: "month", label: "Previous 30 days", within: (age) => age <= 30 * DAY },
  { key: "older", label: "Older", within: () => true },
];

export function conversationTimestamp(c: AiConversation): number {
  const t = Date.parse(c.updatedAt || c.createdAt || "");
  return Number.isNaN(t) ? 0 : t;
}

function startOfDay(ts: number): number {
  const d = new Date(ts);
  d.setHours(0, 0, 0, 0);
  return d.getTime();
}

/** Bucket conversations into recency groups, newest first, dropping empties. */
export function groupConversations(
  conversations: AiConversation[],
  now: number = Date.now(),
): ConversationGroup[] {
  const today = startOfDay(now);
  const groups: ConversationGroup[] = BUCKETS.map((b) => ({
    key: b.key,
    label: b.label,
    items: [],
  }));
  const sorted = [...conversations].sort(
    (a, b) => conversationTimestamp(b) - conversationTimestamp(a),
  );
  for (const c of sorted) {
    const age = today - startOfDay(conversationTimestamp(c));
    const index = BUCKETS.findIndex((b) => b.within(age));
    groups[index].items.push(c);
  }
  return groups.filter((g) => g.items.length > 0);
}

/** Compact relative label (e.g. "5m ago", "3d ago", "Mar 4"); "" when unknown. */
export function relativeTimeLabel(
  c: AiConversation,
  now: number = Date.now(),
): string {
  const t = conversationTimestamp(c);
  if (!t) return "";
  const sec = Math.floor((now - t) / 1000);
  if (sec < 60) return "Just now";
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const day = Math.floor(hr / 24);
  if (day < 7) return `${day}d ago`;
  return new Date(t).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
  });
}
