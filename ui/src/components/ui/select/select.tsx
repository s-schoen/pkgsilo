import IconChevronDown from "@tabler/icons-react/icons/IconChevronDown";
import { Button as AriaButton } from "react-aria-components/Button";
import { Select as AriaSelect, SelectValue } from "react-aria-components/Select";

import { Description, FieldError, Label, fieldClassName } from "@/components/ui/field";
import { ListBox } from "@/components/ui/list-box";
import { Popover } from "@/components/ui/popover";

import type { ReactNode, Ref } from "react";
import type { SelectProps as AriaSelectProps } from "react-aria-components/Select";

import styles from "@/components/ui/select/select.module.css";

// One value only; validationBehavior is always "aria": the browser's own validation never blocks a
// form, whose schema (zod) validates it instead.
type SelectProps<T> = Omit<
  AriaSelectProps<T>,
  "children" | "className" | "style" | "validationBehavior" | "validate" | "selectionMode"
> & {
  // The name of the field, above the trigger: a message.
  label: string;
  // A hint below the trigger.
  description?: string;
  // The error below the trigger, shown while isInvalid is set.
  errorMessage?: string;
  // The trigger's text while nothing is selected: a message.
  placeholder?: string;
  // The options, rendered by children as ListBoxItems (@/components/ui/list-box).
  items?: Iterable<T>;
  children: ReactNode | ((item: T) => ReactNode);
  // For a parent that focuses the trigger.
  ref?: Ref<HTMLButtonElement>;
};

// A field that picks one of a few options from a list, of React Aria Components, which takes its
// props, such as value, onChange and isDisabled:
// <Select label={m.role()} items={roles} value={…} onChange={…}>
//   {(role) => <ListBoxItem id={role.id}>{role.name}</ListBoxItem>}
// </Select>.
// Many options that the user would rather type are a ComboBox instead. In a form of useAppForm, use
// the field's Select (@/components/form), which binds it.
export function Select<T extends object>({
  label,
  description,
  errorMessage,
  items,
  children,
  ref,
  ...props
}: SelectProps<T>) {
  return (
    <AriaSelect
      {...props}
      validationBehavior="aria"
      className={`${fieldClassName} ${styles.select}`}
    >
      <Label>{label}</Label>
      <AriaButton ref={ref} className={styles.trigger}>
        <SelectValue className={styles.value} />
        <IconChevronDown aria-hidden className={styles.chevron} />
      </AriaButton>
      {description && <Description>{description}</Description>}
      <FieldError>{errorMessage}</FieldError>
      <Popover width="trigger">
        <ListBox items={items}>{children}</ListBox>
      </Popover>
    </AriaSelect>
  );
}
