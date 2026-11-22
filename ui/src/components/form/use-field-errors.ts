import { useStore } from "@tanstack/react-form";

import { useFieldContext } from "@/components/form/form-contexts.ts";

// The errors of the field of useAppForm that the calling component renders, as a field of the UI
// library takes them: isInvalid, and the messages in errorMessage. They show once the user has left
// the field or submitted the form, not while the user is still typing the value for the first time.
export function useFieldErrors(): { isInvalid: boolean; errorMessage: string | undefined } {
  const field = useFieldContext<unknown>();
  const submitted = useStore(field.form.store, (state) => state.submissionAttempts > 0);
  const { errors, isBlurred } = field.state.meta;
  const isInvalid = (isBlurred || submitted) && errors.length > 0;

  return {
    isInvalid,
    errorMessage: isInvalid ? joinErrors(errors) : undefined,
  };
}

// The messages of a field's errors, as one text. Validators of several events (onChange, onSubmit)
// may report the same error.
export function joinErrors(errors: ReadonlyArray<unknown>): string {
  return [...new Set(errors.map(errorMessage))].join(" ");
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
