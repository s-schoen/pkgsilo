import { expect } from "storybook/test";

import preview from "#.storybook/preview";
import { Alert } from "@/components/ui/alert";

const meta = preview.meta({
  component: Alert,
  args: { children: "The username or password is wrong." },
});

// A short failure, as a form's.
export const Default = meta.story({
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("alert")).toHaveTextContent(args.children);
    await expect(canvas.getByRole("alert").querySelector("svg")).toHaveAttribute(
      "aria-hidden",
      "true",
    );
  },
});

// A long text wraps next to the icon, which stays on the first line.
export const LongText = meta.story({
  args: {
    children:
      "Signing in failed, because pkgsilo couldn't be reached. Check your connection, then try again in a moment.",
  },
  decorators: [
    (Story) => (
      <div style={{ inlineSize: "20rem" }}>
        <Story />
      </div>
    ),
  ],
  play: async ({ args, canvas }) => {
    await expect(canvas.getByRole("alert")).toHaveTextContent(args.children);
  },
});
