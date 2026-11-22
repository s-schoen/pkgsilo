import IconBan from "@tabler/icons-react/icons/IconBan";
import IconCalendar from "@tabler/icons-react/icons/IconCalendar";
import IconCircleCheck from "@tabler/icons-react/icons/IconCircleCheck";
import IconCircleX from "@tabler/icons-react/icons/IconCircleX";
import IconClock from "@tabler/icons-react/icons/IconClock";
import IconKey from "@tabler/icons-react/icons/IconKey";
import IconShield from "@tabler/icons-react/icons/IconShield";
import IconUser from "@tabler/icons-react/icons/IconUser";
// Only the package root exports RouterProvider.
import { RouterProvider } from "react-aria-components";
import { expect, fireEvent, fn, screen, waitFor, within } from "storybook/test";

import preview from "#.storybook/preview";
import { AppLayout } from "@/components/layouts/app-layout.tsx";
import { Badge } from "@/components/ui/badge";
import { DataTable } from "@/components/ui/data-table";
import { m } from "@/paraglide/messages.js";

import type {
  DataTableBulkAction,
  DataTableColumnDef,
  DataTableGroupingOption,
  DataTableProps,
} from "@/components/ui/data-table";

type Status = "active" | "expired" | "revoked";

type ApiKey = {
  id: string;
  name: string;
  status: Status;
  owner: string;
  // The owner's id, where two owners share a name.
  ownerId?: string;
  scopes: number;
  created: string;
};

// The sample's texts, where a page passes messages.
const text = {
  label: "API keys",
  name: "Name",
  status: "Status",
  owner: "Owner",
  scopes: "Scopes",
  created: "Created",
  active: "Active",
  expired: "Expired",
  revoked: "Revoked",
  empty: "No API keys yet. Create one to publish packages from CI.",
  revoke: "Revoke",
};

const keys: Array<ApiKey> = [
  {
    id: "k1",
    name: "ci-publish",
    status: "active",
    owner: "ci-bot",
    scopes: 3,
    created: "2026-10-02",
  },
  {
    id: "k2",
    name: "deploy-staging",
    status: "active",
    owner: "alice",
    scopes: 1,
    created: "2026-09-14",
  },
  {
    id: "k3",
    name: "backup-mirror",
    status: "expired",
    owner: "bob",
    scopes: 2,
    created: "2026-03-30",
  },
  {
    id: "k4",
    name: "legacy-uploader",
    status: "revoked",
    owner: "ci-bot",
    scopes: 5,
    created: "2025-12-01",
  },
  {
    id: "k5",
    name: "release-signing",
    status: "active",
    owner: "alice",
    scopes: 4,
    created: "2026-11-20",
  },
];

const statusBadges = {
  active: { variant: "success", label: text.active, icon: IconCircleCheck },
  expired: { variant: "warning", label: text.expired, icon: IconClock },
  revoked: { variant: "danger", label: text.revoked, icon: IconCircleX },
} as const;

// Keeps a row whose value is one of the checked options.
function includesValue(values: Array<string>, value: unknown) {
  return values.includes(String(value));
}

const columns: Array<DataTableColumnDef<ApiKey>> = [
  {
    id: "icon",
    meta: { placement: "leading" },
    cell: () => <IconKey aria-hidden />,
  },
  {
    accessorKey: "name",
    meta: { label: () => text.name, placement: "title" },
  },
  {
    accessorKey: "status",
    meta: {
      label: () => text.status,
      placement: "property",
      filterVariant: "select",
      options: (["active", "expired", "revoked"] as const).map((status) => ({
        value: status,
        label: () => statusBadges[status].label,
        icon: statusBadges[status].icon,
      })),
    },
    filterFn: (row, columnId, values: Array<string>) =>
      includesValue(values, row.getValue(columnId)),
    cell: ({ row }) => {
      const badge = statusBadges[row.original.status];
      return (
        <Badge variant={badge.variant} icon={badge.icon}>
          {badge.label}
        </Badge>
      );
    },
  },
  {
    accessorKey: "owner",
    meta: {
      label: () => text.owner,
      placement: "property",
      filterVariant: "select",
      options: ["alice", "bob", "ci-bot"].map((owner) => ({ value: owner, label: owner })),
    },
    filterFn: (row, columnId, values: Array<string>) =>
      includesValue(values, row.getValue(columnId)),
    cell: ({ row }) => (
      <Badge variant="property" icon={IconUser}>
        {row.original.owner}
      </Badge>
    ),
  },
  {
    accessorKey: "scopes",
    meta: { label: () => text.scopes, placement: "property", filterVariant: "number" },
    // At least as many scopes as the filter says.
    filterFn: (row, _columnId, minimum: string) => row.original.scopes >= Number(minimum),
    cell: ({ row }) => (
      <Badge variant="property" icon={IconShield}>
        {String(row.original.scopes)}
      </Badge>
    ),
  },
  {
    accessorKey: "created",
    meta: { label: () => text.created, placement: "property" },
    cell: ({ row }) => (
      <Badge variant="property" icon={IconCalendar}>
        {row.original.created}
      </Badge>
    ),
  },
];

// A route of the app, as getRowHref returns it.
type Href = ReturnType<NonNullable<DataTableProps<ApiKey>["getRowHref"]>>;

// The data table over the sample's API keys, as a page renders it.
function ApiKeyTable(props: Partial<DataTableProps<ApiKey>>) {
  return (
    <DataTable
      label={text.label}
      columns={columns}
      rows={keys}
      getRowId={(key) => key.id}
      // No route shows a key yet: the cast stands in for a route such as /keys/$keyId.
      getRowHref={(key) => `/keys/${key.id}` as Href}
      {...props}
    />
  );
}

const navigate = fn();

