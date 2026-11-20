import IconAlertTriangle from "@tabler/icons-react/icons/IconAlertTriangle";
import { useState } from "react";
import { Dialog, Heading } from "react-aria-components/Dialog";
import { Text } from "react-aria-components/Text";

import { Button } from "@/components/ui/button";
import { ModalOverlay } from "@/components/ui/modal";
import { m } from "@/paraglide/messages.js";

import styles from "@/components/ui/confirm-dialog/confirm-dialog.module.css";

type ConfirmDialogProps = {
  // The question: a message, such as "Delete the group maintainers?".
  title: string;
  // What follows from the action: a message.
  description: string;
  // The action, on the confirm button: a message, such as "Delete".
  confirmLabel: string;
  // danger for an action that destroys data, with a warning icon and a danger button.
  variant?: "danger" | "primary";
  // Runs the action. The dialog closes once its promise resolves; while it runs, the confirm button
  // is pending. If it rejects, the dialog stays open: onConfirm reports the error itself, such as
  // with a toast.
  onConfirm: () => Promise<void> | void;
  // Control the dialog without a DialogTrigger.
  isOpen?: boolean;
  onOpenChange?: (isOpen: boolean) => void;
};

// A question before an action that can't be undone, in an alert dialog:
// <DialogTrigger>
//   <Button variant="danger-outline">{m.delete()}</Button>
//   <ConfirmDialog title={…} description={…} confirmLabel={m.delete()} onConfirm={…} />
// </DialogTrigger>.
// The focus starts on Cancel, so that Enter doesn't confirm by accident; Escape cancels too.
export function ConfirmDialog({
  title,
  description,
  confirmLabel,
  variant = "danger",
  onConfirm,
  ...props
}: ConfirmDialogProps) {
  const [isPending, setPending] = useState(false);

  async function confirm(close: () => void) {
    setPending(true);
    try {
      await onConfirm();
      close();
    } catch {
      // onConfirm has reported the error; the dialog stays open, so that the user can try again.
    } finally {
      setPending(false);
    }
  }

  return (
    <ModalOverlay {...props} size="sm" position="center">
      <Dialog role="alertdialog" className={styles.dialog}>
        {({ close }) => (
          <>
            <div className={styles.content}>
              {variant === "danger" && (
                <span className={styles.icon}>
                  <IconAlertTriangle aria-hidden />
                </span>
              )}
              <Heading slot="title" className={styles.title}>
                {title}
              </Heading>
              <Text slot="description" elementType="p" className={styles.description}>
                {description}
              </Text>
            </div>
            <div className={styles.actions}>
              {/* oxlint-disable-next-line jsx-a11y/no-autofocus -- An alert dialog focuses its least destructive action (WAI-ARIA's alertdialog pattern), so that Enter doesn't confirm by accident. */}
              <Button variant="secondary" autoFocus isDisabled={isPending} onPress={close}>
                {m.cancel()}
              </Button>
              <Button
                variant={variant}
                isPending={isPending}
                onPress={() => {
                  void confirm(close);
                }}
              >
                {confirmLabel}
              </Button>
            </div>
          </>
        )}
      </Dialog>
    </ModalOverlay>
  );
}
