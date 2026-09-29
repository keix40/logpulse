import { describe, expect, it } from "vitest";
import { levelColor } from "./logLevels";

describe("levelColor", () => {
  it("maps error to red tone class", () => {
    expect(levelColor("error")).toContain("red");
  });

  it("is case insensitive", () => {
    expect(levelColor("INFO")).toBe(levelColor("info"));
  });
});
