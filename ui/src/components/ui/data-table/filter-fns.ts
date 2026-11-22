import { constructFilterFn, filterFn_includesString } from "@tanstack/react-table";

import type { DataTableColumnDef, FilterVariant } from "@/components/ui/data-table/types.ts";
import type { RowData } from "@tanstack/react-table";

// A value as an option's value: text, or a number or boolean as its text. Any other value, such
// as null, matches no option.
function optionValue(value: unknown): string | undefined {
  return typeof value === "string" || typeof value === "number" || typeof value === "boolean"
    ? String(value)
    : undefined;
}

function matchesOption(value: unknown, values: Array<string>): boolean {
  const text = optionValue(value);
  return text !== undefined && values.includes(text);
}

// Keeps a row whose value is one of the checked options, comparing as text, since an option's value
// is text: a status of "active", a count of 3 as "3". A list value, such as a row's tags, matches
// when one of its items does.
export const filterFn_select = constructFilterFn({
  filter: (value: unknown, values: Array<string>) =>
    Array.isArray(value)
      ? value.some((item) => matchesOption(item, values))
      : matchesOption(value, values),
  resolveFilterValue: (values: unknown) =>
    Array.isArray(values) ? values.flatMap((value) => optionValue(value) ?? []) : [],
  autoRemove: (values: unknown) => !Array.isArray(values) || values.length === 0,
});

// Keeps a row whose value is the number the user typed. Text that isn't a number keeps no row,
// so that the chip it shows never claims a filter that doesn't apply.
export const filterFn_number = constructFilterFn({
  filter: (value: unknown, typed: number) => typeof value === "number" && value === typed,
  resolveFilterValue: (typed: unknown) => (typeof typed === "string" ? Number(typed) : Number.NaN),
  resolveDataValue: (value: unknown) =>
    typeof value === "string" && value.trim() ? Number(value) : value,
  autoRemove: (typed: unknown) => typeof typed !== "string" || !typed.trim(),
});

// The filter function of each variant, which a column with that variant and without a filterFn of
// its own filters by: TanStack's "auto" would pick one by the column's first value, which doesn't
// fit a list of options or what the user typed. A text filter keeps a row whose value contains the
// text, ignoring case, as the search does.
const variantFilterFns = {
  select: filterFn_select,
  text: filterFn_includesString,
  number: filterFn_number,
} satisfies Record<FilterVariant, unknown>;

// The columns with the filter function of their variant where they name none themselves, the
// columns of a group too.
export function withVariantFilterFns<TData extends RowData>(
  columns: Array<DataTableColumnDef<TData>>,
): Array<DataTableColumnDef<TData>> {
  return columns.map((column) => {
    let resolved = column;
    if ("columns" in column && Array.isArray(column.columns)) {
      resolved = { ...resolved, columns: withVariantFilterFns(column.columns) };
    }
    const variant = column.meta?.filterVariant;
    if (variant && column.filterFn === undefined) {
      resolved = { ...resolved, filterFn: variantFilterFns[variant] };
    }
    return resolved;
  });
}
