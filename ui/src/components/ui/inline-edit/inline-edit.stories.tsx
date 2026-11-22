import { useState } from "react";
import { expect, fn, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { InlineEditError, InlineTextField } from "@/components/ui/inline-edit";
import { m } from "@/paraglide/messages.js";

import type { ComponentProps } from "react";

// The texts that a page passes: messages, the API's data and its errors.
const text = {
  label: "Username",
  value: "ada",
  changed: "ada.lovelace",
  tooShort: "At least 3 characters.",
  taken: "This username is taken.",
  displayName: "Display name",
  title: "Ada Lovelace",
};

// The caller's validation, as a form's schema: a username of 3 characters at least.
function validate(value: string) {
  return value.length < 3 ? text.tooShort : undefined;
}

// InlineTextField holds no value of its own: the story holds it, as a page's form does.
function UsernameField({ value: initial, ...props }: ComponentProps<typeof InlineTextField>) {
  const [value, setValue] = useState(initial);
  return (
    <InlineTextField {...props} value={value} onChange={setValue} errorMessage={validate(value)} />
  );
}

const meta = preview.meta({
  component: InlineTextField,
  args: {
    label: text.label,
    value: text.value,
    onChange: fn(),
    onSave: fn(async () => {}),
    onEditingChange: fn(),
  },
  render: (args) => <UsernameField {...args} />,
  decorators: [
    (Story) => (
      <div style={{ inlineSize: "20rem" }}>
        <Story />
      </div>
    ),
  ],
});

// The value as text, with a button that names the field.
export const View = meta.story({
  play: async ({ args, canvas }) => {
    await expect(canvas.getByText(text.value)).toBeVisible();
    await expect(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    ).toBeVisible();
  },
});

// The pointer over the value fills it.
export const Hovered = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const view = canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) });

    // The pointer may rest where the view rendered, from a story before, which enters nothing.
    await userEvent.unhover(view);
    await userEvent.hover(view);

    await waitFor(() => expect(view).toHaveAttribute("data-hovered"));
  },
});

// A press shows the editor, focused, with the value selected.
export const Editing = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());

    const input = canvas.getByRole("textbox", { name: args.label });
    await expect(input).toHaveFocus();
    await expect(input).toHaveValue(text.value);
    await expect(input).toHaveProperty("selectionEnd", text.value.length);
  },
});

// The keyboard: Enter on the view edits, Enter in the editor saves and returns the focus to the view.
export const SaveWithEnter = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const name = m.inlineEditLabel({ label: args.label });

    await userEvent.tab();
    await userEvent.keyboard("{Enter}");
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(`${text.changed}{Enter}`);

    await expect(args.onSave).toHaveBeenCalledOnce();
    await expect(args.onSave).toHaveBeenCalledWith(text.changed);
    await waitFor(() => expect(canvas.getByRole("button", { name })).toHaveFocus());
    await expect(canvas.getByText(text.changed)).toBeVisible();
    // The edit mode started and ended.
    await expect(args.onEditingChange).toHaveBeenNthCalledWith(1, true);
    await expect(args.onEditingChange).toHaveBeenLastCalledWith(false);
  },
});

// Leaving the editor saves too, and the focus stays where the user moved it.
export const SaveOnBlur = meta.story({
  play: async ({ args, canvas, canvasElement, userEvent }) => {
    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(text.changed);
    await userEvent.click(canvasElement);

    await expect(args.onSave).toHaveBeenCalledOnce();
    await expect(args.onSave).toHaveBeenCalledWith(text.changed);
    await expect(await canvas.findByText(text.changed)).toBeVisible();
  },
});

// An unchanged value leaves the editor without saving.
export const Unchanged = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const name = m.inlineEditLabel({ label: args.label });

    await userEvent.click(canvas.getByRole("button", { name }));
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard("{Enter}");

    await expect(args.onSave).not.toHaveBeenCalled();
    await waitFor(() => expect(canvas.getByRole("button", { name })).toHaveFocus());
  },
});

// Escape restores the value and returns the focus to the view.
export const CancelWithEscape = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const name = m.inlineEditLabel({ label: args.label });

    await userEvent.click(canvas.getByRole("button", { name }));
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(`${text.changed}{Escape}`);

    await expect(args.onSave).not.toHaveBeenCalled();
    await expect(canvas.getByText(text.value)).toBeVisible();
    await waitFor(() => expect(canvas.getByRole("button", { name })).toHaveFocus());
  },
});

