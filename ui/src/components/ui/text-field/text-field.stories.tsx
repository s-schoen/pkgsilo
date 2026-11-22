import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { TextField } from "@/components/ui/text-field";

const meta = preview.meta({
  component: TextField,
  args: { label: "Name", onChange: fn() },
});

// The user types the value, which the field reports as it changes.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const input = canvas.getByRole("textbox", { name: args.label });

    await userEvent.type(input, "ci-bot");

    await expect(input).toHaveValue("ci-bot");
    await expect(args.onChange).toHaveBeenLastCalledWith("ci-bot");
  },
});

// The keyboard reaches the input, whose focus shows the ring.
export const FocusedWithKeyboard = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const input = canvas.getByRole("textbox", { name: args.label });

    await userEvent.tab();

    await expect(input).toHaveFocus();
    await expect(input).toHaveAttribute("data-focused");
  },
});

// A hint below the input describes it.
export const WithDescription = meta.story({
  args: { description: "Lowercase letters, digits and dashes." },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("textbox", { name: args.label })).toHaveAccessibleDescription(
      args.description ?? "",
    );
  },
});

// A sample of the value, while the field is empty.
export const WithPlaceholder = meta.story({
  args: { placeholder: "ci-bot" },
});

// An invalid field shows its error, which describes the input and is announced.
export const Invalid = meta.story({
  args: { isInvalid: true, errorMessage: "Name is taken.", defaultValue: "admin" },
  play: async ({ args, canvas }) => {
    const input = canvas.getByRole("textbox", { name: args.label });

    await expect(canvas.getByRole("alert")).toHaveTextContent(args.errorMessage ?? "");
    await expect(input).toBeInvalid();
    await expect(input).toHaveAccessibleDescription(args.errorMessage ?? "");
  },
});

// The error shows only while the field is invalid.
export const ValidWithErrorMessage = meta.story({
  args: { errorMessage: "Name is taken." },
  play: async ({ canvas }) => {
    await expect(canvas.queryByRole("alert")).not.toBeInTheDocument();
  },
});

// A disabled field can't be focused or changed.
export const Disabled = meta.story({
  args: { isDisabled: true, defaultValue: "ci-bot" },
  play: async ({ args, canvas, userEvent }) => {
    const input = canvas.getByRole("textbox", { name: args.label });

    await userEvent.type(input, "x");

    await expect(input).toBeDisabled();
    await expect(input).toHaveValue("ci-bot");
  },
});

// A password hides what the user types.
export const Password = meta.story({
  args: { label: "Password", type: "password" },
  play: async ({ args, canvasElement }) => {
    await expect(canvasElement.querySelector("input")).toHaveAttribute("type", args.type);
  },
});

// A page's large title, as a new object's name in a details layout: its placeholder shows the
// field's name, and the label names the input without showing.
export const Title = meta.story({
  args: {
    variant: "title",
    isLabelHidden: true,
    label: "Display name",
    placeholder: "Display name",
  },
  play: async ({ args, canvas, userEvent }) => {
    const input = canvas.getByRole("textbox", { name: args.label });

    await expect(canvas.queryByText(args.label, { selector: "label" })).not.toBeInTheDocument();
    await userEvent.type(input, "Ada Lovelace");

    await expect(input).toHaveValue("Ada Lovelace");
  },
});

// Without its label, where the page names the field already, as a property's row: the label still
// names the input.
export const LabelHidden = meta.story({
  args: { isLabelHidden: true },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("textbox", { name: args.label })).toBeInTheDocument();
    await expect(canvas.queryByText(args.label)).not.toBeInTheDocument();
  },
});

// The title's error shows below it, as a field's.
export const TitleInvalid = meta.story({
  args: {
    variant: "title",
    isLabelHidden: true,
    label: "Display name",
    isInvalid: true,
    errorMessage: "Enter a display name.",
  },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("textbox", { name: args.label })).toHaveAccessibleDescription(
      args.errorMessage ?? "",
    );
  },
});
