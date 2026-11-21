import IconAlertCircle from "@tabler/icons-react/icons/IconAlertCircle";

import styles from "@/components/ui/alert/alert.module.css";

type AlertProps = {
  // What went wrong, and what the user can do about it: a message.
  children: string;
};

// A message about a failure on the page itself, such as a form's that no field explains:
// {error && <Alert>{m.signInFailed()}</Alert>}. It's role="alert", so a screen reader announces it
// once it appears. The outcome of an action that leaves the page is a toast instead.
export function Alert({ children }: AlertProps) {
  return (
    <div role="alert" className={styles.alert}>
      <IconAlertCircle aria-hidden className={styles.icon} />
      <p className={styles.text}>{children}</p>
    </div>
  );
}
