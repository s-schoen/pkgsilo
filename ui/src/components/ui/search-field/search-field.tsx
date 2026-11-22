import IconSearch from "@tabler/icons-react/icons/IconSearch";
import IconX from "@tabler/icons-react/icons/IconX";
import { Button as AriaButton } from "react-aria-components/Button";
import { Group } from "react-aria-components/Group";
import { Input } from "react-aria-components/Input";
import { SearchField as AriaSearchField } from "react-aria-components/SearchField";

import { Description, FieldError, Label, fieldClassName } from "@/components/ui/field";

import type { Ref } from "react";
import type { SearchFieldProps as AriaSearchFieldProps } from "react-aria-components/SearchField";

import styles from "@/components/ui/search-field/search-field.module.css";

// validationBehavior is always "aria": the browser's own validation never blocks a form, whose
// schema (zod) validates it instead.
type SearchFieldProps = Omit<
  AriaSearchFieldProps,
  "children" | "className" | "style" | "validationBehavior" | "validate" | "type"
> & {
  // A sample of what to type, while the field is empty: a message.
  placeholder?: string;
  // For a parent that focuses the input.
  ref?: Ref<HTMLInputElement>;
} & (
    | {
        // field: the box of a field, with the label above it, as a form's.
        variant?: "field";
        // The name of the field, above the box: a message.
        label: string;
        // A hint below the box.
        description?: string;
        // The error below the box, shown while isInvalid is set.
        errorMessage?: string;
      }
    | {
        // bare: no box, as the search of a toolbar or at the top of a list; a visible label would
        // crowd it, so it's named by its aria-label.
        variant: "bare";
        "aria-label": string;
        label?: undefined;
        description?: undefined;
        errorMessage?: undefined;
      }
  );

// A text input to search with, of React Aria Components, which takes its props, such as value,
// onChange, onSubmit and onClear: <SearchField label={m.search()} value={…} onChange={…} />, or
// <SearchField variant="bare" aria-label={m.search()} … /> in a toolbar. It shows a search icon
// before the text and a button that clears it after, while there's text; Escape clears it too.
export function SearchField({
  variant = "field",
  label,
  description,
  errorMessage,
  placeholder,
  ref,
  ...props
}: SearchFieldProps) {
  return (
    <AriaSearchField
      {...props}
      validationBehavior="aria"
      className={`${fieldClassName} ${styles.searchField}`}
    >
      {label && <Label>{label}</Label>}
      <Group className={`${styles.group} ${styles[variant]}`}>
        <IconSearch aria-hidden className={styles.searchIcon} />
        <Input ref={ref} placeholder={placeholder} className={styles.input} />
        {/* React Aria names the button ("Clear search", in the locale) and takes it out of the tab
            order: the keyboard clears with Escape. */}
        <AriaButton className={styles.clear}>
          <IconX aria-hidden className={styles.clearIcon} />
        </AriaButton>
      </Group>
      {description && <Description>{description}</Description>}
      <FieldError>{errorMessage}</FieldError>
    </AriaSearchField>
  );
}
