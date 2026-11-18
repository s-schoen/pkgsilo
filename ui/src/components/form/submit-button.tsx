import { useFormContext } from "@/components/form/form-contexts.ts";

// The submit button of a useAppForm form: render it with
// <form.AppForm><form.SubmitButton label={m.…()} /></form.AppForm>. It's disabled while the form is
// submitting, and after a submission has failed validation until the user fixes the errors.
export function SubmitButton({ label }: { label: string }) {
  const form = useFormContext();

  return (
    <form.Subscribe selector={(state) => [state.canSubmit, state.isSubmitting] as const}>
      {([canSubmit, isSubmitting]) => (
        <button type="submit" disabled={!canSubmit || isSubmitting}>
          {label}
        </button>
      )}
    </form.Subscribe>
  );
}
