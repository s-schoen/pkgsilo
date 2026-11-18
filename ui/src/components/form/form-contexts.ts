import { createFormHookContexts } from "@tanstack/react-form";

// The contexts through which the components of use-app-form.ts reach their field and form: a field
// component reads its field with useFieldContext<Value>(), a form component its form with
// useFormContext().
export const { fieldContext, useFieldContext, formContext, useFormContext } =
  createFormHookContexts();