const meta = preview.meta({
  component: ApiKeyTable,
  args: { onFilterStateChange: fn() },
  // Rows and columns are data, which the controls can't edit.
  argTypes: {
    rows: { control: false },
    columns: { control: false },
    emptyState: { control: false },
  },
  decorators: [
    // On the page's surface, as AppLayout's page frame shows it.
    (Story) => (
      <RouterProvider navigate={navigate}>
        <div style={{ background: "var(--color-surface-1)" }}>
          <Story />
        </div>
      </RouterProvider>
    ),
  ],
  beforeEach: () => {
    navigate.mockClear();
  },
});

function rowNames(grid: HTMLElement) {
  return within(grid)
    .getAllByRole("row")
    .map((row) => row.getAttribute("aria-label"));
}

// The keys as rows: an icon, the name as a link, and the properties as chips at the end. Without
// anything to do with them, the rows aren't selectable.
export const Default = meta.story({
  play: async ({ canvas }) => {
    await expect(canvas.queryByRole("checkbox")).not.toBeInTheDocument();
    const grid = canvas.getByRole("grid", { name: text.label });
    await expect(rowNames(grid)).toEqual(keys.map((key) => key.name));
    const row = canvas.getByRole("row", { name: "ci-publish" });
    await expect(within(row).getByRole("link", { name: "ci-publish" })).toBeVisible();
    await expect(within(row).getByText(text.active)).toBeVisible();
    await expect(within(row).getByText("ci-bot")).toBeVisible();
  },
});

// The search narrows the rows to those with a cell that contains the text.
export const Search = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.type(canvas.getByRole("searchbox", { name: m.dataTableSearch() }), "alice");

    const grid = canvas.getByRole("grid", { name: text.label });
    await expect(rowNames(grid)).toEqual(["deploy-staging", "release-signing"]);
    await expect(args.onFilterStateChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ globalFilter: "alice" }),
    );
  },
});

// An option of the Filters popover narrows the rows and shows a chip, whose button clears it.
export const FilterAndClearChip = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: m.dataTableFilters() }));
    const dialog = await screen.findByRole("dialog", { name: m.dataTableFilters() });
    await userEvent.click(within(dialog).getByRole("option", { name: text.revoked }));
    await userEvent.keyboard("{Escape}");
    await waitFor(async () => {
      await expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    const grid = canvas.getByRole("grid", { name: text.label });
    await expect(rowNames(grid)).toEqual(["legacy-uploader"]);
    await expect(
      canvas.getByRole("button", { name: m.dataTableFiltersCount({ count: 1 }) }),
    ).toBeVisible();
    const chips = canvas.getByRole("grid", { name: m.dataTableActiveFilters() });
    await expect(
      within(chips).getByRole("row", { name: `${text.status}: ${text.revoked}` }),
    ).toBeVisible();

    await userEvent.click(within(chips).getByRole("button"));

    await expect(rowNames(canvas.getByRole("grid", { name: text.label }))).toHaveLength(
      keys.length,
    );
    await expect(
      canvas.queryByRole("grid", { name: m.dataTableActiveFilters() }),
    ).not.toBeInTheDocument();
    await expect(canvas.getByRole("searchbox", { name: m.dataTableSearch() })).toHaveFocus();
  },
});

// A number filter in the Filters popover: keys with at least that many scopes.
export const NumberFilter = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: m.dataTableFilters() }));
    const dialog = await screen.findByRole("dialog", { name: m.dataTableFilters() });
    await userEvent.type(within(dialog).getByRole("textbox", { name: text.scopes }), "4");

    await expect(rowNames(canvas.getByRole("grid", { name: text.label }))).toEqual([
      "legacy-uploader",
      "release-signing",
    ]);
    await expect(canvas.getByRole("row", { name: `${text.scopes}: 4` })).toBeInTheDocument();
  },
});

// The Display popover orders the rows by a column, and its button reverses the order.
export const OrderBy = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: m.dataTableDisplay() }));
    const dialog = await screen.findByRole("dialog", { name: m.dataTableDisplay() });
    await expect(
      within(dialog).getByRole("button", { name: m.dataTableAscending() }),
    ).toBeDisabled();

    await userEvent.click(
      within(dialog).getByRole("button", { name: new RegExp(m.dataTableOrderBy()) }),
    );
    await userEvent.click(await screen.findByRole("option", { name: text.name }));

    const grid = canvas.getByRole("grid", { name: text.label });
    await expect(rowNames(grid)).toEqual(keys.map((key) => key.name).toSorted());

    await userEvent.click(within(dialog).getByRole("button", { name: m.dataTableAscending() }));

    await expect(rowNames(grid)).toEqual(
      keys
        .map((key) => key.name)
        .toSorted()
        .toReversed(),
    );
    await expect(
      within(dialog).getByRole("button", { name: m.dataTableDescending() }),
    ).toBeEnabled();
  },
});

// A property's toggle in the Display popover hides its chips from every row.
export const HideProperty = meta.story({
  play: async ({ canvas, userEvent }) => {
    const row = canvas.getByRole("row", { name: "ci-publish" });
    await expect(within(row).getByText("ci-bot")).toBeVisible();

    await userEvent.click(canvas.getByRole("button", { name: m.dataTableDisplay() }));
    const dialog = await screen.findByRole("dialog", { name: m.dataTableDisplay() });
    const owner = within(dialog).getByRole("button", { name: text.owner });
    await expect(owner).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(owner);

    await expect(owner).toHaveAttribute("aria-pressed", "false");
    await expect(
      within(canvas.getByRole("row", { name: "ci-publish" })).queryByText("ci-bot"),
    ).not.toBeInTheDocument();
  },
});

