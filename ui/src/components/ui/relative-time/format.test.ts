import { describe, expect, it } from "vitest";

import { formatDateTime, formatRelativeTime } from "@/components/ui/relative-time/format.ts";

const now = new Date("2026-11-22T12:00:00Z");

function ago(seconds: number) {
  return new Date(now.getTime() - seconds * 1000);
}

describe("formatRelativeTime", () => {
  it.each([
    [0, "now"],
    [59, "now"],
    [60, "1 minute ago"],
    [3 * 60 * 60, "3 hours ago"],
    [24 * 60 * 60, "yesterday"],
    [3 * 24 * 60 * 60, "3 days ago"],
    [8 * 24 * 60 * 60, "last week"],
    [40 * 24 * 60 * 60, "last month"],
    [800 * 24 * 60 * 60, "2 years ago"],
    [-2 * 60 * 60, "in 2 hours"],
    [-3 * 24 * 60 * 60, "in 3 days"],
  ])("of %i seconds ago is %s", (seconds, want) => {
    expect(formatRelativeTime(ago(seconds), "en", now)).toBe(want);
  });

  it("follows the locale", () => {
    expect(formatRelativeTime(ago(3 * 24 * 60 * 60), "de", now)).toBe("vor 3 Tagen");
  });
});

describe("formatDateTime", () => {
  it("writes the date and the time in the locale", () => {
    const date = new Date(2026, 10, 22, 15, 4);
    expect(formatDateTime(date, "en")).toBe("Nov 22, 2026, 3:04 PM");
    expect(formatDateTime(date, "de")).toBe("22.11.2026, 15:04");
  });
});
