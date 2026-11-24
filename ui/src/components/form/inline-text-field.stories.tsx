import { expect, fn, waitFor } from "storybook/test";
import { z } from "zod/v4";

import preview from "#.storybook/preview";
import { InlineTextField } from "@/components/form/inline-text-field.tsx";
import { useAppForm } from "@/components/form/use-app-form.ts";
import { InlineEditError } from "@/components/ui/inline-edit";
import { m } from "@/paraglide/messages.js";

import type { ComponentProps } from "react";

const schema = z.object({ username: z.string().min(3) });

const text = {
  label: "Username",
  value: "ada",
  changed: "ada.lovelace",
  taken: "This username is taken.",
};

// A form of one field, which holds the value while the user edits it, as a details page's.
function UsernameForm(props: ComponentProps<typeof InlineTextField>) {
  const form = useAppForm({
    defaultValues: { username: text.value },
    validators: { onChange: schema },
  });

  return (
    <form.AppField name="username">{(field) => <field.InlineTextField {...props} />}</form.AppField>
  );
}

const meta = preview.meta({
  component: InlineTextField,
  args: { label: text.label, onSave: fn(async () => {}) },
  render: (args) => <UsernameForm {...args} />,
  decorators: [
    (Story) => (
      <div style={{ inlineSize: "20rem" }}>
        <Story />
      </div>
    ),
  ],
});

// The field's value, saved with Enter.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const name = m.inlineEditLabel({ label: args.label });

    await userEvent.click(canvas.getByRole("button", { name }));
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(`${text.changed}{Enter}`);

    await expect(args.onSave).toHaveBeenCalledOnce();
    await expect(args.onSave).toHaveBeenCalledWith(text.changed);
    await waitFor(() => expect(canvas.getByRole("button", { name })).toHaveFocus());
    await expect(canvas.getByText(text.changed)).toBeVisible();
  },
});

// The schema's error keeps the editor.
export const InvalidValue = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    // zod's message for the value, in the locale of the story, so that the story doesn't repeat it.
    const tooShort = schema.shape.username.safeParse("ab").error?.issues[0]?.message ?? "";

    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard("ab{Enter}");

    await expect(canvas.getByRole("alert")).toHaveTextContent(tooShort);
    await expect(canvas.getByRole("textbox", { name: args.label })).toBeInvalid();
    await expect(args.onSave).not.toHaveBeenCalled();
  },
});

// The server's field error shows on the field.
export const ServerFieldError = meta.story({
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

    await expect(await canvas.findByRole("alert")).toHaveTextContent(text.taken);
    await expect(canvas.getByRole("textbox", { name: args.label })).toHaveValue(text.changed);
  },
});

// Any other failure restores the field's value.
export const Reverted = meta.story({
  args: { onSave: fn(() => Promise.reject(new Error("Forbidden"))) },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(`${text.changed}{Enter}`);

    await expect(await canvas.findByText(text.value)).toBeVisible();
  },
});

// A value that the actor may not change is the field's value as plain text, without the edit mode.
export const ReadOnly = meta.story({
  args: { isReadOnly: true },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByText(text.value)).toBeVisible();
    await expect(
      canvas.queryByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    ).not.toBeInTheDocument();
  },
});
