import { expect } from "storybook/test";
import { z } from "zod/v4";

import preview from "#.storybook/preview";
import { Checkbox } from "@/components/form/checkbox.tsx";
import { useAppForm } from "@/components/form/use-app-form.ts";

import type { ComponentProps } from "react";

const schema = z.object({ confirmed: z.literal(true) });

const deleteLabel = "Delete";

// A form of one box, which must be checked.
function ConfirmForm(props: ComponentProps<typeof Checkbox>) {
  const form = useAppForm({
    defaultValues: { confirmed: false as boolean },
    validators: { onChange: schema, onSubmit: schema },
  });

  return (
    <form.AppForm>
      <form.Form>
        <form.AppField name="confirmed">{(field) => <field.Checkbox {...props} />}</form.AppField>
        <form.SubmitButton label={deleteLabel} />
      </form.Form>
    </form.AppForm>
  );
}

const meta = preview.meta({
  component: Checkbox,
  args: { label: "I understand that this can't be undone" },
  render: (args) => <ConfirmForm {...args} />,
});

// A click checks the box, which the form takes as true.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const checkbox = canvas.getByRole("checkbox", { name: args.label });

    await userEvent.click(checkbox);

    await expect(checkbox).toBeChecked();
    await expect(canvas.queryByRole("alert")).not.toBeInTheDocument();
  },
});

// Submitting with the box unchecked shows the schema's error under it.
export const InvalidOnSubmit = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const notChecked = schema.shape.confirmed.safeParse(false).error?.issues[0]?.message ?? "";
    const checkbox = canvas.getByRole("checkbox", { name: args.label });

    await userEvent.click(canvas.getByRole("button", { name: deleteLabel }));

    await expect(canvas.getByRole("alert")).toHaveTextContent(notChecked);
    await expect(checkbox).toBeInvalid();
    await expect(checkbox).toHaveAccessibleDescription(notChecked);
  },
});