// The keyboard: the rows are one tab stop after the toolbar, the arrow keys move between the rows
// and into a row's link, and Enter follows the row's link.
export const Keyboard = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("searchbox", { name: m.dataTableSearch() }));
    await userEvent.tab();
    await expect(canvas.getByRole("button", { name: m.dataTableFilters() })).toHaveFocus();
    await userEvent.tab();
    await userEvent.tab();
    await expect(canvas.getByRole("row", { name: "ci-publish" })).toHaveFocus();

    await userEvent.keyboard("{ArrowDown}");
    const second = canvas.getByRole("row", { name: "deploy-staging" });
    await expect(second).toHaveFocus();

    await userEvent.keyboard("{ArrowRight}");
    await expect(within(second).getByRole("link", { name: "deploy-staging" })).toHaveFocus();

    await userEvent.keyboard("{ArrowLeft}{ArrowDown}{Enter}");
    await expect(canvas.getByRole("row", { name: "backup-mirror" })).toHaveFocus();
    await expect(navigate).toHaveBeenCalledWith("/keys/k3", undefined);
  },
});

// With onRowAction, a click on the row or its title, or Enter, opens the row (as a peek) instead
// of following the link.
export const RowAction = meta.story({
  args: { onRowAction: fn() },
  play: async ({ args, canvas, userEvent }) => {
    const row = canvas.getByRole("row", { name: "backup-mirror" });
    await userEvent.click(within(row).getByText(text.expired));
    await expect(args.onRowAction).toHaveBeenLastCalledWith(keys[2]);

    await userEvent.click(within(row).getByRole("link", { name: "backup-mirror" }));
    await expect(args.onRowAction).toHaveBeenCalledTimes(2);
    await expect(navigate).not.toHaveBeenCalled();

    await userEvent.keyboard("{ArrowLeft}{ArrowDown}{Enter}");
    await expect(args.onRowAction).toHaveBeenLastCalledWith(keys[3]);
  },
});

// The row that's open, as the one of a peek, takes the accent's tint.
export const ActiveRow = meta.story({
  args: { onRowAction: fn(), isRowActive: (key: ApiKey) => key.id === "k2" },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("row", { name: "deploy-staging" })).toHaveAttribute(
      "data-active",
      "true",
    );
    await expect(canvas.getByRole("row", { name: "ci-publish" })).not.toHaveAttribute(
      "data-active",
    );
  },
});

// While the rows load: placeholders in a list that says it's busy.
export const Loading = meta.story({
  args: { isLoading: true },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("grid", { name: text.label })).toHaveAttribute(
      "aria-busy",
      "true",
    );
  },
});

// No rows at all.
export const Empty = meta.story({
  args: { rows: [] },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(m.dataTableEmpty())).toBeVisible();
  },
});

// No rows, with the page's own empty state.
export const CustomEmpty = meta.story({
  args: { rows: [], emptyState: <p>{text.empty}</p> },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(text.empty)).toBeVisible();
    await expect(canvas.queryByText(m.dataTableEmpty())).not.toBeInTheDocument();
  },
});

// Filters that hide every row: Clear filters goes back to the defaults.
export const FilteredToNothing = meta.story({
  play: async ({ canvas, userEvent }) => {
    const search = canvas.getByRole("searchbox", { name: m.dataTableSearch() });
    await userEvent.type(search, "nothing-matches");
    await expect(canvas.getByText(m.dataTableNoMatches())).toBeVisible();

    await userEvent.click(canvas.getByRole("button", { name: m.dataTableClearFilters() }));

    await expect(rowNames(canvas.getByRole("grid", { name: text.label }))).toHaveLength(
      keys.length,
    );
    await expect(search).toHaveValue("");
  },
});

// Rows that the page's default filters hide are the page's empty state, not a filtered one.
export const DefaultFiltersMatchNothing = meta.story({
  args: {
    rows: keys.filter((key) => key.status !== "revoked"),
    defaultFilterState: { globalFilter: "", selectFilters: { status: ["revoked"] } },
  },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(m.dataTableEmpty())).toBeVisible();
    await expect(canvas.queryByText(m.dataTableNoMatches())).not.toBeInTheDocument();
  },
});

// A narrow screen: the toolbar wraps, and a row's properties wrap below its title.
export const Narrow = meta.story({
  render: (args) => (
    <div style={{ inlineSize: "22rem" }}>
      <ApiKeyTable {...args} />
    </div>
  ),
  play: async ({ canvas }) => {
    const row = canvas.getByRole("row", { name: "ci-publish" });
    const title = within(row).getByRole("link", { name: "ci-publish" }).getBoundingClientRect();
    const owner = within(row).getByText("ci-bot").getBoundingClientRect();
    await expect(owner.top).toBeGreaterThanOrEqual(title.bottom);
  },
});

// Enough keys to scroll the window.
const manyKeys = Array.from({ length: 8 }, (_, round) =>
  keys.map((key) => ({ ...key, id: `${key.id}-${round}`, name: `${key.name}-${round}` })),
).flat();

// On a page of the app: the table reaches the edges of the page's frame, below the page's title,
// and its toolbar sticks to the top of the window while the rows scroll below it.
export const InPage = meta.story({
  args: { rows: manyKeys },
  parameters: { layout: "fullscreen" },
  render: (args) => (
    <AppLayout navigation={() => null}>
      <h1>{text.label}</h1>
      <ApiKeyTable {...args} />
    </AppLayout>
  ),
  play: async ({ canvas }) => {
    const main = canvas.getByRole("main");
    const grid = canvas.getByRole("grid", { name: text.label });
    const frame = main.getBoundingClientRect();
    const rows = grid.getBoundingClientRect();
    // The frame's border is all that's left on either side.
    await expect(rows.left - frame.left).toBeLessThanOrEqual(1);
    await expect(frame.right - rows.right).toBeLessThanOrEqual(1);

    const search = canvas.getByRole("searchbox", { name: m.dataTableSearch() });
    window.scrollTo({ top: document.documentElement.scrollHeight });
    // The page's title has scrolled away; the toolbar's search is still in view.
    await waitFor(async () => {
      await expect(Math.round(search.getBoundingClientRect().top)).toBeGreaterThanOrEqual(0);
    });
    await expect(
      canvas.getByRole("heading", { name: text.label }).getBoundingClientRect().bottom,
    ).toBeLessThan(0);
    window.scrollTo({ top: 0 });
  },
});

