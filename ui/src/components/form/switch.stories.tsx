import { expect } from "storybook/test";
import { z } from "zod/v4";

import preview from "#.storybook/preview";
import { Switch } from "@/components/form/switch.tsx";
import { useAppForm } from "@/components/form/use-app-form.ts";

import type { ComponentProps } from "react";

const schema = z.object({ signUps: z.boolean() });

// A form of one switch.
function SettingsForm(props: ComponentProps<typeof Switch>) {
  const form = useAppForm({
    defaultValues: { signUps: true },
    validators: { onChange: schema },
  });

  return <form.AppField name="signUps">{(field) => <field.Switch {...props} />}</form.AppField>;
}

const meta = preview.meta({
  component: Switch,
  args: { label: "Sign-ups", description: "Anyone may create an identity." },
  render: (args) => <SettingsForm {...args} />,
});

// The switch starts with the field's value, and the keyboard turns it off.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const toggle = canvas.getByRole("switch", { name: args.label });

    await expect(toggle).toBeChecked();

    await userEvent.tab();
    await userEvent.keyboard(" ");

    await expect(toggle).not.toBeChecked();
  },
});
