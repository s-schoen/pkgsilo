import IconX from "@tabler/icons-react/icons/IconX";
import { Dialog, Heading } from "react-aria-components/Dialog";
import { Modal as AriaModal, ModalOverlay as AriaModalOverlay } from "react-aria-components/Modal";

import { Button } from "@/components/ui/button";
import { m } from "@/paraglide/messages.js";

import type { ReactNode } from "react";
import type { ModalOverlayProps as AriaModalOverlayProps } from "react-aria-components/Modal";

import styles from "@/components/ui/drawer/drawer.module.css";

// The trigger of a drawer: it pairs a button with the Drawer that the button opens, and returns the
// focus to the button when it closes.
export { DialogTrigger } from "react-aria-components/Dialog";

type DrawerProps = Omit<
  AriaModalOverlayProps,
  "children" | "className" | "style" | "isDismissable"
> & {
  // The drawer's heading, which names it: a message.
  title: string;
  // The content, or a function of close for the content that closes the drawer.
  children: ReactNode | ((renderProps: { close: () => void }) => ReactNode);
};

// A modal panel that slides in from the start of the screen, of React Aria Components, as the app's
// navigation on a small screen:
// <DialogTrigger>
//   <Button variant="ghost" icon={IconMenu2} aria-label={m.openNavigation()} />
//   <Drawer title={m.navigation()}>…</Drawer>
// </DialogTrigger>.
// It keeps the focus inside, and closes on Escape, its close button and a click outside; isOpen and
// onOpenChange control it without a trigger.
export function Drawer({ title, children, ...props }: DrawerProps) {
  return (
    <AriaModalOverlay {...props} isDismissable className={styles.overlay}>
      <AriaModal className={styles.drawer}>
        <Dialog className={styles.dialog}>
          {({ close }) => (
            <>
              <header className={styles.header}>
                <Heading slot="title" className={styles.title}>
                  {title}
                </Heading>
                <Button
                  variant="ghost"
                  size="sm"
                  icon={IconX}
                  aria-label={m.close()}
                  onPress={close}
                />
              </header>
              <div className={styles.body}>
                {typeof children === "function" ? children({ close }) : children}
              </div>
            </>
          )}
        </Dialog>
      </AriaModal>
    </AriaModalOverlay>
  );
}
