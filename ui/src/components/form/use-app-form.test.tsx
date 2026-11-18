import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { z } from "zod/v4";

import { useAppForm } from "@/components/form/use-app-form.ts";

const schema = z.object({ name: z.string().min(3) });

type Values = z.infer<typeof schema>;

// The message that zod gives for a name that's too short, so that the tests don't repeat zod's text.
const tooShort = schema.safeParse({ name: "ab" }).error?.issues[0]?.message;

function NameForm({ onSubmit }: { onSubmit: (value: Values) => Promise<void> | void }) {
  const form = useAppForm({
    defaultValues: { name: "" },
    validators: { onChange: schema },
    onSubmit: ({ value }) => onSubmit(value),
  });

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        void form.handleSubmit();
      }}
    >
      <form.AppField name="name">{(field) => <field.TextField label="Name" />}</form.AppField>
      <form.AppForm>
        <form.SubmitButton label="Save" />
      </form.AppForm>
    </form>
  );
}

describe("useAppForm", () => {
  it("labels the text field", () => {
    render(<NameForm onSubmit={vi.fn<(value: Values) => void>()} />);

    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("");
  });

  it("shows no error before the user leaves the field", async () => {
    const user = userEvent.setup();
    render(<NameForm onSubmit={vi.fn<(value: Values) => void>()} />);

    await user.type(screen.getByRole("textbox", { name: "Name" }), "ab");

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows the schema's error once the user leaves the field", async () => {
    const user = userEvent.setup();
    render(<NameForm onSubmit={vi.fn<(value: Values) => void>()} />);
    const input = screen.getByRole("textbox", { name: "Name" });

    await user.type(input, "ab");
    await user.tab();

    expect(screen.getByRole("alert")).toHaveTextContent(tooShort ?? "");
    expect(input).toBeInvalid();
    expect(input).toHaveAccessibleDescription(tooShort ?? "");
  });

  it("removes the error once the value is valid", async () => {
    const user = userEvent.setup();
    render(<NameForm onSubmit={vi.fn<(value: Values) => void>()} />);
    const input = screen.getByRole("textbox", { name: "Name" });
    await user.type(input, "ab");
    await user.tab();

    await user.type(input, "c");

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(input).toBeValid();
  });

  it("submits the values", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn<(value: Values) => void>();
    render(<NameForm onSubmit={onSubmit} />);

    await user.type(screen.getByRole("textbox", { name: "Name" }), "abc");
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(onSubmit).toHaveBeenCalledWith({ name: "abc" });
  });

  it("doesn't submit invalid values, and shows their errors", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn<(value: Values) => void>();
    render(<NameForm onSubmit={onSubmit} />);

    await user.type(screen.getByRole("textbox", { name: "Name" }), "ab");
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(tooShort ?? "");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("disables the submit button while the form submits", async () => {
    const user = userEvent.setup();
    let finishSubmission = () => {};
    const submission = new Promise<void>((resolve) => {
      finishSubmission = resolve;
    });
    render(<NameForm onSubmit={() => submission} />);
    await user.type(screen.getByRole("textbox", { name: "Name" }), "abc");

    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    finishSubmission();
    expect(await screen.findByRole("button", { name: "Save" })).toBeEnabled();
  });
});
