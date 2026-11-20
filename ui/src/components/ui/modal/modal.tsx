import IconX from "@tabler/icons-react/icons/IconX";
import { Dialog, Heading } from "react-aria-components/Dialog";
import { Modal as AriaModal, ModalOverlay as AriaModalOverlay } from "react-aria-components/Modal";

import { Button } from "@/components/ui/button";
import { m } from "@/paraglide/messages.js";

import type { ReactNode } from "react";
import type { ModalOverlayProps as AriaModalOverlayProps } from "react-aria-components/Modal";

import styles from "@/components/ui/modal/modal.module.css";

// The trigger of a dialog: it pairs a button with the Modal (or ConfirmDialog) that the button opens,
// and returns the focus to the button when it closes.
export { DialogTrigger } from "react-aria-components/Dialog";

type ModalOverlayProps = Omit<AriaModalOverlayProps, "children" | "className" | "style"> & {
  children: ReactNode;
  // The width: sm for a short form or a question, md for more.
  size?: "sm" | "md";
  // Where the surface sits: near the top for a dialog, whose content may grow, or in the center for
  // a short question.
  position?: "top" | "center";
};

// The backdrop and the surface of a modal dialog, of React Aria Components, without the dialog:
// Modal and ConfirmDialog render their dialogs in it. It keeps the focus inside, hides the page from
// screen readers, and closes on Escape and, with isDismissable, a click on the backdrop.
export function ModalOverlay({
  children,
  size = "md",
  position = "top",
  ...props
}: ModalOverlayProps) {
  return (
    <AriaModalOverlay {...props} className={`${styles.overlay} ${styles[position]}`}>
      <AriaModal className={`${styles.modal} ${styles[size]}`}>{children}</AriaModal>
    </AriaModalOverlay>
  );
}

type ModalProps = Omit<ModalOverlayProps, "children" | "position"> & {
  // The dialog's heading, which names it: a message.
  title: string;
  // The content, usually a ModalBody and a ModalFooter, or a function of close for the buttons
  // that close the dialog.
  children: ReactNode | ((renderProps: { close: () => void }) => ReactNode);
};

// A modal dialog with its heading and a close button:
// <DialogTrigger>
//   <Button>{m.createGroup()}</Button>
//   <Modal title={m.createGroup()}>
//     {({ close }) => (
//       <>
//         <ModalBody>…</ModalBody>
//         <ModalFooter><Button variant="secondary" onPress={close}>{m.cancel()}</Button>…</ModalFooter>
//       </>
//     )}
//   </Modal>
// </DialogTrigger>.
// It closes on Escape, the close button and, unless isDismissable is false, a click outside; isOpen
// and onOpenChange control it without a trigger.
export function Modal({ title, children, isDismissable = true, ...props }: ModalProps) {
  return (
    <ModalOverlay {...props} isDismissable={isDismissable}>
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
            {typeof children === "function" ? children({ close }) : children}
          </>
        )}
      </Dialog>
    </ModalOverlay>
  );
}

// The content of a Modal, which scrolls when it's taller than the screen.
export function ModalBody({ children }: { children: ReactNode }) {
  return <div className={styles.body}>{children}</div>;
}

// The buttons of a Modal, at its end: the main action last.
export function ModalFooter({ children }: { children: ReactNode }) {
  return <div className={styles.footer}>{children}</div>;
}
