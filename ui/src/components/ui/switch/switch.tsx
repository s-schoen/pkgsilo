import { SwitchButton, SwitchField } from "react-aria-components/Switch";

import { Description, FieldError } from "@/components/ui/field";

import type { SwitchFieldProps } from "react-aria-components/Switch";

import styles from "@/components/ui/switch/switch.module.css";

// validationBehavior is always "aria": the browser's own validation never blocks a form, whose
// schema (zod) validates it instead.
type SwitchProps = Omit<
  SwitchFieldProps,
  "children" | "className" | "style" | "validationBehavior" | "validate"
> & {
  // The text next to the switch, which names the setting: a message.
  children: string;
  // A hint below the text.
  description?: string;
  // The error below the text, shown while isInvalid is set.
  errorMessage?: string;
};

// A switch that turns a setting on or off at once, of React Aria Components, which takes its props,
// such as isSelected, onChange and isDisabled: <Switch isSelected={…} onChange={…}>{m.enabled()}
// </Switch>. A choice that a form submits later is a Checkbox instead.
export function Switch({ children, description, errorMessage, ...props }: SwitchProps) {
  return (
    <SwitchField {...props} validationBehavior="aria" className={styles.field}>
      <SwitchButton className={styles.button}>
        <span className={styles.track}>
          <span className={styles.thumb} />
        </span>
        {children}
      </SwitchButton>
      {description && <Description>{description}</Description>}
      <FieldError>{errorMessage}</FieldError>
    </SwitchField>
  );
}
