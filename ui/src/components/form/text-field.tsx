import { useStore } from "@tanstack/react-form";

import { useFieldContext } from "@/components/form/form-contexts.ts";

type TextFieldProps = {
  label: string;
  type?: "text" | "email" | "password" | "search" | "url";
};

// A text input with its label, for a string field of useAppForm: render it with
// <form.AppField name="…">{(field) => <field.TextField label={m.…()} />}</form.AppField>.
// It shows the field's errors once the user has left the field, or the form has been submitted.
export function TextField({ label, type = "text" }: TextFieldProps) {
  const field = useFieldContext<string>();
  const submitted = useStore(field.form.store, (state) => state.submissionAttempts > 0);
  const { errors, isBlurred } = field.state.meta;
  const errorId = `${field.name}-error`;
  // Not while the user is still typing the value for the first time.
  const showErrors = (isBlurred || submitted) && errors.length > 0;

  return (
    <div>
      <label htmlFor={field.name}>{label}</label>
      <input
        id={field.name}
        name={field.name}
        type={type}
        value={field.state.value}
        onBlur={field.handleBlur}
        onChange={(event) => {
          field.handleChange(event.target.value);
        }}
        aria-invalid={showErrors}
        aria-describedby={showErrors ? errorId : undefined}
      />
      {showErrors && (
        <p id={errorId} role="alert">
          {errors.map(errorMessage).join(" ")}
        </p>
      )}
    </div>
  );
}

// A validator returns its errors as it likes: a schema's validator (such as zod's) an issue with a
// message, a function a string.
function errorMessage(error: unknown): string {
  if (typeof error === "string") {
    return error;
  }
  if (typeof error === "object" && error !== null && "message" in error) {
    return String(error.message);
  }
  return String(error);
}
