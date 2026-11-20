import { UNSTABLE_ToastQueue as ToastQueue } from "react-aria-components/Toast";

// React Aria's toasts are still UNSTABLE_ in this version: only toast/ names them, so that a change
// of their API stays in this folder.

export type ToastContent = {
  // What happened: a message, such as "Group created".
  title: string;
  // More about it: a message.
  description?: string;
  // The status: success for an action that worked, danger for one that failed, neutral otherwise.
  variant: "neutral" | "success" | "danger";
};

// The toasts of the app, which the Toaster in the root route shows.
export const toastQueue = new ToastQueue<ToastContent>({ maxVisibleToasts: 5 });

// The time a toast stays, unless the user hovers or focuses it.
const timeout = 5000;

// Shows a toast: showToast({ title: m.groupCreated(), variant: "success" }). A danger toast stays
// until the user dismisses it, so that an error isn't missed; the others leave after 5 seconds.
// Returns the toast's key, for toastQueue.close(key).
export function showToast(content: ToastContent): string {
  return toastQueue.add(content, {
    timeout: content.variant === "danger" ? undefined : timeout,
  });
}
