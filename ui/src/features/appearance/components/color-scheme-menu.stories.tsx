import { expect, screen, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { ColorSchemeMenu } from "@/features/appearance/components/color-scheme-menu.tsx";
import { writeColorScheme } from "@/features/appearance/utils/color-scheme.ts";
import { m } from "@/paraglide/messages.js";

const meta = preview.meta({
  component: ColorSchemeMenu,
  // Each story starts from no stored preference, and leaves none behind; it removes only its own
  // key, since Storybook's manager keeps its state in the same storage. The toolbar's scheme
  // decorator sets <html>'s attribute again for the next story.
  beforeEach: () => {
    writeColorScheme("system");
    return () => {
      writeColorScheme("system");
    };
  },
});

// A click opens the menu with System checked; a click on Dark forces the dark scheme, and the
// menu then checks Dark.
export const Default = meta.story({
  play: async ({ canvas, userEvent }) => {
    const trigger = canvas.getByRole("button", { name: m.colorScheme() });

    await userEvent.click(trigger);
    await expect(
      await screen.findByRole("menuitemradio", { name: m.colorSchemeSystem() }),
    ).toHaveAttribute("aria-checked", "true");

    await userEvent.click(screen.getByRole("menuitemradio", { name: m.colorSchemeDark() }));

    await expect(document.documentElement).toHaveAttribute("data-color-scheme", "dark");
    await waitFor(async () => {
      await expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    });

    await userEvent.click(trigger);
    await expect(
      await screen.findByRole("menuitemradio", { name: m.colorSchemeDark() }),
    ).toHaveAttribute("aria-checked", "true");
  },
});

// Enter opens the menu, the arrow keys move to Light, and Enter picks it; the focus returns to the
// button.
export const ChosenWithKeyboard = meta.story({
  globals: { colorScheme: "dark" },
  play: async ({ canvas, userEvent }) => {
    const trigger = canvas.getByRole("button", { name: m.colorScheme() });

    await userEvent.tab();
    await userEvent.keyboard("{Enter}");
    await waitFor(async () => {
      await expect(
        screen.getByRole("menuitemradio", { name: m.colorSchemeSystem() }),
      ).toHaveFocus();
    });

    await userEvent.keyboard("{ArrowDown}{Enter}");

    await expect(document.documentElement).toHaveAttribute("data-color-scheme", "light");
    await waitFor(async () => {
      await expect(trigger).toHaveFocus();
    });
  },
});

// The preference that the browser stored is the checked one.
export const StoredPreference = meta.story({
  beforeEach: () => {
    writeColorScheme("light");
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: m.colorScheme() }));

    await expect(
      await screen.findByRole("menuitemradio", { name: m.colorSchemeLight() }),
    ).toHaveAttribute("aria-checked", "true");
  },
});

// The button's name in German.
export const German = meta.story({
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(
      canvas.getByRole("button", { name: m.colorScheme({}, { locale: "de" }) }),
    ).toBeVisible();
  },
});
