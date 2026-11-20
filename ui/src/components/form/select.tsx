import { useFieldContext } from "@/components/form/form-contexts.ts";
import { useFieldErrors } from "@/components/form/use-field-errors.ts";
import { Select as UiSelect } from "@/components/ui/select";

import type { ReactNode } from "react";
import type { Key } from "react-aria-components/Select";

type SelectProps<T> = {
  label: string;
  description?: string;
  placeholder?: string;
  items?: Iterable<T>;
  children: ReactNode | ((item: T) => ReactNode);
};

// The Select of the UI library, bound to a field of useAppForm that holds the id of the picked
// option, or null while there's none: render it with
// <form.AppField name="…">
//   {(field) => <field.Select label={m.…()} items={…}>{(item) => <ListBoxItem id={item.id}>…</ListBoxItem>}</field.Select>}
// </form.AppField>.
// It shows the field's errors once the user has left the select, or the form has been submitted.
export function Select<T extends object>(props: SelectProps<T>) {
  const field = useFieldContext<Key | null>();
  const errors = useFieldErrors();

  return (
    <UiSelect
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
