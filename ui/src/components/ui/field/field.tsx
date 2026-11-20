import { FieldError as AriaFieldError } from "react-aria-components/FieldError";
import { Label as AriaLabel } from "react-aria-components/Label";
import { Text } from "react-aria-components/Text";

import styles from "@/components/ui/field/field.module.css";

// The parts that every field of the library (TextField, Select, Checkbox, …) shares: its label, its
// description and its error. They're not used on their own: a field renders them inside the React
// Aria field that it wraps, which links them to its input through aria-labelledby and
// aria-describedby.

// The name of the field, above its control.
export function Label({ children }: { children: string }) {
  return <AriaLabel className={styles.label}>{children}</AriaLabel>;
}

// A hint below the control, which describes the input.
export function Description({ children }: { children: string }) {
  return (
    <Text slot="description" className={styles.description}>
      {children}
    </Text>
  );
}

// The error below the control, shown while the field is invalid (its isInvalid prop), which describes
// the input. role="alert" announces it once it appears.
export function FieldError({ children }: { children: string | undefined }) {
  return (
    <AriaFieldError className={styles.error}>
      <span role="alert">{children}</span>
    </AriaFieldError>
  );
}
