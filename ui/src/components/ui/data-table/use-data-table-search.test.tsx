import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { render, screen, waitFor, within } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import { z } from "zod/v4";

import { DataTable, dataTableSearchSchema, useDataTableSearch } from "@/components/ui/data-table";
import { m } from "@/paraglide/messages.js";

import type { DataTableColumnDef, DataTableSearchDefaults } from "@/components/ui/data-table";

type Key = { id: string; name: string; status: string; owner: string };

const keys: Array<Key> = [
  { id: "k1", name: "ci-publish", status: "active", owner: "alice" },
  { id: "k2", name: "backup-mirror", status: "expired", owner: "bob" },
  { id: "k3", name: "ci-legacy", status: "revoked", owner: "bob" },
  { id: "k4", name: "deploy", status: "active", owner: "alice" },
];

const columns: Array<DataTableColumnDef<Key>> = [
  { accessorKey: "name", meta: { label: () => "Name", placement: "title" } },
  {
    accessorKey: "status",
    meta: {
      label: () => "Status",
      placement: "property",
      filterVariant: "select",
      options: ["active", "expired", "revoked"].map((value) => ({ value, label: value })),
    },
  },
  // A select filter that the URL doesn't keep.
  {
    accessorKey: "owner",
    meta: {
      label: () => "Owner",
      filterVariant: "select",
      options: ["alice", "bob"].map((value) => ({ value, label: value })),
    },
  },
];

const filters = ["status"] as const;

// The page lists the active keys by name, unless its URL says otherwise.
const defaults: DataTableSearchDefaults = {
  filterState: { globalFilter: "", selectFilters: { status: ["active"] } },
  sorting: [{ id: "name", desc: false }],
};

const rootRoute = createRootRoute({ component: Outlet });

const homeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: () => <h1>Home</h1>,
});

// A list page as a route file would build it: the table's params besides one of the page's own.
const keysRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/keys",
  validateSearch: z.object({
    ...dataTableSearchSchema({ filters }).shape,
    peek: z.string().optional().catch(undefined),
  }),
  component: function KeysPage() {
    const search = useDataTableSearch(keysRoute, { filters, defaults });
    return (
      <DataTable
        label="Keys"
        columns={columns}
        rows={keys}
        getRowId={(key) => key.id}
        {...search}
      />
    );
  },
});

// What the navigations of the slow page wait for, as a slow loader would make them.
let gate = Promise.resolve();

// A list page without defaults, whose navigations complete only once the gate opens: until then,
// its search stays the one of the last navigation that completed.
const slowKeysRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/slow-keys",
  validateSearch: dataTableSearchSchema({ filters }),
  beforeLoad: () => gate,
  component: function SlowKeysPage() {
    const search = useDataTableSearch(slowKeysRoute, { filters });
    return (
      <DataTable
        label="Keys"
        columns={columns}
        rows={keys}
        getRowId={(key) => key.id}
        {...search}
      />
    );
  },
});

function renderPage(path: string) {
  const history = createMemoryHistory({ initialEntries: ["/", path] });
  const router = createRouter({
    routeTree: rootRoute.addChildren([homeRoute, keysRoute, slowKeysRoute]),
    history,
  });
  render(<RouterProvider router={router} />);
  return { router, history };
}

async function rowNames() {
  const grid = await screen.findByRole("grid", { name: "Keys" });
  return within(grid)
    .getAllByRole("row")
    .map((row) => row.textContent);
}

// Opens the Filters popover and checks its options, one after the other.
async function checkFilters(user: ReturnType<typeof userEvent.setup>, ...options: Array<string>) {
  await user.click(screen.getByRole("button", { name: m.dataTableFilters() }));
  const dialog = await screen.findByRole("dialog", { name: m.dataTableFilters() });
  for (const option of options) {
    await user.click(within(dialog).getByRole("option", { name: option }));
  }
}

afterEach(() => {
  gate = Promise.resolve();
});

describe("useDataTableSearch", () => {
  it("opens with the defaults and keeps them out of the URL", async () => {
    const { router } = renderPage("/keys");

    expect(await rowNames()).toEqual([
      expect.stringContaining("ci-publish"),
      expect.stringContaining("deploy"),
    ]);
    expect(router.state.location.searchStr).toBe("");
  });

  it("keeps the search in the URL as the user types, with the page's other params", async () => {
    const user = userEvent.setup();
    const { router } = renderPage("/keys?peek=k1");

    await user.type(await screen.findByRole("searchbox", { name: m.dataTableSearch() }), "ci");

    await waitFor(() => {
      expect(router.state.location.search).toEqual({ q: "ci", peek: "k1" });
    });
    expect(await rowNames()).toEqual([expect.stringContaining("ci-publish")]);
  });

  it("writes an empty filter when the user clears the default one", async () => {
    const user = userEvent.setup();
    const { router } = renderPage("/keys");

    const chips = await screen.findByRole("grid", { name: m.dataTableActiveFilters() });
    await user.click(within(chips).getByRole("button"));

    await waitFor(() => {
      expect(router.state.location.search).toEqual({ status: "" });
    });
    expect(await rowNames()).toHaveLength(4);
  });

  it("reads the filters and order of the URL and keeps a change of the order", async () => {
    const user = userEvent.setup();
    const { router } = renderPage("/keys?status=active,revoked&sort=-name");

    expect(await rowNames()).toEqual([
      expect.stringContaining("deploy"),
      expect.stringContaining("ci-publish"),
      expect.stringContaining("ci-legacy"),
    ]);

    await user.click(screen.getByRole("button", { name: m.dataTableDisplay() }));
    await user.click(await screen.findByRole("button", { name: m.dataTableDescending() }));

    // Ascending by name is the default, so it leaves the URL.
    await waitFor(() => {
      expect(router.state.location.search).toEqual({ status: "active,revoked" });
    });
    await user.keyboard("{Escape}");
    expect(await rowNames()).toEqual([
      expect.stringContaining("ci-legacy"),
      expect.stringContaining("ci-publish"),
      expect.stringContaining("deploy"),
    ]);
  });

  it("replaces the history entry, so Back leaves the page", async () => {
    const user = userEvent.setup();
    const { router, history } = renderPage("/keys");

    await user.type(await screen.findByRole("searchbox", { name: m.dataTableSearch() }), "ci");
    await waitFor(() => {
      expect(router.state.location.search).toEqual({ q: "ci" });
    });

    history.back();

    expect(await screen.findByRole("heading", { name: "Home" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
  });

  it("keeps a select filter that filters doesn't list in the page's state", async () => {
    const user = userEvent.setup();
    const { router } = renderPage("/keys?status=");
    expect(await rowNames()).toHaveLength(4);

    await checkFilters(user, "alice");
    await user.keyboard("{Escape}");

    expect(await rowNames()).toEqual([
      expect.stringContaining("ci-publish"),
      expect.stringContaining("deploy"),
    ]);
    expect(router.state.location.search).toEqual({ status: "" });
  });

  it("builds a change on the last one while its navigation is pending", async () => {
    const user = userEvent.setup();
    const { router } = renderPage("/slow-keys");
    expect(await rowNames()).toHaveLength(4);

    let open = () => {};
    gate = new Promise((resolve) => {
      open = resolve;
    });
    await checkFilters(user, "expired", "revoked");
    open();

    await waitFor(() => {
      expect(router.state.location.search).toEqual({ status: "expired,revoked" });
    });
    await user.keyboard("{Escape}");
    expect(await rowNames()).toEqual([
      expect.stringContaining("backup-mirror"),
      expect.stringContaining("ci-legacy"),
    ]);
  });
});
