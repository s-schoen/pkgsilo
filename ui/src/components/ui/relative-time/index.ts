// The public API of the relative time: other code imports it as @/components/ui/relative-time.
// The formats serve a place where the tooltip can't go, such as inside a control: a data table's
// property chip shows the relative time, and its preview card the date and the time.
export { RelativeTime } from "@/components/ui/relative-time/relative-time.tsx";
export { formatDateTime, formatRelativeTime } from "@/components/ui/relative-time/format.ts";
