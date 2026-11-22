import IconCircleCheck from "@tabler/icons-react/icons/IconCircleCheck";
import IconCircleX from "@tabler/icons-react/icons/IconCircleX";
import IconClock from "@tabler/icons-react/icons/IconClock";
import IconFilter from "@tabler/icons-react/icons/IconFilter";
import { Dialog, DialogTrigger } from "react-aria-components/Dialog";
import { expect, fn, screen, waitFor, within } from "storybook/test";

import preview from "#.storybook/preview";
import { Button } from "@/components/ui/button";
import { FilterList, FilterListItem, FilterListSection } from "@/components/ui/filter-list";
import { Popover } from "@/components/ui/popover";
import { m } from "@/paraglide/messages.js";

import type { ComponentProps } from "react";

const statuses = [
  { id: "status:active", name: "Active", icon: IconCircleCheck },
  { id: "status:expired", name: "Expired", icon: IconClock },
  { id: "status:revoked", name: "Revoked", icon: IconCircleX },
];

// A group's icon is an element, a dot in its color, rather than a Tabler icon.
const groups = [
  { id: "group:admins", name: "admins", color: "var(--color-text-danger)" },
  { id: "group:developers", name: "developers", color: "var(--color-accent)" },
  { id: "group:ci", name: "ci", color: "var(--color-text-success)" },
];

const text = { status: "Status", group: "Group", open: "Filters" };

// The Filters popover's list: one section per property, with its values as options.
function KeyFilterList(props: Omit<ComponentProps<typeof FilterList>, "children">) {
  return (
    <FilterList {...props}>
      <FilterListSection title={text.status}>
        {statuses.map((status) => (
          <FilterListItem key={status.id} id={status.id} icon={status.icon}>
            {status.name}
          </FilterListItem>
        ))}
      </FilterListSection>
      <FilterListSection title={text.group}>
        {groups.map((group) => (
          <FilterListItem
            key={group.id}
            id={group.id}
            icon={
              <span
                style={{
                  inlineSize: "0.5em",
                  blockSize: "0.5em",
                  borderRadius: "var(--radius-full)",
                  background: group.color,
                }}
              />
            }
          >
            {group.name}
          </FilterListItem>
        ))}
      </FilterListSection>
    </FilterList>
  );
}

const meta = preview.meta({
  component: KeyFilterList,
  args: {
    "aria-label": "Filters",
    searchLabel: "Search filters",
    placeholder: "Filter by…",
    onSelectionChange: fn(),
  },
  render: (args) => (
    // As wide as the Filters popover.
    <div style={{ inlineSize: "16rem" }}>
      <KeyFilterList {...args} />
    </div>
  ),
});

// A click checks an option, and another unchecks it; the others stay as they are.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const expired = canvas.getByRole("option", { name: "Expired" });
    const ci = canvas.getByRole("option", { name: "ci" });

    await userEvent.click(expired);

    await expect(expired).toHaveAttribute("aria-selected", "true");
    await expect(args.onSelectionChange).toHaveBeenCalledOnce();

    await userEvent.click(ci);
    await userEvent.click(expired);

    await expect(expired).toHaveAttribute("aria-selected", "false");
    await expect(ci).toHaveAttribute("aria-selected", "true");
  },
});

// Options checked already show their box filled.
export const Checked = meta.story({
  args: { defaultSelectedKeys: ["status:active", "group:admins"] },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("option", { name: "Active" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(canvas.getByRole("option", { name: "admins" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  },
});

// Typing narrows the options to those that match, ignoring case, and hides a section without one.
export const Searched = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.type(canvas.getByRole("searchbox", { name: args.searchLabel }), "DEV");

    const list = canvas.getByRole("listbox", { name: args["aria-label"] });
    await waitFor(async () => {
      await expect(within(list).getAllByRole("option")).toHaveLength(1);
    });
    await expect(within(list).getByRole("option", { name: "developers" })).toBeVisible();
    await expect(within(list).queryByText(text.status)).not.toBeInTheDocument();
  },
});

// The focus stays in the search: Enter checks the highlighted option, the arrow keys move the
// highlight, and Escape clears the search.
export const WithKeyboard = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const search = canvas.getByRole("searchbox", { name: args.searchLabel });

    await userEvent.tab();
    await expect(search).toHaveFocus();

    // Typing highlights the first match.
    await userEvent.keyboard("rev");
    const revoked = canvas.getByRole("option", { name: "Revoked" });
    await waitFor(async () => {
      await expect(search).toHaveAttribute("aria-activedescendant", revoked.id);
    });
    await userEvent.keyboard("{Enter}");

    await expect(revoked).toHaveAttribute("aria-selected", "true");
    await expect(search).toHaveFocus();

    await userEvent.keyboard("{Escape}");
    await expect(search).toHaveValue("");
    await waitFor(async () => {
      await expect(canvas.getAllByRole("option")).toHaveLength(statuses.length + groups.length);
    });

    // Clearing the search highlights the first option again, and Escape keeps the checked ones.
    await waitFor(async () => {
      await expect(search).toHaveAttribute(
        "aria-activedescendant",
        canvas.getByRole("option", { name: "Active" }).id,
      );
    });
    await expect(revoked).toHaveAttribute("aria-selected", "true");
    await userEvent.keyboard("{ArrowDown}");
    await waitFor(async () => {
      await expect(search).toHaveAttribute(
        "aria-activedescendant",
        canvas.getByRole("option", { name: "Expired" }).id,
      );
    });
    await userEvent.keyboard("{Enter}");

    await expect(canvas.getByRole("option", { name: "Expired" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  },
});

// A search that matches nothing says so.
export const NoMatches = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.type(canvas.getByRole("searchbox", { name: args.searchLabel }), "zz");

    await expect(await canvas.findByText(m.noMatchingOptions())).toBeVisible();
    await expect(canvas.queryByRole("option", { name: "Active" })).not.toBeInTheDocument();
  },
});

// In a popover, as the Filters popover holds it: the search takes the focus as it opens, and
// Escape on an empty search closes it.
export const InPopover = meta.story({
  args: { autoFocus: true },
  render: (args) => (
    <DialogTrigger>
      <Button variant="secondary" icon={IconFilter}>
        {text.open}
      </Button>
      <Popover placement="bottom start">
        <Dialog aria-label={text.open}>
          <div style={{ inlineSize: "16rem" }}>
            <KeyFilterList {...args} />
          </div>
        </Dialog>
      </Popover>
    </DialogTrigger>
  ),
  play: async ({ args, canvas, userEvent }) => {
    const trigger = canvas.getByRole("button", { name: text.open });

    await userEvent.click(trigger);

    const search = await screen.findByRole("searchbox", { name: args.searchLabel });
    await waitFor(async () => {
      await expect(search).toHaveFocus();
    });

    await userEvent.keyboard("act{Enter}");
    await expect(screen.getByRole("option", { name: "Active" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    // The first Escape clears the search, the second closes the popover.
    await userEvent.keyboard("{Escape}");
    await expect(search).toHaveValue("");
    await userEvent.keyboard("{Escape}");

    await waitFor(async () => {
      await expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
    await waitFor(async () => {
      await expect(trigger).toHaveFocus();
    });
  },
});

// The empty state follows the locale.
export const German = meta.story({
  globals: { locale: "de" },
  args: { "aria-label": "Filter", searchLabel: "Filter durchsuchen", placeholder: "Filtern nach…" },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.type(canvas.getByRole("searchbox", { name: args.searchLabel }), "zz");

    await expect(await canvas.findByText(m.noMatchingOptions({}, { locale: "de" }))).toBeVisible();
  },
});