// Revokes the active keys among the selected ones.
const revoke = fn<(rows: Array<ApiKey>) => void>();
const bulkActions: Array<DataTableBulkAction<ApiKey>> = [
  {
    label: () => text.revoke,
    icon: IconBan,
    appliesTo: (key) => key.status === "active",
    onAction: revoke,
  },
];

// The args of a table whose rows are selectable: they can be revoked and deleted.
const selectable = { bulkActions, onDelete: fn() };

function selectedCount(count: number) {
  return m.dataTableSelected({ count });
}

function selectionBox(row: HTMLElement) {
  return within(row).getByRole("checkbox");
}

// How visible a row's selection box is: the product of the opacities up to the row, since a
// parent's opacity isn't inherited by the computed style.
function boxOpacity(row: HTMLElement) {
  let opacity = 1;
  for (let node: HTMLElement | null = selectionBox(row); node && node !== row;) {
    opacity *= Number(getComputedStyle(node).opacity);
    node = node.parentElement;
  }
  return opacity;
}

// A row's box shows on hover and selects it; once a row is selected, every box shows, a click on a
// row toggles it, and Shift+click selects the rows up to it. The clear button ends the selection.
export const SelectWithPointer = meta.story({
  args: selectable,
  beforeEach: () => {
    revoke.mockClear();
  },
  play: async ({ canvas, userEvent }) => {
    const first = canvas.getByRole("row", { name: "ci-publish" });
    const last = canvas.getByRole("row", { name: "release-signing" });
    await expect(boxOpacity(last)).toBe(0);
    await userEvent.hover(last);
    await waitFor(async () => {
      await expect(boxOpacity(last)).toBe(1);
    });
    await userEvent.unhover(last);

    await userEvent.click(selectionBox(first));
    await expect(selectionBox(first)).toBeChecked();
    await expect(first).toHaveAttribute("aria-selected", "true");
    await expect(canvas.getByText(selectedCount(1))).toBeVisible();
    await expect(navigate).not.toHaveBeenCalled();

    // Away from the first row, the other rows' boxes still show.
    const third = canvas.getByRole("row", { name: "backup-mirror" });
    await userEvent.unhover(first);
    await waitFor(async () => {
      await expect(boxOpacity(third)).toBe(1);
    });

    // Each call of userEvent starts with no key held: one session holds Shift through the click.
    const session = userEvent.setup({});
    await session.keyboard("{Shift>}");
    await session.click(within(third).getByText(text.expired));
    await session.keyboard("{/Shift}");
    await expect(canvas.getByText(selectedCount(3))).toBeVisible();
    await expect(canvas.getByRole("row", { name: "deploy-staging" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    // A plain click on a selected row unselects it rather than opening it.
    await userEvent.click(within(third).getByText(text.expired));
    await expect(canvas.getByText(selectedCount(2))).toBeVisible();
    await expect(navigate).not.toHaveBeenCalled();

    await userEvent.click(canvas.getByRole("button", { name: m.dataTableClearSelection() }));
    await expect(canvas.queryByText(selectedCount(2))).not.toBeInTheDocument();
    await expect(first).toHaveAttribute("aria-selected", "false");
    // The focus moves from the gone button to the rows.
    await expect(canvas.getByRole("grid", { name: text.label })).toContainElement(
      document.activeElement as HTMLElement,
    );
  },
});

// Space selects the focused row, and Ctrl+A every row; Escape clears the selection.
export const SelectWithKeyboard = meta.story({
  args: selectable,
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("searchbox", { name: m.dataTableSearch() }));
    await userEvent.tab();
    await userEvent.tab();
    await userEvent.tab();
    const first = canvas.getByRole("row", { name: "ci-publish" });
    await expect(first).toHaveFocus();
    await waitFor(async () => {
      await expect(boxOpacity(first)).toBe(1);
    });

    await userEvent.keyboard(" ");
    await userEvent.keyboard("{ArrowDown} ");
    await expect(canvas.getByText(selectedCount(2))).toBeVisible();
    await expect(navigate).not.toHaveBeenCalled();

    await userEvent.keyboard("{Control>}a{/Control}");
    await expect(canvas.getByText(selectedCount(keys.length))).toBeVisible();

    await userEvent.keyboard("{Escape}");
    await expect(canvas.queryByText(selectedCount(keys.length))).not.toBeInTheDocument();
  },
});

// Select all takes the rows that the filters keep, and the rows hidden later don't come back with
// the filter's end.
export const SelectAllFiltered = meta.story({
  args: selectable,
  play: async ({ canvas, userEvent }) => {
    const search = canvas.getByRole("searchbox", { name: m.dataTableSearch() });
    await userEvent.type(search, "alice");
    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "deploy-staging" })));
    await userEvent.keyboard("{Control>}a{/Control}");
    await expect(canvas.getByText(selectedCount(2))).toBeVisible();

    await userEvent.clear(search);
    await expect(rowNames(canvas.getByRole("grid", { name: text.label }))).toHaveLength(
      keys.length,
    );
    await expect(canvas.getByText(selectedCount(2))).toBeVisible();
    await expect(canvas.getByRole("row", { name: "ci-publish" })).toHaveAttribute(
      "aria-selected",
      "false",
    );
  },
});

// A filter that hides a selected row takes it out of the selection for good.
export const SelectionClearedByFilter = meta.story({
  args: selectable,
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "ci-publish" })));
    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "deploy-staging" })));
    await expect(canvas.getByText(selectedCount(2))).toBeVisible();

    const search = canvas.getByRole("searchbox", { name: m.dataTableSearch() });
    await userEvent.type(search, "alice");
    await expect(canvas.getByText(selectedCount(1))).toBeVisible();

    await userEvent.clear(search);
    await expect(canvas.getByText(selectedCount(1))).toBeVisible();
    await expect(canvas.getByRole("row", { name: "ci-publish" })).toHaveAttribute(
      "aria-selected",
      "false",
    );
  },
});

