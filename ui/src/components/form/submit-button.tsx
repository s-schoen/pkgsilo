import { useFormContext } from "@/components/form/form-contexts.ts";
import { Button } from "@/components/ui/button";

// The submit button of a useAppForm form: render it with
// <form.AppForm><form.SubmitButton label={m.…()} /></form.AppForm>. It's pending, with a spinner,
// while the form submits, and disabled after a submission has failed validation until the user
// fixes the errors.
export function SubmitButton({ label }: { label: string }) {
  const form = useFormContext();

  return (
    <form.Subscribe selector={(state) => [state.canSubmit, state.isSubmitting] as const}>
      {([canSubmit, isSubmitting]) => (
        <Button type="submit" isDisabled={!canSubmit} isPending={isSubmitting}>
          {label}
        </Button>
      )}
    </form.Subscribe>
  );
}
