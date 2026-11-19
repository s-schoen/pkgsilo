import { expect } from "storybook/test";
import { z } from "zod/v4";

import preview from "#.storybook/preview";
import { TextField } from "@/components/form/text-field.tsx";
import { useAppForm } from "@/components/form/use-app-form.ts";

import type { ComponentProps } from "react";

const schema = z.object({ name: z.string().min(3) });

// A form of one field, since TextField renders a field of useAppForm.
function NameForm(props: ComponentProps<typeof TextField>) {
  const form = useAppForm({
    defaultValues: { name: "" },
    validators: { onChange: schema },
  });

  return <form.AppField name="name">{(field) => <field.TextField {...props} />}</form.AppField>;
}

const meta = preview.meta({
  component: TextField,
  args: { label: "Name" },
  render: (args) => <NameForm {...args} />,
});

export const Default = meta.story();

export const Password = meta.story({
  args: { type: "password" },
});

// The schema's error shows once the user leaves the field, and describes the input.
export const InvalidValue = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    // zod's message for the value, in the locale of the story, so that the story doesn't repeat it.
    const tooShort = schema.shape.name.safeParse("ab").error?.issues[0]?.message ?? "";
    const input = canvas.getByRole("textbox", { name: args.label });

    await userEvent.type(input, "ab");
    await userEvent.tab();

    await expect(canvas.getByRole("alert")).toHaveTextContent(tooShort);
    await expect(input).toBeInvalid();
    await expect(input).toHaveAccessibleDescription(tooShort);
  },
});
