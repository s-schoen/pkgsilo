import IconCheck from "@tabler/icons-react/icons/IconCheck";
import IconMinus from "@tabler/icons-react/icons/IconMinus";
import { CheckboxButton, CheckboxField } from "react-aria-components/Checkbox";

import { Description, FieldError } from "@/components/ui/field";

import type { CheckboxFieldProps } from "react-aria-components/Checkbox";

import styles from "@/components/ui/checkbox/checkbox.module.css";

// validationBehavior is always "aria": the browser's own validation never blocks a form, whose
// schema (zod) validates it instead.
type CheckboxProps = Omit<
  CheckboxFieldProps,
  "children" | "className" | "style" | "validationBehavior" | "validate"
> & {
  // The text next to the box, which names it: a message.
  children: string;
  // A hint below the text.
  description?: string;
  // The error below the text, shown while isInvalid is set.
  errorMessage?: string;
};

// A checkbox with its text, of React Aria Components, which takes its props, such as isSelected,
// onChange, isIndeterminate and isDisabled: <Checkbox isSelected={…} onChange={…}>{m.admin()}
// </Checkbox>. The text and the box both toggle it. In a form of useAppForm, use the field's
// Checkbox (@/components/form), which binds it.
export function Checkbox({ children, description, errorMessage, ...props }: CheckboxProps) {
  return (
    <CheckboxField {...props} validationBehavior="aria" className={styles.field}>
      <CheckboxButton className={styles.button}>
        {({ isSelected, isIndeterminate }) => (
          <>
            <span className={styles.box}>
              {isIndeterminate ? (
                <IconMinus aria-hidden className={styles.icon} />
              ) : (
                isSelected && <IconCheck aria-hidden className={styles.icon} />
              )}
            </span>
            {children}
          </>
        )}
      </CheckboxButton>
      {description && <Description>{description}</Description>}
      <FieldError>{errorMessage}</FieldError>
    </CheckboxField>
  );
}
