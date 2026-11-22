import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { Checkbox } from "@/components/ui/checkbox";

// The box's text, which a play function clicks; the type of args.children allows the box alone.
const text = { label: "Administrator" };

const meta = preview.meta({
  component: Checkbox,
  args: { children: text.label, onChange: fn() },
});

// A click on the text checks the box.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const checkbox = canvas.getByRole("checkbox", { name: args.children });

    await userEvent.click(canvas.getByText(text.label));

    await expect(checkbox).toBeChecked();
    await expect(args.onChange).toHaveBeenCalledWith(true);
  },
});

// Space toggles the focused box, whose focus shows the ring.
export const ToggledWithKeyboard = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const checkbox = canvas.getByRole("checkbox", { name: args.children });

    await userEvent.tab();
    await userEvent.keyboard(" ");

    await expect(checkbox).toHaveFocus();
    await expect(checkbox).toBeChecked();
    await expect(checkbox.closest("label")).toHaveAttribute("data-focus-visible");
  },
});

export const Checked = meta.story({
  args: { defaultSelected: true },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("checkbox", { name: args.children })).toBeChecked();
  },
});

// Neither checked nor unchecked, such as "select all" over a partial selection.
export const Indeterminate = meta.story({
  args: { isIndeterminate: true },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("checkbox", { name: args.children })).toBePartiallyChecked();
  },
});

// A hint below the text describes the box.
export const WithDescription = meta.story({
  args: { description: "Administrators may change every policy." },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("checkbox", { name: args.children })).toHaveAccessibleDescription(
      args.description ?? "",
    );
  },
});

export const Invalid = meta.story({
  args: { children: "I understand", isInvalid: true, errorMessage: "Confirm to go on." },
  play: async ({ args, canvas }) => {
    const checkbox = canvas.getByRole("checkbox", { name: args.children });

    await expect(canvas.getByRole("alert")).toHaveTextContent(args.errorMessage ?? "");
    await expect(checkbox).toBeInvalid();
  },
});

// A disabled box can't be toggled, checked or not.
export const Disabled = meta.story({
  args: { isDisabled: true },
  render: (args) => (
    <div style={{ display: "flex", flexDirection: "column", gap: "var(--space-2)" }}>
      <Checkbox {...args} />
      <Checkbox {...args} defaultSelected />
    </div>
  ),
  play: async ({ args, canvas, userEvent }) => {
    const [unchecked, checked] = canvas.getAllByRole("checkbox", { name: args.children });

    await userEvent.click(canvas.getAllByText(text.label)[0] ?? unchecked);

    await expect(unchecked).toBeDisabled();
    await expect(unchecked).not.toBeChecked();
    await expect(checked).toBeChecked();
    await expect(args.onChange).not.toHaveBeenCalled();
  },
});
