import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { SearchField } from "@/components/ui/search-field";

// React Aria names the clear button itself, in the locale.
const clearSearch = { en: "Clear search", de: "Suche zurücksetzen" };

const meta = preview.meta({
  component: SearchField,
  args: {
    label: "Search packages",
    placeholder: "Name or version",
    onChange: fn(),
    onClear: fn(),
    onSubmit: fn(),
  },
});

// Typing shows the clear button (hidden while there's nothing to clear), whose click empties the
// field and keeps the focus in it.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const input = canvas.getByRole("searchbox", { name: args.label });

    await expect(canvas.queryByRole("button", { name: clearSearch.en })).not.toBeInTheDocument();

    await userEvent.type(input, "react");

    await expect(args.onChange).toHaveBeenLastCalledWith("react");

    await userEvent.click(canvas.getByRole("button", { name: clearSearch.en }));

    await expect(input).toHaveValue("");
    await expect(input).toHaveFocus();
    await expect(args.onClear).toHaveBeenCalledOnce();
    await expect(canvas.queryByRole("button", { name: clearSearch.en })).not.toBeInTheDocument();
  },
});

// The keyboard reaches the input alone (the clear button is out of the tab order): Enter submits
// the text, and Escape clears it.
export const WithKeyboard = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    const input = canvas.getByRole("searchbox", { name: args.label });

    await userEvent.tab();
    await expect(input).toHaveFocus();

    await userEvent.keyboard("react{Enter}");

    await expect(args.onSubmit).toHaveBeenCalledWith("react");

    await userEvent.keyboard("{Escape}");

    await expect(input).toHaveValue("");
    await expect(args.onClear).toHaveBeenCalledOnce();

    await userEvent.tab();
    await expect(input).not.toHaveFocus();
  },
});

// The look of a toolbar's search: no box, and no visible label, so it's named by its aria-label.
export const Bare = meta.story({
  args: { variant: "bare", label: undefined, "aria-label": "Search packages" },
  play: async ({ args, canvas, userEvent }) => {
    const input = canvas.getByRole("searchbox", { name: args["aria-label"] });

    await userEvent.type(input, "react");

    await expect(input).toHaveValue("react");
  },
});

// A hint below the box.
export const WithDescription = meta.story({
  args: { description: "Matches the start of a name." },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("searchbox", { name: args.label })).toHaveAccessibleDescription(
      args.description ?? "",
    );
  },
});

// An invalid field shows its error.
export const Invalid = meta.story({
  args: { isInvalid: true, errorMessage: "Type at least two characters." },
  play: async ({ args, canvas }) => {
    const input = canvas.getByRole("searchbox", { name: args.label });

    await expect(canvas.getByRole("alert")).toHaveTextContent(args.errorMessage ?? "");
    await expect(input).toBeInvalid();
  },
});

// A disabled field takes no text, and hides its clear button, even with a value.
export const Disabled = meta.story({
  args: { isDisabled: true, defaultValue: "react" },
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("searchbox", { name: args.label })).toBeDisabled();
    await expect(canvas.queryByRole("button", { name: clearSearch.en })).not.toBeInTheDocument();
  },
});

// React Aria's name of the clear button follows the locale.
export const German = meta.story({
  globals: { locale: "de" },
  args: { label: "Pakete suchen", placeholder: "Name oder Version", defaultValue: "react" },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("button", { name: clearSearch.de })).toBeVisible();
  },
});
