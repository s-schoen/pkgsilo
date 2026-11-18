import { createFormHook } from "@tanstack/react-form";

import { fieldContext, formContext } from "@/components/form/form-contexts.ts";
import { SubmitButton } from "@/components/form/submit-button.tsx";
import { TextField } from "@/components/form/text-field.tsx";

// The form hook of the UI: useAppForm takes the options of TanStack Form's useForm, and its form
// renders the components below bound to a field (form.AppField) or to the form (form.AppForm).
// withForm splits a form into components. Register a new field or form component here.
export const { useAppForm, withForm } = createFormHook({
  fieldContext,
  formContext,
  fieldComponents: { TextField },
  formComponents: { SubmitButton },
});
