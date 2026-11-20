import { useFieldContext } from "@/components/form/form-contexts.ts";
import { useFieldErrors } from "@/components/form/use-field-errors.ts";
import { Checkbox as UiCheckbox } from "@/components/ui/checkbox";

type CheckboxProps = {
  label: string;
  description?: string;
};

// The Checkbox of the UI library, bound to a boolean field of useAppForm: render it with
// <form.AppField name="…">{(field) => <field.Checkbox label={m.…()} />}</form.AppField>.
// It shows the field's errors once the user has left the box, or the form has been submitted.
export function Checkbox({ label, description }: CheckboxProps) {
  const field = useFieldContext<boolean>();
  const errors = useFieldErrors();

  return (
    <UiCheckbox
      {...errors}
      description={description}
      name={field.name}
      isSelected={field.state.value}
      onChange={field.handleChange}
      onBlur={() => {
        field.handleBlur();
      }}
    >
      {label}
    </UiCheckbox>
  );
}
