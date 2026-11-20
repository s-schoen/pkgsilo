import {
  Tab as AriaTab,
  TabList as AriaTabList,
  TabPanel as AriaTabPanel,
  Tabs as AriaTabs,
} from "react-aria-components/Tabs";

import type { TablerIcon } from "@tabler/icons-react";
import type {
  TabListProps as AriaTabListProps,
  TabPanelProps as AriaTabPanelProps,
  TabProps as AriaTabProps,
  TabsProps as AriaTabsProps,
} from "react-aria-components/Tabs";

import styles from "@/components/ui/tabs/tabs.module.css";

// Views of one object that the user switches between, of React Aria Components, which takes its
// props, such as selectedKey and onSelectionChange:
// <Tabs>
//   <TabList aria-label={m.identity()}>
//     <Tab id="groups">{m.groups()}</Tab>
//     <Tab id="keys">{m.apiKeys()}</Tab>
//   </TabList>
//   <TabPanel id="groups">…</TabPanel>
//   <TabPanel id="keys">…</TabPanel>
// </Tabs>.
// The arrow keys move between the tabs. A tab with href is a link that navigates through the
// router: the route then picks the selected tab with selectedKey.
export function Tabs(props: Omit<AriaTabsProps, "className" | "style">) {
  return <AriaTabs {...props} className={styles.tabs} />;
}

// The row of a Tabs' tabs, named by its aria-label.
export function TabList<T extends object>(props: Omit<AriaTabListProps<T>, "className" | "style">) {
  return <AriaTabList {...props} className={styles.tabList} />;
}

type TabProps = Omit<AriaTabProps, "children" | "className" | "style"> & {
  // The tab's name: a message.
  children: string;
  // An icon of @tabler/icons-react before the name, decorative.
  icon?: TablerIcon;
};

// A tab of a TabList, with a bar under it while it's selected.
export function Tab({ children, icon: Icon, ...props }: TabProps) {
  return (
    <AriaTab {...props} className={styles.tab}>
      <span className={styles.label}>
        {Icon && <Icon aria-hidden className={styles.icon} />}
        {children}
      </span>
      <span className={styles.bar} />
    </AriaTab>
  );
}

// The content of the tab with the same id.
export function TabPanel(props: Omit<AriaTabPanelProps, "className" | "style">) {
  return <AriaTabPanel {...props} className={styles.tabPanel} />;
}