// A bulk action shows while one of the selected rows qualifies, and acts on those rows alone.
export const BulkAction = meta.story({
  args: selectable,
  beforeEach: () => {
    revoke.mockClear();
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "backup-mirror" })));
    await expect(canvas.getByText(selectedCount(1))).toBeVisible();
    await expect(canvas.queryByRole("button", { name: text.revoke })).not.toBeInTheDocument();

    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "ci-publish" })));
    await userEvent.click(canvas.getByRole("button", { name: text.revoke }));
    await expect(revoke).toHaveBeenCalledOnce();
    await expect(revoke).toHaveBeenCalledWith([keys[0]]);

    // Delete comes after the bulk actions.
    const buttons = canvas.getAllByRole("button").map((button) => button.textContent);
    await expect(buttons.indexOf(m.dataTableDelete())).toBeGreaterThan(
      buttons.indexOf(text.revoke),
    );
  },
});

// Delete asks first, naming how many rows it deletes; confirming deletes them and ends the
// selection.
export const DeleteConfirmed = meta.story({
  args: selectable,
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "ci-publish" })));
    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "backup-mirror" })));
    await userEvent.click(canvas.getByRole("button", { name: m.dataTableDelete() }));

    const dialog = await screen.findByRole("alertdialog", {
      name: m.dataTableDeleteTitle({ count: 2 }),
    });
    await userEvent.click(within(dialog).getByRole("button", { name: m.dataTableDelete() }));

    await expect(args.onDelete).toHaveBeenCalledOnce();
    await expect(args.onDelete).toHaveBeenCalledWith([keys[0], keys[2]]);
    await waitFor(async () => {
      await expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    await expect(canvas.queryByText(selectedCount(2))).not.toBeInTheDocument();
  },
});

// Cancel deletes nothing and keeps the selection.
export const DeleteCancelled = meta.story({
  args: selectable,
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "ci-publish" })));
    await userEvent.click(canvas.getByRole("button", { name: m.dataTableDelete() }));

    const dialog = await screen.findByRole("alertdialog", {
      name: m.dataTableDeleteTitle({ count: 1 }),
    });
    await userEvent.click(within(dialog).getByRole("button", { name: m.cancel() }));

    await waitFor(async () => {
      await expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    await expect(args.onDelete).not.toHaveBeenCalled();
    await expect(canvas.getByText(selectedCount(1))).toBeVisible();
  },
});

// In German: the selection's texts are messages, the count's in its plural.
export const SelectionGerman = meta.story({
  args: selectable,
  globals: { locale: "de" },
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "ci-publish" })));
    await expect(
      canvas.getByText(m.dataTableSelected({ count: 1 }, { locale: "de" })),
    ).toBeVisible();
    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "deploy-staging" })));
    await expect(
      canvas.getByText(m.dataTableSelected({ count: 2 }, { locale: "de" })),
    ).toBeVisible();
    await expect(
      canvas.getByRole("button", { name: m.dataTableDelete({}, { locale: "de" }) }),
    ).toBeVisible();
  },
});

// In German: the toolbar's texts are messages.
export const German = meta.story({
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(
      canvas.getByRole("searchbox", { name: m.dataTableSearch({}, { locale: "de" }) }),
    ).toBeVisible();
    await expect(
      canvas.getByRole("button", { name: m.dataTableFilters({}, { locale: "de" }) }),
    ).toBeVisible();
    await expect(
      canvas.getByRole("button", { name: m.dataTableDisplay({}, { locale: "de" }) }),
    ).toBeVisible();
  },
});

// The keys by status, a status's group with its icon, and by owner, by the owners' names.
const groupingOptions: Array<DataTableGroupingOption> = [
  {
    id: "status",
    label: () => text.status,
    formatValue: (value) => statusBadges[value as Status].label,
    renderIcon: (value) => {
      const Icon = statusBadges[value as Status].icon;
      return <Icon aria-hidden />;
    },
  },
  { id: "owner", label: () => text.owner },
];

function treeGrid(canvas: { getByRole: typeof screen.getByRole }) {
  return canvas.getByRole("treegrid", { name: text.label });
}

// The groups' rows, the first level of the tree.
function groupRows(canvas: { getByRole: typeof screen.getByRole }) {
  return within(treeGrid(canvas))
    .getAllByRole("row")
    .filter((row) => row.getAttribute("aria-level") === "1");
}

function groupNames(canvas: { getByRole: typeof screen.getByRole }) {
  return groupRows(canvas).map((row) => row.getAttribute("aria-label"));
}

function groupRow(canvas: { getByRole: typeof screen.getByRole }, name: string) {
  return within(treeGrid(canvas)).getByRole("row", { name, expanded: true });
}

// Grouped by owner: a row for each owner, in the locale's order of their names, with the count of
// its keys, and its keys below it.
export const Grouped = meta.story({
  args: { groupingOptions, grouping: "owner" },
  play: async ({ canvas }) => {
    await expect(rowNames(treeGrid(canvas))).toEqual([
      "alice",
      "deploy-staging",
      "release-signing",
      "bob",
      "backup-mirror",
      "ci-bot",
      "ci-publish",
      "legacy-uploader",
    ]);
    const alice = groupRow(canvas, "alice");
    await expect(alice).toHaveAttribute("aria-expanded", "true");
    await expect(within(alice).getByText("2")).toBeVisible();
    await expect(canvas.getByRole("row", { name: "ci-publish" })).toHaveAttribute(
      "aria-level",
      "2",
    );
  },
});

