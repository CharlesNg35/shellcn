import { describe, expect, it } from "vitest";
import { ref } from "vue";
import { useLiveCollection } from "./useLiveCollection";
import type { ResourceEvent, Row } from "@/types/projection";

const row = (uid: string, label = uid): Row => ({
  ref: { kind: "thing", name: uid, uid },
  label,
});
const ev = (type: ResourceEvent["type"], uid: string): ResourceEvent => ({
  type,
  ref: { kind: "thing", name: uid, uid },
  resource: row(uid),
});
const keyOf = (r: Row) => (r.ref as { uid: string }).uid;

describe("useLiveCollection", () => {
  it("prepends new entries and upserts by uid", () => {
    const rows = ref<Row[]>([row("a")]);
    const { apply } = useLiveCollection({ rows, keyOf, prepend: true });
    apply(ev("added", "b"));
    expect(rows.value.map(keyOf)).toEqual(["b", "a"]);
    apply({
      type: "updated",
      ref: { kind: "thing", name: "a", uid: "a" },
      resource: row("a", "A2"),
    });
    expect(rows.value.find((r) => keyOf(r) === "a")?.label).toBe("A2");
  });

  it("removes on delete and honors max", () => {
    const rows = ref<Row[]>([row("a"), row("b")]);
    const { apply } = useLiveCollection({ rows, keyOf, prepend: true, max: 2 });
    apply(ev("deleted", "a"));
    expect(rows.value.map(keyOf)).toEqual(["b"]);
    apply(ev("added", "c"));
    apply(ev("added", "d"));
    expect(rows.value.map(keyOf)).toEqual(["d", "c"]); // capped at 2
  });
});
