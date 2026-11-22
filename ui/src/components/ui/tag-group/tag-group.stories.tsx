import IconCircleCheck from "@tabler/icons-react/icons/IconCircleCheck";
import IconCircleX from "@tabler/icons-react/icons/IconCircleX";
import { useState } from "react";
import { expect, fn, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { Tag, TagGroup } from "@/components/ui/tag-group";

import type { ComponentProps } from "react";

const text = { label: "Active filters", status: "Status", owner: "Owner" };

const initialTags = [
  { id: "status:active", property: text.status, value: "Active", icon: IconCircleCheck },
  { id: "status:revoked", property: text.status, value: "Revoked", icon: IconCircleX },
  { id: "owner:ci-bot", property: text.owner, value: "ci-bot" },
];

// The toolbar's filter chips: removing one drops it from the list, as the data table does.
function FilterTags({ onRemove, ...props }: Omit<ComponentProps<typeof TagGroup>, "children">) {
  const [tags, setTags] = useState(initialTags);
  return (
    <TagGroup
      {...props}
      onRemove={
        onRemove &&
        ((keys) => {
          setTags((current) => current.filter((tag) => !keys.has(tag.id)));
          onRemove(keys);
        })
      }
    >
      {tags.map((tag) => (
        <Tag key={tag.id} id={tag.id} property={tag.property} icon={tag.icon}>
          {tag.value}
        </Tag>
      ))}
    </TagGroup>
  );
}

const meta = preview.meta({
  component: FilterTags,
  args: { "aria-label": text.label, onRemove: fn() },
});

// Removable tags: the button at the end of a tag removes it, which React Aria names after the tag.
export const Removable = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const group = canvas.getByRole("grid", { name: args["aria-label"] });
    await expect(canvas.getByRole("row", { name: "Status: Active" })).toBeVisible();

    await userEvent.click(canvas.getByRole("button", { name: /Status: Revoked/ }));

    await expect(args.onRemove).toHaveBeenCalledWith(new Set(["status:revoked"]));
    await waitFor(async () => {
      await expect(canvas.queryByRole("row", { name: "Status: Revoked" })).not.toBeInTheDocument();
    });
    await expect(group).toBeVisible();
  },
});

// The keyboard: one tab stop, the arrow keys move between the tags, Delete removes the focused one.
export const Keyboard = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.tab();
    await expect(canvas.getByRole("row", { name: "Status: Active" })).toHaveFocus();

    await userEvent.keyboard("{ArrowRight}");
    await expect(canvas.getByRole("row", { name: "Status: Revoked" })).toHaveFocus();

    await userEvent.keyboard("{Delete}");

    await expect(args.onRemove).toHaveBeenCalledWith(new Set(["status:revoked"]));
    await waitFor(async () => {
      await expect(canvas.queryByRole("row", { name: "Status: Revoked" })).not.toBeInTheDocument();
    });
  },
});

// Without onRemove, the tags only show what's set.
export const Static = meta.story({
  args: { onRemove: undefined },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("row", { name: "Owner: ci-bot" })).toBeVisible();
    await expect(canvas.queryByRole("button")).not.toBeInTheDocument();
  },
});

// The button's name is React Aria's, in the locale.
export const German = meta.story({
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("button", { name: /^Entfernen Status: Active/ })).toBeVisible();
  },
});