// Group by in the Display popover groups the rows by a column, None ungroups them again.
export const GroupAndUngroup = meta.story({
  args: { groupingOptions },
  play: async ({ canvas, userEvent }) => {
    await expect(canvas.getByRole("grid", { name: text.label })).toBeVisible();

    await userEvent.click(canvas.getByRole("button", { name: m.dataTableDisplay() }));
    const dialog = await screen.findByRole("dialog", { name: m.dataTableDisplay() });
    const groupBy = within(dialog).getByRole("button", { name: new RegExp(m.dataTableGroupBy()) });
    await expect(groupBy).toHaveTextContent(m.dataTableGroupNone());
    await userEvent.click(groupBy);
    await userEvent.click(await screen.findByRole("option", { name: text.status }));

    // The statuses by their labels.
    await expect(groupNames(canvas)).toEqual([text.active, text.expired, text.revoked]);
    await expect(within(groupRow(canvas, text.active)).getByText("3")).toBeVisible();

    await userEvent.click(groupBy);
    await userEvent.click(await screen.findByRole("option", { name: m.dataTableGroupNone() }));

    await expect(rowNames(canvas.getByRole("grid", { name: text.label }))).toEqual(
      keys.map((key) => key.name),
    );
  },
});

// With onGroupingChange, the page controls the grouping: Group by reports the choice.
export const ControlledGrouping = meta.story({
  args: { groupingOptions, grouping: "owner", onGroupingChange: fn() },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: m.dataTableDisplay() }));
    const dialog = await screen.findByRole("dialog", { name: m.dataTableDisplay() });
    await userEvent.click(
      within(dialog).getByRole("button", { name: new RegExp(m.dataTableGroupBy()) }),
    );
    await userEvent.click(await screen.findByRole("option", { name: m.dataTableGroupNone() }));

    await expect(args.onGroupingChange).toHaveBeenCalledWith(undefined);
    await expect(groupRow(canvas, "alice")).toBeVisible();
  },
});

// A group's chevron collapses its rows, and expands them again.
export const CollapseWithPointer = meta.story({
  args: { groupingOptions, grouping: "status" },
  play: async ({ canvas, userEvent }) => {
    const active = groupRow(canvas, text.active);
    await userEvent.click(within(active).getByRole("button"));

    await expect(active).toHaveAttribute("aria-expanded", "false");
    await expect(canvas.queryByRole("row", { name: "ci-publish" })).not.toBeInTheDocument();
    await expect(canvas.getByRole("row", { name: "backup-mirror" })).toBeVisible();

    await userEvent.click(within(active).getByRole("button"));

    await expect(active).toHaveAttribute("aria-expanded", "true");
    // The group's rows fade in.
    await waitFor(async () => {
      await expect(canvas.getByRole("row", { name: "ci-publish" })).toBeVisible();
    });
  },
});

// The keyboard: Left collapses a group and Right expands it, Enter toggles it, Down skips the rows
// of a collapsed group, and Left on a row moves to its group.
export const CollapseWithKeyboard = meta.story({
  args: { groupingOptions, grouping: "status" },
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("searchbox", { name: m.dataTableSearch() }));
    await userEvent.tab();
    await userEvent.tab();
    await userEvent.tab();
    const active = groupRow(canvas, text.active);
    await expect(active).toHaveFocus();

    await userEvent.keyboard("{ArrowLeft}");
    await expect(active).toHaveAttribute("aria-expanded", "false");
    await userEvent.keyboard("{ArrowDown}");
    await expect(groupRow(canvas, text.expired)).toHaveFocus();

    await userEvent.keyboard("{ArrowUp}{ArrowRight}");
    await expect(active).toHaveAttribute("aria-expanded", "true");
    await userEvent.keyboard("{ArrowDown}");
    await expect(canvas.getByRole("row", { name: "ci-publish" })).toHaveFocus();

    await userEvent.keyboard("{ArrowLeft}");
    await expect(active).toHaveFocus();

    await userEvent.keyboard("{Enter}");
    await expect(active).toHaveAttribute("aria-expanded", "false");
    await userEvent.keyboard("{Enter}");
    await expect(active).toHaveAttribute("aria-expanded", "true");
    await expect(navigate).not.toHaveBeenCalled();
  },
});

// With onRowAction, Enter on a group still toggles it, and on a row opens the row.
export const GroupsWithRowAction = meta.story({
  args: { groupingOptions, grouping: "status", onRowAction: fn() },
  play: async ({ args, canvas, userEvent }) => {
    const expired = groupRow(canvas, text.expired);
    await userEvent.click(expired);
    await expect(expired).toHaveAttribute("aria-expanded", "false");
    await userEvent.keyboard("{Enter}");
    await expect(expired).toHaveAttribute("aria-expanded", "true");
    await expect(args.onRowAction).not.toHaveBeenCalled();

    await userEvent.keyboard("{ArrowDown}{Enter}");
    await expect(args.onRowAction).toHaveBeenCalledWith(keys[2]);
  },
});

// Groups in the page's own order (compareValues): the statuses that need attention first.
export const CustomGroupOrder = meta.story({
  args: {
    grouping: "status",
    groupingOptions: groupingOptions.map((option) =>
      option.id === "status"
        ? {
            ...option,
            compareValues: (left: unknown, right: unknown) => {
              const rank: Record<string, number> = { revoked: 0, expired: 1, active: 2 };
              return (rank[String(left)] ?? 0) - (rank[String(right)] ?? 0);
            },
          }
        : option,
    ),
  },
  play: async ({ canvas }) => {
    await expect(groupNames(canvas)).toEqual([text.revoked, text.expired, text.active]);
  },
});

// Two owners of the same name: the owner column groups by the owner's id (getGroupingValue), so
// each keeps a group of its own.
const ownerNames: Record<string, string> = { u1: "alice", u2: "bob", u3: "ci-bot", u4: "alice" };
const ownerIds: Record<string, string> = { alice: "u1", bob: "u2", "ci-bot": "u3" };

