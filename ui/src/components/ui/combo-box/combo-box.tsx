import IconChevronDown from "@tabler/icons-react/icons/IconChevronDown";
import { Button as AriaButton } from "react-aria-components/Button";
import { ComboBox as AriaComboBox } from "react-aria-components/ComboBox";
import { Group } from "react-aria-components/Group";
import { Input } from "react-aria-components/Input";

import { Description, FieldError, Label, fieldClassName } from "@/components/ui/field";
import { ListBox } from "@/components/ui/list-box";
import { Popover } from "@/components/ui/popover";

import type { ReactNode, Ref } from "react";
import type { ComboBoxProps as AriaComboBoxProps } from "react-aria-components/ComboBox";

import styles from "@/components/ui/combo-box/combo-box.module.css";

// One value only; validationBehavior is always "aria": the browser's own validation never blocks a
// form, whose schema (zod) validates it instead.
type ComboBoxProps<T> = Omit<
  AriaComboBoxProps<T>,
  "children" | "className" | "style" | "validationBehavior" | "validate" | "selectionMode"
> & {
  // The name of the field, above the input: a message.
  label: string;
  // A hint below the input.
  description?: string;
  // The error below the input, shown while isInvalid is set.
  errorMessage?: string;
  // A sample of what to type, while the input is empty: a message.
  placeholder?: string;
  // The options, rendered by children as ListBoxItems (@/components/ui/list-box).
  items?: Iterable<T>;
  children: ReactNode | ((item: T) => ReactNode);
  // For a parent that focuses the input.
  ref?: Ref<HTMLInputElement>;
};

// A field that picks one of many options by typing part of it, of React Aria Components, which takes
// its props, such as value, onChange, inputValue, onInputChange and allowsCustomValue:
// <ComboBox label={m.member()} items={identities} value={…} onChange={…}>
//   {(identity) => <ListBoxItem id={identity.id}>{identity.name}</ListBoxItem>}
// </ComboBox>.
// It filters static items itself, and says "No results" when none match; for items that the API searches, pass inputValue and
// onInputChange, and items for the input. In a form of useAppForm, use the field's ComboBox
// (@/components/form), which binds it.
export function ComboBox<T extends object>({
  label,
  description,
  errorMessage,
  placeholder,
  items,
  children,
  ref,
  ...props
}: ComboBoxProps<T>) {
  return (
    <AriaComboBox
      allowsEmptyCollection
      {...props}
      validationBehavior="aria"
      className={`${fieldClassName} ${styles.comboBox}`}
    >
      <Label>{label}</Label>
      <Group className={styles.group}>
        <Input ref={ref} placeholder={placeholder} className={styles.input} />
        <AriaButton className={styles.button}>
          <IconChevronDown aria-hidden className={styles.chevron} />
        </AriaButton>
      </Group>
      {description && <Description>{description}</Description>}
      <FieldError>{errorMessage}</FieldError>
      <Popover width="trigger">
        <ListBox items={items}>{children}</ListBox>
      </Popover>
    </AriaComboBox>
  );
}
