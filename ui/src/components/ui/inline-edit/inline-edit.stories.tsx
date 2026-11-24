import { useState } from "react";
import { expect, fireEvent, fn, spyOn, waitFor } from "storybook/test";

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
  lock: "Lock",
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

// A field that turns read-only once the button beside it is pressed, as when the policies change
// while the user edits it.
function LockingField(props: ComponentProps<typeof InlineTextField>) {
  const [isReadOnly, setIsReadOnly] = useState(false);
  return (
    <>
      <UsernameField {...props} isReadOnly={isReadOnly} />
      <button
        type="button"
        onClick={() => {
          setIsReadOnly(true);
        }}
      >
        {text.lock}
      </button>
    </>
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

    // The pointer may rest where the view rendered, from a story before, which enters nothing: the
    // pointer leaves the view first. On a busy machine, the browser also moves its own pointer
    // while the page still lays out, as a late font or style does, and its events over another
    // element end the hover. So the pointer enters again until the hover holds.
    await waitFor(
      async () => {
        await userEvent.unhover(view);
        await userEvent.hover(view);
        await expect(view).toHaveAttribute("data-hovered");
      },
      { timeout: 10000 },
    );
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

// Finishes the pending save of FocusMovedOn's first field.
let finishSave = () => {};

// After Enter, the user moves on to another field while the save runs: the save's end leaves the
// focus there, and that field's edit goes on.
export const FocusMovedOn = meta.story({
  args: {
    onSave: fn(
      () =>
        new Promise<void>((resolve) => {
          finishSave = resolve;
        }),
    ),
  },
  render: (args) => (
    <>
      <UsernameField {...args} />
      <UsernameField
        {...args}
        label={text.displayName}
        value={text.title}
        onSave={fn(async () => {})}
      />
    </>
  ),
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(`${text.changed}{Enter}`);
    await expect(args.onSave).toHaveBeenCalledOnce();

    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: text.displayName }) }),
    );
    const other = await canvas.findByRole("textbox", { name: text.displayName });
    await waitFor(() => expect(other).toHaveFocus());
    finishSave();

    await expect(await canvas.findByText(text.changed)).toBeVisible();
    // The editor selected its value, which the typing replaces.
    await userEvent.keyboard("Ada King");
    await expect(canvas.getByRole("textbox", { name: text.displayName })).toHaveValue("Ada King");
  },
});

// The window loses the focus, as when the user switches to another tab: the edit stays, unsaved.
export const WindowLeft = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(text.changed);

    const hasFocus = spyOn(document, "hasFocus").mockReturnValue(false);
    try {
      canvas.getByRole("textbox", { name: args.label }).blur();
    } finally {
      hasFocus.mockRestore();
    }

    await expect(args.onSave).not.toHaveBeenCalled();
    await expect(canvas.getByRole("textbox", { name: args.label })).toHaveValue(text.changed);
  },
});

// The Enter that confirms an input method's composition, as of Japanese characters, doesn't save;
// the next one does.
export const Composing = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(text.changed);
    const input = canvas.getByRole("textbox", { name: args.label });

    await fireEvent.keyDown(input, { key: "Enter", isComposing: true });
    // Safari's, after the composition has ended.
    await fireEvent.keyDown(input, { key: "Enter", keyCode: 229 });

    await expect(args.onSave).not.toHaveBeenCalled();
    await expect(input).toHaveValue(text.changed);

    await userEvent.keyboard("{Enter}");

    await expect(args.onSave).toHaveBeenCalledWith(text.changed);
  },
});

// A value that the actor may not change is plain text, without the edit mode.
export const ReadOnly = meta.story({
  args: { isReadOnly: true },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(text.value)).toBeVisible();
    await expect(canvas.queryByRole("button")).not.toBeInTheDocument();
  },
});

// A value that turns read-only during its edit ends the edit as Escape does: the value before it
// shows as text, unsaved, and the edit mode ended.
export const TurnsReadOnlyWhileEditing = meta.story({
  render: (args) => <LockingField {...args} />,
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(text.changed);

    // Without moving the focus, as a refetch while the user is in another tab.
    await fireEvent.click(canvas.getByRole("button", { name: text.lock }));

    await waitFor(() => expect(canvas.queryByRole("textbox")).not.toBeInTheDocument());
    await expect(canvas.getByText(text.value)).toBeVisible();
    await expect(canvas.queryByText(text.changed)).not.toBeInTheDocument();
    await expect(
      canvas.queryByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    ).not.toBeInTheDocument();
    await expect(args.onSave).not.toHaveBeenCalled();
    await expect(args.onEditingChange).toHaveBeenLastCalledWith(false);
  },
});

// The save of TurnsReadOnlyWhileSaving, which the story fails once the value has turned read-only.
const runningSave = { fail: (_error: Error) => {} };

// A value that turns read-only while its save runs keeps the editor until the save has finished,
// since the API decides it; a save that the API rejects then ends the edit as Escape does.
export const TurnsReadOnlyWhileSaving = meta.story({
  args: {
    onSave: fn(
      () =>
        new Promise<void>((_resolve, reject) => {
          runningSave.fail = reject;
        }),
    ),
  },
  render: (args) => <LockingField {...args} />,
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(
      canvas.getByRole("button", { name: m.inlineEditLabel({ label: args.label }) }),
    );
    // The editor takes the focus once it has rendered.
    await waitFor(() => expect(canvas.getByRole("textbox", { name: args.label })).toHaveFocus());
    await userEvent.keyboard(`${text.changed}{Enter}`);
    await expect(canvas.getByRole("progressbar", { name: m.inlineEditSaving() })).toBeVisible();

    await fireEvent.click(canvas.getByRole("button", { name: text.lock }));

    await expect(canvas.getByRole("textbox", { name: args.label })).toHaveValue(text.changed);

    runningSave.fail(new InlineEditError(text.taken));

    await waitFor(() => expect(canvas.queryByRole("textbox")).not.toBeInTheDocument());
    await expect(canvas.getByText(text.value)).toBeVisible();
    await expect(canvas.queryByRole("alert")).not.toBeInTheDocument();
    await expect(args.onEditingChange).toHaveBeenLastCalledWith(false);
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
