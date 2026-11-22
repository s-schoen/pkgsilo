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
  "children" | "className" | "style" | "validationBehavior" | "validate" | "slot"
> & {
  // A hint below the text.
  description?: string;
  // The error below the text, shown while isInvalid is set.
  errorMessage?: string;
} & (
    | {
        // The text next to the box, which names it: a message.
        children: string;
        slot?: undefined;
      }
    // The box alone, as a row's selection in a GridList, which names it ("Select") and binds it.
    | { children?: undefined; slot: "selection" }
  );

// A checkbox with its text, of React Aria Components, which takes its props, such as isSelected,
// onChange, isIndeterminate and isDisabled: <Checkbox isSelected={…} onChange={…}>{m.admin()}
// </Checkbox>. The text and the box both toggle it. In a form of useAppForm, use the field's
// Checkbox (@/components/form), which binds it. In a GridListItem, <Checkbox slot="selection" />
// is the row's selection box.
export function Checkbox({ children, description, errorMessage, ...props }: CheckboxProps) {
  return (
    <CheckboxField
      {...props}
      validationBehavior="aria"
      className={children ? styles.field : `${styles.field} ${styles.boxOnly}`}
    >
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
