import { Form as AriaForm } from "react-aria-components/Form";

import { useFormContext } from "@/components/form/form-contexts.ts";

import type { ReactNode } from "react";

import styles from "@/components/form/form.module.css";

// The <form> of a useAppForm form, which submits it through form.handleSubmit() and stacks its
// fields: render it with <form.AppForm><form.Form>{fields and form.SubmitButton}</form.Form>
// </form.AppForm>. Its fields validate with the form's schema (zod), never with the browser's own
// validation, which would block the submission: React Aria's validationBehavior is "aria".
// layout="page" lays out nothing, for a form that holds a whole page, as a create page whose
// PageHeader holds the submit button and whose DetailLayout holds the fields.
export function Form({
  children,
  layout = "stack",
}: {
  children: ReactNode;
  layout?: "stack" | "page";
}) {
  const form = useFormContext();

  return (
    <AriaForm
      validationBehavior="aria"
      onSubmit={(event) => {
        event.preventDefault();
        // A form inside another doesn't submit the outer one.
        event.stopPropagation();
        void form.handleSubmit();
      }}
      className={layout === "page" ? styles.page : styles.form}
    >
      {children}
    </AriaForm>
  );
}
