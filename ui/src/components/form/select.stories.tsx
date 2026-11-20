import { expect, screen } from "storybook/test";
import { z } from "zod/v4";

import preview from "#.storybook/preview";
import { Select } from "@/components/form/select.tsx";
import { useAppForm } from "@/components/form/use-app-form.ts";
import { ListBoxItem } from "@/components/ui/list-box";

const roles = [
  { id: "reader", name: "Reader" },
  { id: "maintainer", name: "Maintainer" },
];

const schema = z.object({ role: z.string() });

const text = { label: "Role", submit: "Save" };

// A form of one select, which must have a value.
function RoleForm() {
  const form = useAppForm({
    defaultValues: { role: null as string | null },
    validators: { onChange: schema },
  });

  return (
    <form.AppForm>
      <form.Form>
        <form.AppField name="role">
          {(field) => (
            <field.Select label={text.label} items={roles}>
              {(role) => <ListBoxItem id={role.id}>{role.name}</ListBoxItem>}
            </field.Select>
          )}
        </form.AppField>
        <form.SubmitButton label={text.submit} />
      </form.Form>
    </form.AppForm>
  );
}

const meta = preview.meta({
  component: Select,
  args: { label: text.label, children: undefined },
  render: () => <RoleForm />,
});

// The picked option's id becomes the field's value, which the schema takes.
export const Default = meta.story({
  play: async ({ canvas, userEvent }) => {
    const trigger = canvas.getByRole("button", { name: new RegExp(text.label) });

    await userEvent.click(trigger);
    await userEvent.click(await screen.findByRole("option", { name: "Maintainer" }));

    await expect(trigger).toHaveTextContent("Maintainer");
    await expect(canvas.queryByRole("alert")).not.toBeInTheDocument();
  },
});

// Submitting without a value shows the schema's error.
export const InvalidOnSubmit = meta.story({
  play: async ({ canvas, userEvent }) => {
    const missing = schema.shape.role.safeParse(null).error?.issues[0]?.message ?? "";

    await userEvent.click(canvas.getByRole("button", { name: text.submit }));

    await expect(canvas.getByRole("alert")).toHaveTextContent(missing);
  },
});
