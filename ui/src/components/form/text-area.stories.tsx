import { expect } from "storybook/test";
import { z } from "zod/v4";

import preview from "#.storybook/preview";
import { TextArea } from "@/components/form/text-area.tsx";
import { useAppForm } from "@/components/form/use-app-form.ts";

import type { ComponentProps } from "react";

const schema = z.object({ source: z.string().min(1) });

// A form of one field, since TextArea renders a field of useAppForm.
function SourceForm(props: ComponentProps<typeof TextArea>) {
  const form = useAppForm({
    defaultValues: { source: "" },
    validators: { onChange: schema },
  });

  return <form.AppField name="source">{(field) => <field.TextArea {...props} />}</form.AppField>;
}

const meta = preview.meta({
  component: TextArea,
  args: { label: "Source" },
  render: (args) => <SourceForm {...args} />,
});

// The user's lines become the field's value.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const textArea = canvas.getByRole("textbox", { name: args.label });

    await userEvent.type(textArea, "permit (\n  principal,\n);");

    await expect(textArea).toHaveValue("permit (\n  principal,\n);");
    await expect(canvas.queryByRole("alert")).not.toBeInTheDocument();
  },
});

// The schema's error shows once the user leaves the field, and describes it.
export const InvalidValue = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const empty = schema.shape.source.safeParse("").error?.issues[0]?.message ?? "";
    const textArea = canvas.getByRole("textbox", { name: args.label });

    await userEvent.type(textArea, "x");
    await userEvent.clear(textArea);
    await userEvent.tab();

    await expect(canvas.getByRole("alert")).toHaveTextContent(empty);
    await expect(textArea).toBeInvalid();
    await expect(textArea).toHaveAccessibleDescription(empty);
  },
});