export const SameLabelsApart = meta.story({
  args: {
    rows: [
      ...keys.map((key) => ({ ...key, ownerId: ownerIds[key.owner] })),
      {
        id: "k6",
        name: "docs-preview",
        status: "active",
        owner: "alice",
        ownerId: "u4",
        scopes: 1,
        created: "2026-11-21",
      },
    ],
    columns: columns.map((column) =>
      column.id === "owner" || ("accessorKey" in column && column.accessorKey === "owner")
        ? { ...column, getGroupingValue: (key: ApiKey) => key.ownerId }
        : column,
    ),
    groupingOptions: [
      { id: "owner", label: () => text.owner, formatValue: (id) => ownerNames[String(id)] ?? "" },
    ],
    grouping: "owner",
  },
  play: async ({ canvas }) => {
    const alices = groupRows(canvas).filter((row) => row.getAttribute("aria-label") === "alice");
    await expect(alices).toHaveLength(2);
    await expect(within(alices[0]).getByText("2")).toBeVisible();
    await expect(within(alices[1]).getByText("1")).toBeVisible();
  },
});

// Selection spans the groups: Shift+click selects across them, and Ctrl/Cmd+A selects the rows
// of a collapsed group too; the groups take no part in it.
export const SelectAcrossGroups = meta.story({
  args: { ...selectable, groupingOptions, grouping: "owner" },
  play: async ({ canvas, userEvent }) => {
    await expect(within(groupRow(canvas, "alice")).queryByRole("checkbox")).not.toBeInTheDocument();

    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "deploy-staging" })));
    await userEvent.keyboard("{Shift>}");
    await userEvent.click(canvas.getByRole("row", { name: "backup-mirror" }));
    await userEvent.keyboard("{/Shift}");
    await expect(canvas.getByText(selectedCount(3))).toBeVisible();

    // Enter toggles a group, which takes no part in the selection.
    const ciBot = groupRow(canvas, "ci-bot");
    await userEvent.click(within(ciBot).getByRole("button"));
    await expect(ciBot).toHaveAttribute("aria-expanded", "false");
    await expect(ciBot).toHaveFocus();
    await userEvent.keyboard("{Enter}");
    await expect(ciBot).toHaveAttribute("aria-expanded", "true");
    await userEvent.keyboard("{Enter}");
    await expect(ciBot).toHaveAttribute("aria-expanded", "false");
    await expect(canvas.getByText(selectedCount(3))).toBeVisible();
    await userEvent.click(canvas.getByRole("row", { name: "deploy-staging" }));
    await userEvent.keyboard("{Control>}a{/Control}");
    await expect(canvas.getByText(selectedCount(keys.length))).toBeVisible();

    await userEvent.click(canvas.getByRole("button", { name: m.dataTableDelete() }));
    const dialog = await screen.findByRole("alertdialog", {
      name: m.dataTableDeleteTitle({ count: keys.length }),
    });
    await userEvent.click(within(dialog).getByRole("button", { name: m.cancel() }));
    await waitFor(async () => {
      await expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
  },
});

// In German: Group by and its None are messages, and the groups are ordered in the locale's
// collation.
export const GroupedGerman = meta.story({
  args: { groupingOptions, grouping: "owner" },
  globals: { locale: "de" },
  play: async ({ canvas, userEvent }) => {
    await expect(groupRow(canvas, "alice")).toBeVisible();
    await userEvent.click(
      canvas.getByRole("button", { name: m.dataTableDisplay({}, { locale: "de" }) }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: m.dataTableDisplay({}, { locale: "de" }),
    });
    await userEvent.click(
      within(dialog).getByRole("button", {
        name: new RegExp(m.dataTableGroupBy({}, { locale: "de" })),
      }),
    );
    // The options' popover fades in.
    const none = await screen.findByRole("option", {
      name: m.dataTableGroupNone({}, { locale: "de" }),
    });
    await waitFor(async () => {
      await expect(none).toBeVisible();
    });
    await userEvent.keyboard("{Escape}");
  },
});

// Right-clicks a row and waits for its menu's first item to take the focus.
async function rightClick(
  userEvent: { pointer: (input: { keys: string; target: Element }) => Promise<void> },
  row: HTMLElement,
) {
  await userEvent.pointer({ keys: "[MouseRight]", target: row });
  const menu = await screen.findByRole("menu");
  await waitFor(async () => {
    await expect(within(menu).getAllByRole("menuitem")[0]).toHaveFocus();
  });
  return menu;
}

function menuItemNames(menu: HTMLElement) {
  return within(menu)
    .getAllByRole("menuitem")
    .map((item) => item.textContent);
}

async function waitForMenuToClose() {
  await waitFor(async () => {
    await expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });
}

// A right-click on a row that isn't selected selects it alone, and its menu acts on it: Open, Open
// in new tab, the bulk actions that apply to it, and Delete last. The focus returns to the row.
export const ContextMenuOnRow = meta.story({
  args: selectable,
  beforeEach: () => {
    revoke.mockClear();
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "ci-publish" })));
    const row = canvas.getByRole("row", { name: "deploy-staging" });

    const menu = await rightClick(userEvent, row);

    await expect(menu).toHaveAccessibleName(m.dataTableRowActions({ name: "deploy-staging" }));
    await expect(menuItemNames(menu)).toEqual([
      m.dataTableOpen(),
      m.dataTableOpenInNewTab(),
      text.revoke,
      m.dataTableDelete(),
    ]);
    await expect(canvas.getByText(selectedCount(1))).toBeVisible();
    await expect(row).toHaveAttribute("aria-selected", "true");
    await expect(canvas.getByRole("row", { name: "ci-publish" })).toHaveAttribute(
      "aria-selected",
      "false",
    );
    // A new tab is the browser's: the item is a link to the row's route.
    const newTab = within(menu).getByRole("menuitem", { name: m.dataTableOpenInNewTab() });
    await expect(newTab).toHaveAttribute("href", "/keys/k2");
    await expect(newTab).toHaveAttribute("target", "_blank");

    await userEvent.click(within(menu).getByRole("menuitem", { name: text.revoke }));

    await expect(revoke).toHaveBeenCalledWith([keys[1]]);
    await waitForMenuToClose();
    await waitFor(async () => {
      await expect(row).toHaveFocus();
    });
  },
});

