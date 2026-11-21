import { useFieldContext } from "@/components/form/form-contexts.ts";
import { useFieldErrors } from "@/components/form/use-field-errors.ts";
import { TextField as UiTextField } from "@/components/ui/text-field";

type TextFieldProps = {
  label: string;
  description?: string;
  placeholder?: string;
  type?: "text" | "email" | "password" | "search" | "url" | "tel";
  // What the browser and a password manager may fill in, as HTML's autocomplete: "username" and
  // "current-password" for a login.
  autoComplete?: string;
};

// The TextField of the UI library, bound to a string field of useAppForm: render it with
// <form.AppField name="…">{(field) => <field.TextField label={m.…()} />}</form.AppField>.
// It shows the field's errors once the user has left the field, or the form has been submitted.
export function TextField(props: TextFieldProps) {
  const field = useFieldContext<string>();
  const errors = useFieldErrors();

  return (
    <UiTextField
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
