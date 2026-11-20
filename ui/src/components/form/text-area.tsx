import { useFieldContext } from "@/components/form/form-contexts.ts";
import { useFieldErrors } from "@/components/form/use-field-errors.ts";
import { TextArea as UiTextArea } from "@/components/ui/text-field";

type TextAreaProps = {
  label: string;
  description?: string;
  placeholder?: string;
  rows?: number;
};

// The TextArea of the UI library, bound to a string field of useAppForm: render it with
// <form.AppField name="…">{(field) => <field.TextArea label={m.…()} />}</form.AppField>.
// It shows the field's errors once the user has left the field, or the form has been submitted.
export function TextArea(props: TextAreaProps) {
  const field = useFieldContext<string>();
  const errors = useFieldErrors();

  return (
    <UiTextArea
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
