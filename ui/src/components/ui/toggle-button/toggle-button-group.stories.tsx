import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { ToggleButton, ToggleButtonGroup } from "@/components/ui/toggle-button";

import type { ComponentProps } from "react";

const properties = [
  { id: "owner", name: "Owner" },
  { id: "version", name: "Version" },
  { id: "license", name: "License" },
  { id: "updated", name: "Updated" },
  { id: "downloads", name: "Downloads" },
];

// The properties that a list shows: the group holds the ids of the pressed ones.
function PropertyGroup(props: Omit<ComponentProps<typeof ToggleButtonGroup>, "children">) {
  return (
    // As narrow as the Display popover, so that the buttons wrap.
    <div style={{ maxInlineSize: "16rem" }}>
      <ToggleButtonGroup {...props}>
        {properties.map((property) => (
          <ToggleButton key={property.id} id={property.id}>
            {property.name}
          </ToggleButton>
        ))}
      </ToggleButtonGroup>
    </div>
  );
}

const meta = preview.meta({
  component: PropertyGroup,
  args: {
    "aria-label": "Display properties",
    defaultSelectedKeys: ["owner", "version"],
    onSelectionChange: fn(),
  },
});

// Any of the buttons is pressed: a click toggles one and leaves the others.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const license = canvas.getByRole("button", { name: "License" });
    const owner = canvas.getByRole("button", { name: "Owner" });

    await expect(canvas.getByRole("toolbar", { name: args["aria-label"] })).toBeVisible();

    await userEvent.click(license);
    await userEvent.click(owner);

    await expect(license).toHaveAttribute("aria-pressed", "true");
    await expect(owner).toHaveAttribute("aria-pressed", "false");
    await expect(canvas.getByRole("button", { name: "Version" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await expect(args.onSelectionChange).toHaveBeenLastCalledWith(new Set(["version", "license"]));
  },
});

// The group is one stop of the tab order: the arrow keys move within it, and Space toggles.
export const WithKeyboard = meta.story({
  play: async ({ canvas, userEvent }) => {
    const owner = canvas.getByRole("button", { name: "Owner" });
    const version = canvas.getByRole("button", { name: "Version" });

    await userEvent.tab();
    await expect(owner).toHaveFocus();

    await userEvent.keyboard("{ArrowRight}");
    await expect(version).toHaveFocus();

    await userEvent.keyboard(" ");
    await expect(version).toHaveAttribute("aria-pressed", "false");

    await userEvent.tab();
    await expect(version).not.toHaveFocus();
  },
});

// A disabled group presses none of its buttons.
export const Disabled = meta.story({
  args: { isDisabled: true },
  play: async ({ args, canvas, userEvent }) => {
    const license = canvas.getByRole("button", { name: "License" });

    await userEvent.click(license);

    await expect(license).toBeDisabled();
    await expect(args.onSelectionChange).not.toHaveBeenCalled();
  },
});
