import { useEffect, useRef } from "react";
import { z } from "zod/v4";

import { useAppForm } from "@/components/form/use-app-form.ts";

type IdentityTextFieldProps = {
  // The field's value in identitiesCollection.
  value: string;
  // The rules of the value: a schema of identity-schema.ts.
  schema: z.ZodType<string, string>;
  label: string;
  // Saves the changed value, as useSaveIdentityField's function for this field.
  onSave: (value: string) => Promise<void>;
  placeholder?: string;
  variant?: "default" | "title";
};

// A field of an identity's page that the user edits in place: the form's InlineTextField, in a form
// of its own that holds the value while the user edits it and checks it with schema.
export function IdentityTextField({ value, schema, ...props }: IdentityTextFieldProps) {
  const form = useAppForm({
    defaultValues: { value },
    validators: { onChange: z.object({ value: schema }) },
  });

  // The value in the collection changes, from the user's own save and its rollback, or from another
  // tab or the list: the form takes the latest value while the user isn't editing, and once the
  // edit mode ends, so that neither a rollback nor another change replaces what the user types.
  const editing = useRef(false);
  const latest = useRef(value);
  useEffect(() => {
    latest.current = value;
    if (!editing.current) {
      form.setFieldValue("value", value);
    }
  }, [form, value]);

  return (
    <form.AppField name="value">
      {(field) => (
        <field.InlineTextField
          {...props}
          onEditingChange={(isEditing) => {
            editing.current = isEditing;
            if (!isEditing) {
              form.setFieldValue("value", latest.current);
            }
          }}
        />
      )}
    </form.AppField>
  );
}
