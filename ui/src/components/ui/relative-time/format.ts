// The units of a relative time, from the largest, with their length in seconds: a month is 30
// days and a year 365, which is close enough for "3 months ago".
const units: Array<[Intl.RelativeTimeFormatUnit, number]> = [
  ["year", 365 * 24 * 60 * 60],
  ["month", 30 * 24 * 60 * 60],
  ["week", 7 * 24 * 60 * 60],
  ["day", 24 * 60 * 60],
  ["hour", 60 * 60],
  ["minute", 60],
];

// How long ago, or in how long, date is from now, in the largest unit that fits, rounded down, as
// the locale says it: "3 days ago", "in 2 hours", "yesterday", "now".
export function formatRelativeTime(date: Date, locale: string, now: Date = new Date()): string {
  const format = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  const seconds = (date.getTime() - now.getTime()) / 1000;
  for (const [unit, length] of units) {
    if (Math.abs(seconds) >= length) {
      return format.format(Math.trunc(seconds / length), unit);
    }
  }
  return format.format(0, "second");
}

// The date and the time, as the locale writes them: "Nov 22, 2026, 3:04 PM".
export function formatDateTime(date: Date, locale: string): string {
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(date);
}
