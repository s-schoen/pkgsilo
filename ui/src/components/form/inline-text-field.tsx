import { useFieldContext } from "@/components/form/form-contexts.ts";
import { joinErrors } from "@/components/form/use-field-errors.ts";
import { InlineTextField as UiInlineTextField } from "@/components/ui/inline-edit";

type InlineTextFieldProps = {
  label: string;
  // Saves the field's changed value, as the UI library's InlineEdit: reject with InlineEditError to
  // show a message on the field, such as a taken username; any other rejection reverts the value,
  // so show its toast before.
  onSave: (value: string) => Promise<void>;
  // Called when the edit mode starts and ends, as the UI library's InlineEdit's.
  onEditingChange?: (isEditing: boolean) => void;
  placeholder?: string;
  // Renders the value as plain text, without the edit mode, as for a value the actor may not change.
  isReadOnly?: boolean;
  variant?: "default" | "title";
};

// The InlineTextField of the UI library, bound to a string field of useAppForm, whose validators
// (a zod schema) check it: render it with
// <form.AppField name="…">{(field) => <field.InlineTextField label={m.…()} onSave={…} />}</form.AppField>.
// The field holds the value while the user edits it, and the value before the edit once the user
// cancels or the save fails; the form is never submitted, since each field saves on its own.
export function InlineTextField(props: InlineTextFieldProps) {
  const field = useFieldContext<string>();
  const { errors } = field.state.meta;

  return (
    <UiInlineTextField
      {...props}
      value={field.state.value}
      onChange={field.handleChange}
      // The edit shows the error once the user tries to save, so it needs it at once.
      errorMessage={errors.length > 0 ? joinErrors(errors) : undefined}
    />
  );
}
