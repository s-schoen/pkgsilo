import type { DataTableGroupingOption } from "@/components/ui/data-table/types.ts";

// A row of the data, which gives the value that it's grouped by: the column's getGroupingValue, else
// its value.
type GroupedRow = {
  getGroupingValue: (columnId: string) => unknown;
};

// A row of TanStack's grouped row model that holds a group: its id and the rows in it. The table
// groups by one column, so its rows are the data's rows. Its own groupingValue is no use: TanStack
// turns the value into the text of its key, so that null becomes "null" and false "false".
type GroupRow<TRow extends GroupedRow> = {
  id: string;
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
// of `locale`. Groups that tie keep the grouped row model's order, which is the rows' order. A
// group's value is the one that its rows share, as the column gives it: a date stays a Date, and
// null stays null.
export function orderGroups<TRow extends GroupedRow>(
  groupRows: Array<GroupRow<TRow>>,
  option: DataTableGroupingOption,
  locale: string,
): Array<Group<TRow>> {
  const collator = new Intl.Collator(locale, { numeric: true });
  const groups = groupRows.map((row) => {
    const value = row.subRows.at(0)?.getGroupingValue(option.id);
    return {
      id: row.id,
      value,
      label: formatGroupValue(value, option),
      rows: row.subRows,
    };
  });
  const { compareValues } = option;
  return groups.toSorted((left, right) =>
    compareValues
      ? compareValues(left.value, right.value)
      : collator.compare(left.label, right.label),
  );
}