// An invalid value stays in the editor with its error, on Enter and on leaving it.
export const Invalid = meta.story({
  play: async ({ args, canvas, canvasElement, userEvent }) => {
    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard("ab{Enter}");

    const input = canvas.getByRole("textbox", { name: args.label });
    await expect(canvas.getByRole("alert")).toHaveTextContent(text.tooShort);
    await expect(input).toBeInvalid();
    await expect(input).toHaveAccessibleDescription(text.tooShort);

    await userEvent.click(canvasElement);

    await expect(canvas.getByRole("textbox", { name: args.label })).toHaveValue("ab");
    await expect(args.onSave).not.toHaveBeenCalled();
  },
});

// While the save runs, the editor shows a spinner and takes no input.
export const Pending = meta.story({
  args: { onSave: fn(() => new Promise<void>(() => {})) },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(`${text.changed}{Enter}`);

    await expect(canvas.getByRole("progressbar", { name: m.inlineEditSaving() })).toBeVisible();
    await expect(canvas.getByRole("textbox", { name: args.label })).toHaveAttribute("readonly");
  },
});

// The save's field error, as a taken username, keeps the editor and shows on it until the value
// changes.
export const FieldError = meta.story({
  args: {
    onSave: fn(async () => {
      throw new InlineEditError(text.taken);
    }),
  },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(`${text.changed}{Enter}`);

    const input = await canvas.findByRole("textbox", { name: args.label });
    await expect(await canvas.findByRole("alert")).toHaveTextContent(text.taken);
    await expect(input).toBeInvalid();
    await expect(input).toHaveValue(text.changed);
    // Still in the edit mode.
    await expect(args.onEditingChange).toHaveBeenCalledOnce();
    await expect(args.onEditingChange).toHaveBeenCalledWith(true);

    await userEvent.keyboard("x");

    await expect(canvas.queryByRole("alert")).not.toBeInTheDocument();
  },
});

// Any other failure of the save reverts the value; the caller shows the toast.
export const Reverted = meta.story({
  args: { onSave: fn(() => Promise.reject(new Error("Forbidden"))) },
  play: async ({ args, canvas, userEvent }) => {
    const name = m.inlineEditLabel({ label: args.label });

    await userEvent.click(canvas.getByRole("button", { name }));
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(`${text.changed}{Enter}`);

    await expect(args.onSave).toHaveBeenCalledOnce();
    await expect(args.onSave).toHaveBeenCalledWith(text.changed);
    await expect(await canvas.findByText(text.value)).toBeVisible();
    await waitFor(() => expect(canvas.getByRole("button", { name })).toHaveFocus());
    // The edit mode ended once the value was restored.
    await expect(args.onEditingChange).toHaveBeenLastCalledWith(false);
  },
});

// A value that the actor can't change is plain text.
export const Disabled = meta.story({
  args: { isDisabled: true },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(text.value)).toBeVisible();
    await expect(canvas.queryByRole("button")).not.toBeInTheDocument();
  },
});

// An empty value shows the placeholder.
export const Empty = meta.story({
  args: { value: "", placeholder: text.label },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByText(args.placeholder ?? "")).toBeVisible();
  },
});

// A page's title: the heading is named by the value, and the editor takes its size.
export const Title = meta.story({
  args: { label: text.displayName, value: text.title, variant: "title" },
  decorators: [
    (Story) => (
      <div style={{ inlineSize: "36rem" }}>
        <Story />
      </div>
    ),
  ],
  play: async ({ args, canvas, userEvent }) => {
    await expect(canvas.getByRole("heading", { level: 1, name: text.title })).toBeVisible();

    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());

    await expect(canvas.getByRole("textbox", { name: args.label })).toHaveValue(text.title);
  },
});

// The view's name in German.
export const German = meta.story({
  globals: { locale: "de" },
  play: async ({ args, canvas }) => {
    await expect(
      canvas.getByRole("button", {
        name: m.inlineEditLabel({ label: args.label }, { locale: "de" }),
      }),
    ).toBeVisible();
  },
});