// A right-click on a selected row acts on the whole selection: the menu's header names the count,
// it has no Open, a bulk action gets the rows it applies to, and Delete asks about them all.
export const ContextMenuOnSelection = meta.story({
  args: selectable,
  beforeEach: () => {
    revoke.mockClear();
  },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "ci-publish" })));
    await userEvent.click(selectionBox(canvas.getByRole("row", { name: "backup-mirror" })));

    const menu = await rightClick(userEvent, canvas.getByRole("row", { name: "backup-mirror" }));

    await expect(menu).toHaveAccessibleName(selectedCount(2));
    // Once the menu has entered with its animation.
    await waitFor(async () => {
      await expect(within(menu).getByText(selectedCount(2))).toBeVisible();
    });
    await expect(menuItemNames(menu)).toEqual([text.revoke, m.dataTableDelete()]);
    await expect(canvas.getByText(selectedCount(2))).toBeVisible();

    await userEvent.click(within(menu).getByRole("menuitem", { name: m.dataTableDelete() }));
    const dialog = await screen.findByRole("alertdialog", {
      name: m.dataTableDeleteTitle({ count: 2 }),
    });
    await userEvent.click(within(dialog).getByRole("button", { name: m.dataTableDelete() }));

    await expect(args.onDelete).toHaveBeenCalledWith([keys[0], keys[2]]);
    await expect(revoke).not.toHaveBeenCalled();
    await waitFor(async () => {
      await expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
  },
});

// Shift+F10 opens the focused row's menu below it; Open follows the row's link. The context-menu
// key opens it as well, and Escape closes it, giving the row its focus back.
export const ContextMenuWithKeyboard = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("searchbox", { name: m.dataTableSearch() }));
    await userEvent.tab();
    await userEvent.tab();
    await userEvent.tab();
    await userEvent.keyboard("{ArrowDown}");
    const row = canvas.getByRole("row", { name: "deploy-staging" });
    await expect(row).toHaveFocus();

    await userEvent.keyboard("{ContextMenu}");
    const menu = await screen.findByRole("menu", {
      name: m.dataTableRowActions({ name: "deploy-staging" }),
    });
    await waitFor(async () => {
      await expect(within(menu).getByRole("menuitem", { name: m.dataTableOpen() })).toHaveFocus();
    });
    await expect(menu.getBoundingClientRect().top).toBeGreaterThanOrEqual(
      row.getBoundingClientRect().bottom,
    );
    await userEvent.keyboard("{Escape}");
    await waitForMenuToClose();
    await waitFor(async () => {
      await expect(row).toHaveFocus();
    });

    await userEvent.keyboard("{Shift>}{F10}{/Shift}");
    await waitFor(async () => {
      await expect(screen.getByRole("menuitem", { name: m.dataTableOpen() })).toHaveFocus();
    });
    await userEvent.keyboard("{Enter}");

    await expect(navigate).toHaveBeenCalledWith("/keys/k2", undefined);
    await waitForMenuToClose();
  },
});

// With onRowAction, Open opens the row as a click does.
export const ContextMenuOpensRow = meta.story({
  args: { onRowAction: fn() },
  play: async ({ args, canvas, userEvent }) => {
    const menu = await rightClick(userEvent, canvas.getByRole("row", { name: "backup-mirror" }));
    await expect(menuItemNames(menu)).toEqual([m.dataTableOpen(), m.dataTableOpenInNewTab()]);

    await userEvent.click(within(menu).getByRole("menuitem", { name: m.dataTableOpen() }));

    await expect(args.onRowAction).toHaveBeenCalledWith(keys[2]);
    await expect(navigate).not.toHaveBeenCalled();
    await waitForMenuToClose();
  },
});

// With nothing to open and no actions, a right-click leaves the browser its own menu.
export const NoContextMenu = meta.story({
  args: { getRowHref: undefined },
  play: async ({ canvas }) => {
    const notCancelled = fireEvent.contextMenu(canvas.getByRole("row", { name: "ci-publish" }));

    await expect(notCancelled).toBe(true);
    await expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  },
});

// In groups, a row has its menu as well; a group has none.
export const ContextMenuInGroups = meta.story({
  args: { ...selectable, groupingOptions, grouping: "owner" },
  play: async ({ canvas, userEvent }) => {
    await expect(fireEvent.contextMenu(groupRow(canvas, "alice"))).toBe(true);
    await expect(screen.queryByRole("menu")).not.toBeInTheDocument();

    const row = canvas.getByRole("row", { name: "release-signing" });
    const menu = await rightClick(userEvent, row);

    await expect(menu).toHaveAccessibleName(m.dataTableRowActions({ name: "release-signing" }));
    await expect(row).toHaveAttribute("aria-selected", "true");
    await userEvent.keyboard("{Escape}");
    await waitForMenuToClose();
    await waitFor(async () => {
      await expect(row).toHaveFocus();
    });
  },
});

// In German: the menu's items and name are messages.
export const ContextMenuGerman = meta.story({
  args: selectable,
  globals: { locale: "de" },
  play: async ({ canvas, userEvent }) => {
    const menu = await rightClick(userEvent, canvas.getByRole("row", { name: "ci-publish" }));

    await expect(menu).toHaveAccessibleName(
      m.dataTableRowActions({ name: "ci-publish" }, { locale: "de" }),
    );
    await expect(menuItemNames(menu)).toEqual([
      m.dataTableOpen({}, { locale: "de" }),
      m.dataTableOpenInNewTab({}, { locale: "de" }),
      text.revoke,
      m.dataTableDelete({}, { locale: "de" }),
    ]);
    await userEvent.keyboard("{Escape}");
    await waitForMenuToClose();
  },
});
