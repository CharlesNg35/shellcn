import { describe, expect, it } from "vitest";
import { toCSV, toJSON } from "./exportData";

describe("exportData", () => {
  it("escapes CSV fields with commas, quotes, and newlines", () => {
    const csv = toCSV(
      ["id", "note"],
      [
        [1, "plain"],
        [2, 'has "quote", comma'],
        [3, "line\nbreak"],
      ],
    );
    const lines = csv.split("\r\n");
    expect(lines[0]).toBe("id,note");
    expect(lines[1]).toBe("1,plain");
    expect(lines[2]).toBe('2,"has ""quote"", comma"');
    expect(lines[3]).toBe('3,"line\nbreak"');
  });

  it("renders objects and nulls predictably in CSV", () => {
    const csv = toCSV(["a", "b"], [[null, { x: 1 }]]);
    expect(csv.split("\r\n")[1]).toBe(',"{""x"":1}"');
  });

  it("maps matrix rows to keyed JSON objects", () => {
    const json = JSON.parse(toJSON(["id", "name"], [[1, "alice"]]));
    expect(json).toEqual([{ id: 1, name: "alice" }]);
  });
});
