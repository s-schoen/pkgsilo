import { expect, screen } from "storybook/test";
import { z } from "zod/v4";

import preview from "#.storybook/preview";
import { ComboBox } from "@/components/form/combo-box.tsx";
import { useAppForm } from "@/components/form/use-app-form.ts";
import { ListBoxItem } from "@/components/ui/list-box";

const identities = [
  { id: "id-alice", name: "alice" },
  { id: "id-bob", name: "bob" },
];

const schema = z.object({ member: z.string() });

const text = { label: "Member", submit: "Add" };

// A form of one combobox, which must have a value.
function MemberForm() {
  const form = useAppForm({
    defaultValues: { member: null as string | null },
    validators: { onChange: schema },
  });

  return (
    <form.AppForm>
      <form.Form>
        <form.AppField name="member">
          {(field) => (
            <field.ComboBox label={text.label} items={identities}>
              {(identity) => <ListBoxItem id={identity.id}>{identity.name}</ListBoxItem>}
            </field.ComboBox>
          )}
        </form.AppField>
        <form.SubmitButton label={text.submit} />
      </form.Form>
    </form.AppForm>
  );
}

const meta = preview.meta({
  component: ComboBox,
  args: { label: text.label, children: undefined },
  render: () => <MemberForm />,
});

// The picked option's id becomes the field's value, which the schema takes.
export const Default = meta.story({
  play: async ({ canvas, userEvent }) => {
    const input = canvas.getByRole("combobox", { name: text.label });

    await userEvent.type(input, "bo");
    await userEvent.click(await screen.findByRole("option", { name: "bob" }));
    await userEvent.tab();

    await expect(input).toHaveValue("bob");
    await expect(canvas.queryByRole("alert")).not.toBeInTheDocument();
  },
});

// Submitting without a value shows the schema's error.
export const InvalidOnSubmit = meta.story({
  play: async ({ canvas, userEvent }) => {
    const missing = schema.shape.member.safeParse(null).error?.issues[0]?.message ?? "";

    await userEvent.click(canvas.getByRole("button", { name: text.submit }));

    await expect(canvas.getByRole("alert")).toHaveTextContent(missing);
    await expect(canvas.getByRole("combobox", { name: text.label })).toBeInvalid();
  },
});
