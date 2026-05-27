import { describe, expect, it } from "vitest";
import { buildSqlSchema } from "./codemirror";

describe("buildSqlSchema", () => {
  it("groups columns under their table and schema for context-aware completion", () => {
    const schema = buildSqlSchema([
      { label: "public", type: "namespace", detail: "schema" },
      { label: "users", type: "table", detail: "public" },
      { label: "id", type: "property", detail: "public.users" },
      { label: "email", type: "property", detail: "public.users" },
      { label: "SELECT", type: "keyword" },
    ]);
    // Unqualified table -> columns (for `users.` completion).
    expect(schema.users).toEqual(["id", "email"]);
    // Schema -> table -> columns (for `public.users.` completion).
    expect(schema.public).toEqual({ users: ["id", "email"] });
  });

  it("ignores non-relational catalog entries", () => {
    const schema = buildSqlSchema([{ label: "SELECT", type: "keyword" }]);
    expect(Object.keys(schema)).toHaveLength(0);
  });
});
