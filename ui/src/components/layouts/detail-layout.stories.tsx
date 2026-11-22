import IconAt from "@tabler/icons-react/icons/IconAt";
import IconClock from "@tabler/icons-react/icons/IconClock";
import { expect } from "storybook/test";

import preview from "#.storybook/preview";
import { DetailLayout } from "@/components/layouts/detail-layout.tsx";
import { PropertyItem, PropertyList } from "@/components/ui/property-list";

// The texts of the page, where a page passes messages and the API's names.
const text = {
  title: "Ada Lovelace",
  username: "Username",
  usernameValue: "ada",
  updated: "Updated",
  updatedValue: "3 days ago",
  body: "The main column's body, such as the object's tabs.",
};

const meta = preview.meta({
  component: DetailLayout,
  args: {
    title: <h1>{text.title}</h1>,
    sidebar: (
      <PropertyList>
        <PropertyItem icon={IconAt} label={text.username}>
          {text.usernameValue}
        </PropertyItem>
        <PropertyItem icon={IconClock} label={text.updated}>
          {text.updatedValue}
        </PropertyItem>
      </PropertyList>
    ),
    children: <p>{text.body}</p>,
  },
});

// The parts' boxes, to tell where the layout put them.
function boxes(canvasElement: HTMLElement) {
  const box = (element: Element | null) => {
    if (element === null) {
      throw new Error("part not found");
    }
    return element.getBoundingClientRect();
  };
  return {
    title: box(canvasElement.querySelector("h1")),
    sidebar: box(canvasElement.querySelector("dl")),
    body: box(canvasElement.querySelector("p")),
  };
}

// From --lg, the title and the body in the main column at the start, the sidebar beside them.
export const Wide = meta.story({
  globals: { viewport: { value: "desktop", isRotated: false } },
  play: async ({ canvas, canvasElement }) => {
    await expect(canvas.getByRole("heading", { level: 1, name: text.title })).toBeVisible();
    const { title, sidebar, body } = boxes(canvasElement);
    await expect(sidebar.left).toBeGreaterThan(title.right);
    await expect(body.top).toBeGreaterThan(title.bottom);
    await expect(body.right).toBeLessThan(sidebar.left);
  },
});

// Below --lg, one column: the title, the sidebar, then the body.
export const Narrow = meta.story({
  globals: { viewport: { value: "mobile2", isRotated: false } },
  play: async ({ canvasElement }) => {
    const { title, sidebar, body } = boxes(canvasElement);
    await expect(sidebar.top).toBeGreaterThan(title.bottom);
    await expect(body.top).toBeGreaterThan(sidebar.bottom);
  },
});

// Without a body, as the create page: the title and the sidebar only.
export const WithoutBody = meta.story({
  args: { children: undefined },
  globals: { viewport: { value: "desktop", isRotated: false } },
  play: async ({ canvas }) => {
    await expect(canvas.queryByText(text.body)).not.toBeInTheDocument();
    await expect(canvas.getByText(text.usernameValue)).toBeVisible();
  },
});
