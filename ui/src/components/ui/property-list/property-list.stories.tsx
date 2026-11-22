import IconAt from "@tabler/icons-react/icons/IconAt";
import IconCalendar from "@tabler/icons-react/icons/IconCalendar";
import IconUser from "@tabler/icons-react/icons/IconUser";
import { expect } from "storybook/test";

import preview from "#.storybook/preview";
import { Link } from "@/components/ui/link";
import { PropertyItem, PropertyList } from "@/components/ui/property-list";
import { RelativeTime } from "@/components/ui/relative-time";
import { m } from "@/paraglide/messages.js";

// The texts of the rows, where a page passes messages and the API's data.
const text = {
  username: "Username",
  usernameValue: "ada",
  createdBy: "Created by",
  creator: "Grace Hopper",
  created: "Created",
  displayName: "Display name",
  longValue: "Augusta Ada King, Countess of Lovelace, née Byron",
};

const threeDaysAgo = new Date(Date.now() - 3 * 24 * 60 * 60 * 1000);

const meta = preview.meta({
  component: PropertyList,
  args: {
    children: (
      <>
        <PropertyItem icon={IconAt} label={text.username}>
          {text.usernameValue}
        </PropertyItem>
        <PropertyItem icon={IconUser} label={text.createdBy}>
          <Link variant="secondary" href="/">
            {text.creator}
          </Link>
        </PropertyItem>
        <PropertyItem icon={IconCalendar} label={text.created}>
          <RelativeTime date={threeDaysAgo} />
        </PropertyItem>
      </>
    ),
  },
  decorators: [
    (Story) => (
      <div style={{ inlineSize: "20rem" }}>
        <Story />
      </div>
    ),
  ],
});

// Rows of text, a link and a time: each a term and its value.
export const Values = meta.story({
  play: async ({ canvas }) => {
    const terms = canvas.getAllByRole("term");
    await expect(terms.map((term) => term.textContent)).toEqual([
      text.username,
      text.createdBy,
      text.created,
    ]);
    await expect(canvas.getAllByRole("definition")).toHaveLength(3);
    await expect(canvas.getByRole("link", { name: text.creator })).toHaveAttribute("href", "/");
    await expect(canvas.getByRole("img", { name: "3 days ago" })).toBeVisible();
  },
});

// A row without a value says that it's empty.
export const EmptyValue = meta.story({
  args: {
    children: <PropertyItem icon={IconAt} label={text.displayName} />,
  },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("definition")).toHaveTextContent(m.propertyEmpty());
  },
});

// A value longer than its column wraps within it.
export const LongValue = meta.story({
  args: {
    children: (
      <PropertyItem icon={IconAt} label={text.displayName}>
        {text.longValue.repeat(2)}
      </PropertyItem>
    ),
  },
  play: async ({ canvas }) => {
    const value = canvas.getByRole("definition");
    await expect(value.scrollWidth).toBeLessThanOrEqual(value.clientWidth);
  },
});

// The empty value in German.
export const German = meta.story({
  args: {
    children: <PropertyItem icon={IconAt} label={text.displayName} />,
  },
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("definition")).toHaveTextContent(
      m.propertyEmpty({}, { locale: "de" }),
    );
  },
});
