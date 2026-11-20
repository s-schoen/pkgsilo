import { useFieldContext } from "@/components/form/form-contexts.ts";
import { useFieldErrors } from "@/components/form/use-field-errors.ts";
import { ComboBox as UiComboBox } from "@/components/ui/combo-box";

import type { ReactNode } from "react";
import type { Key } from "react-aria-components/ComboBox";

type ComboBoxProps<T> = {
  label: string;
  description?: string;
  placeholder?: string;
  items?: Iterable<T>;
  children: ReactNode | ((item: T) => ReactNode);
};

// The ComboBox of the UI library, bound to a field of useAppForm that holds the id of the picked
// option, or null while there's none: render it as the field's Select. The text that the user types
// only filters the options; it's not the field's value.
// It shows the field's errors once the user has left the combobox, or the form has been submitted.
export function ComboBox<T extends object>(props: ComboBoxProps<T>) {
  const field = useFieldContext<Key | null>();
  const errors = useFieldErrors();

  return (
    <UiComboBox
      {...props}
      {...errors}
      name={field.name}
      value={field.state.value}
      onChange={field.handleChange}
      onBlur={() => {
        field.handleBlur();
      }}
    />
  );
}
