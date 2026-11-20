import { useFieldContext } from "@/components/form/form-contexts.ts";
import { useFieldErrors } from "@/components/form/use-field-errors.ts";
import { Switch as UiSwitch } from "@/components/ui/switch";

type SwitchProps = {
  label: string;
  description?: string;
};

// The Switch of the UI library, bound to a boolean field of useAppForm: render it with
// <form.AppField name="…">{(field) => <field.Switch label={m.…()} />}</form.AppField>.
// It shows the field's errors once the user has left the switch, or the form has been submitted.
export function Switch({ label, description }: SwitchProps) {
  const field = useFieldContext<boolean>();
  const errors = useFieldErrors();

  return (
    <UiSwitch
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
    </UiSwitch>
  );
}
