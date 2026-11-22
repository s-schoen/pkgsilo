import { delay } from "msw";
import { expect, fn, waitFor } from "storybook/test";
import { z } from "zod/v4";

import preview from "#.storybook/preview";
import { Form } from "@/components/form/form.tsx";
import { useAppForm } from "@/components/form/use-app-form.ts";

const schema = z.object({
  name: z.string().min(3),
  description: z.string(),
  admin: z.boolean(),
});

type Values = z.infer<typeof schema>;

// The texts of the form, where a view passes messages.
const text = {
  name: "Name",
  description: "Description",
  admin: "Administrators",
  adminDescription: "Members may change every policy.",
  submit: "Create group",
};

// A form of several fields and a submit button, as a view builds one. onSubmit takes a moment, as a
// request to the API would.
function GroupForm({
  onSubmit,
  layout,
}: {
  onSubmit: (value: Values) => void;
  layout?: "stack" | "page";
}) {
  const form = useAppForm({
    defaultValues: { name: "", description: "", admin: false },
    validators: { onChange: schema },
    onSubmit: async ({ value }) => {
      await delay(300);
      onSubmit(value);
    },
  });

  return (
    <form.AppForm>
      <form.Form layout={layout}>
        <form.AppField name="name">
          {(field) => <field.TextField label={text.name} />}
        </form.AppField>
        <form.AppField name="description">
          {(field) => <field.TextArea label={text.description} rows={3} />}
        </form.AppField>
        <form.AppField name="admin">
          {(field) => <field.Checkbox label={text.admin} description={text.adminDescription} />}
        </form.AppField>
        <form.SubmitButton label={text.submit} />
      </form.Form>
    </form.AppForm>
  );
}

const meta = preview.meta({
  component: Form,
  args: { children: undefined },
  render: () => <GroupForm onSubmit={submitted} />,
});

const submitted = fn<(value: Values) => void>();

// The user fills the fields and submits: the button is pending until the submission ends.
export const Submitted = meta.story({
  beforeEach: () => {
    submitted.mockClear();
  },
  play: async ({ canvas, userEvent }) => {
    const button = canvas.getByRole("button", { name: text.submit });

    await userEvent.type(canvas.getByRole("textbox", { name: text.name }), "maintainers");
    await userEvent.type(canvas.getByRole("textbox", { name: text.description }), "Maintain it.");
    await userEvent.click(canvas.getByRole("checkbox", { name: text.admin }));
    await userEvent.click(button);

    await expect(button).toHaveAttribute("data-pending");
    await waitFor(async () => {
      await expect(submitted).toHaveBeenCalledWith({
        name: "maintainers",
        description: "Maintain it.",
        admin: true,
      });
    });
    await expect(button).not.toHaveAttribute("data-pending");
  },
});

// Submitting the form untouched shows the schema's errors instead, and submits nothing.
export const InvalidOnSubmit = meta.story({
  beforeEach: () => {
    submitted.mockClear();
  },
  play: async ({ canvas, userEvent }) => {
    const tooShort = schema.shape.name.safeParse("").error?.issues[0]?.message ?? "";
    const name = canvas.getByRole("textbox", { name: text.name });

    await userEvent.click(canvas.getByRole("button", { name: text.submit }));

    await expect(canvas.getByRole("alert")).toHaveTextContent(tooShort);
    await expect(name).toBeInvalid();
    await expect(canvas.getByRole("button", { name: text.submit })).toBeDisabled();
    await expect(submitted).not.toHaveBeenCalled();
  },
});

// Enter in a field submits the form.
export const SubmittedWithEnter = meta.story({
  beforeEach: () => {
    submitted.mockClear();
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(canvas.getByRole("textbox", { name: text.name }), "maintainers{Enter}");

    await waitFor(async () => {
      await expect(submitted).toHaveBeenCalledWith({
        name: "maintainers",
        description: "",
        admin: false,
      });
    });
  },
});

// A form that holds a page lays out nothing: its parts lay themselves out, and it submits as the
// stacked one does.
export const PageLayout = meta.story({
  render: () => <GroupForm onSubmit={submitted} layout="page" />,
  beforeEach: () => {
    submitted.mockClear();
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(canvas.getByRole("textbox", { name: text.name }), "maintainers{Enter}");

    await waitFor(async () => {
      await expect(submitted).toHaveBeenCalledWith({
        name: "maintainers",
        description: "",
        admin: false,
      });
    });
  },
});
