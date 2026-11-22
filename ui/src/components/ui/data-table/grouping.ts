import type { DataTableGroupingOption } from "@/components/ui/data-table/types.ts";

// A row of TanStack's grouped row model that holds a group: its id, the value it groups by and the
// rows in it. The table groups by one column, so its rows are the data's rows.
type GroupRow<TRow> = {
  id: string;
  groupingValue?: unknown;
  subRows: Array<TRow>;
};

// A group as the table shows it.
export type Group<TRow> = {
  id: string;
  value: unknown;
  label: string;
  rows: Array<TRow>;
};

function formatGroupValue(value: unknown, option: DataTableGroupingOption): string {
  if (option.formatValue) {
    return option.formatValue(value);
  }
  return typeof value === "string" || typeof value === "number" || typeof value === "boolean"
    ? String(value)
    : "";
}

// The groups in their order: by the option's compareValues, else by their labels in the collation
// of `locale`. Groups that tie keep the grouped row model's order, which is the rows' order.
export function orderGroups<TRow>(
  groupRows: Array<GroupRow<TRow>>,
  option: DataTableGroupingOption,
  locale: string,
): Array<Group<TRow>> {
  const collator = new Intl.Collator(locale, { numeric: true });
  const groups = groupRows.map((row) => ({
    id: row.id,
    value: row.groupingValue,
    label: formatGroupValue(row.groupingValue, option),
    rows: row.subRows,
  }));
  const { compareValues } = option;
  return groups.toSorted((left, right) =>
    compareValues
      ? compareValues(left.value, right.value)
      : collator.compare(left.label, right.label),
  );
}
