import IconAlertCircle from "@tabler/icons-react/icons/IconAlertCircle";
import IconCircleCheck from "@tabler/icons-react/icons/IconCircleCheck";
import IconInfoCircle from "@tabler/icons-react/icons/IconInfoCircle";
import IconX from "@tabler/icons-react/icons/IconX";
import {
  UNSTABLE_Toast as AriaToast,
  UNSTABLE_ToastContent as AriaToastContent,
  UNSTABLE_ToastRegion as AriaToastRegion,
  Text,
} from "react-aria-components/Toast";

import { Button } from "@/components/ui/button";
import { toastQueue as appToastQueue } from "@/components/ui/toast/toast-queue.ts";
import { m } from "@/paraglide/messages.js";

import type { ToastContent, toastQueue } from "@/components/ui/toast/toast-queue.ts";

import styles from "@/components/ui/toast/toast.module.css";

const icons = {
  neutral: IconInfoCircle,
  success: IconCircleCheck,
  danger: IconAlertCircle,
};

// The region that shows the toasts of showToast() at the end of the page's bottom: the root route
// renders it once. It's a landmark, which F6 reaches, and pauses the toasts' timers while the user
// hovers or focuses it.
export function Toaster({ queue = appToastQueue }: { queue?: typeof toastQueue }) {
  return (
    <AriaToastRegion queue={queue} className={styles.region}>
      {({ toast }) => {
        const Icon = icons[toast.content.variant];
        return (
          <AriaToast toast={toast} className={`${styles.toast} ${styles[toast.content.variant]}`}>
            <Icon aria-hidden className={styles.icon} />
            <AriaToastContent className={styles.content}>
              <Text slot="title" className={styles.title}>
                {toast.content.title}
              </Text>
              {toast.content.description && (
                <Text slot="description" className={styles.description}>
                  {toast.content.description}
                </Text>
              )}
            </AriaToastContent>
            <Button slot="close" variant="ghost" size="sm" icon={IconX} aria-label={m.dismiss()} />
          </AriaToast>
        );
      }}
    </AriaToastRegion>
  );
}

export type { ToastContent };
