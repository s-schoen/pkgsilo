import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { TextArea } from "@/components/ui/text-field";

const meta = preview.meta({
  component: TextArea,
  args: { label: "Source", onChange: fn() },
});

// The user types several lines, such as a policy's source.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const textArea = canvas.getByRole("textbox", { name: args.label });

    await userEvent.type(textArea, "permit (\n  principal,\n);");

    await expect(textArea).toHaveValue("permit (\n  principal,\n);");
    await expect(args.onChange).toHaveBeenLastCalledWith("permit (\n  principal,\n);");
  },
});

// The height follows rows.
export const Rows = meta.story({
  args: { rows: 8 },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("textbox", { name: args.label })).toHaveAttribute("rows", "8");
  },
});

export const WithDescription = meta.story({
  args: { description: "The policy in Cedar." },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("textbox", { name: args.label })).toHaveAccessibleDescription(
      args.description ?? "",
    );
  },
});

// An invalid text area shows its error, which describes it.
export const Invalid = meta.story({
  args: { isInvalid: true, errorMessage: "The source doesn't parse." },
  play: async ({ args, canvas }) => {
    const textArea = canvas.getByRole("textbox", { name: args.label });

    await expect(canvas.getByRole("alert")).toHaveTextContent(args.errorMessage ?? "");
    await expect(textArea).toBeInvalid();
  },
});

export const Disabled = meta.story({
  args: { isDisabled: true, defaultValue: "permit (principal, action, resource);" },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("textbox", { name: args.label })).toBeDisabled();
  },
});
