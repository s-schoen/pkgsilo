import { Form as AriaForm } from "react-aria-components/Form";

import { useFormContext } from "@/components/form/form-contexts.ts";

import type { ReactNode } from "react";

import styles from "@/components/form/form.module.css";

// The <form> of a useAppForm form, which submits it through form.handleSubmit() and stacks its
// fields: render it with <form.AppForm><form.Form>{fields and form.SubmitButton}</form.Form>
// </form.AppForm>. Its fields validate with the form's schema (zod), never with the browser's own
// validation, which would block the submission: React Aria's validationBehavior is "aria".
export function Form({ children }: { children: ReactNode }) {
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
      className={styles.form}
    >
      {children}
    </AriaForm>
  );
}
