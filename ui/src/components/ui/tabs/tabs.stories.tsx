import IconKey from "@tabler/icons-react/icons/IconKey";
import IconUsers from "@tabler/icons-react/icons/IconUsers";
import { expect, fn } from "storybook/test";

import preview from "#.storybook/preview";
import { Tab, TabList, TabPanel, Tabs } from "@/components/ui/tabs";

// The texts of the tabs, where a view passes messages.
const text = {
  list: "Identity",
  groups: "Groups",
  keys: "API keys",
  password: "Password",
  groupsPanel: "Member of 3 groups.",
  keysPanel: "2 active keys.",
  passwordPanel: "Set 4 days ago.",
};

const meta = preview.meta({
  component: Tabs,
  args: { onSelectionChange: fn() },
  render: (args) => (
    <Tabs {...args}>
      <TabList aria-label={text.list}>
        <Tab id="groups" icon={IconUsers}>
          {text.groups}
        </Tab>
        <Tab id="keys" icon={IconKey}>
          {text.keys}
        </Tab>
        <Tab id="password">{text.password}</Tab>
      </TabList>
      <TabPanel id="groups">{text.groupsPanel}</TabPanel>
      <TabPanel id="keys">{text.keysPanel}</TabPanel>
      <TabPanel id="password">{text.passwordPanel}</TabPanel>
    </Tabs>
  ),
});

// A click on a tab shows its panel.
export const Default = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await expect(canvas.getByRole("tab", { name: text.groups })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    await userEvent.click(canvas.getByRole("tab", { name: text.keys }));

    await expect(canvas.getByRole("tab", { name: text.keys })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(canvas.getByRole("tabpanel")).toHaveTextContent(text.keysPanel);
    await expect(args.onSelectionChange).toHaveBeenCalledWith("keys");
  },
});

// The arrow keys move between the tabs and select them.
export const SelectedWithKeyboard = meta.story({
  play: async ({ canvas, userEvent }) => {
    await userEvent.tab();
    await userEvent.keyboard("{ArrowRight}{ArrowRight}");

    const password = canvas.getByRole("tab", { name: text.password });
    await expect(password).toHaveFocus();
    await expect(password).toHaveAttribute("aria-selected", "true");
    await expect(canvas.getByRole("tabpanel")).toHaveTextContent(text.passwordPanel);
  },
});

// A tab that can't be selected.
export const DisabledTab = meta.story({
  args: { disabledKeys: ["password"] },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("tab", { name: text.password })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  },
});
