import { z } from "zod/v4";

import { useCreateSession } from "@/api/gen/client/sessions/sessions.ts";
import { CreateSessionBody } from "@/api/gen/zod/sessions/sessions.zod.ts";
import { useAppForm } from "@/components/form/use-app-form.ts";
import { Alert } from "@/components/ui/alert";
import { isUnauthorized } from "@/features/session/utils/unauthorized.ts";
import { m } from "@/paraglide/messages.js";

// The generated schema takes any string; the API rejects an empty one, so the form says so first.
const schema = CreateSessionBody.extend({
  username: z.string().min(1, { error: () => m.usernameRequired() }),
  password: z.string().min(1, { error: () => m.passwordRequired() }),
});

// The form of the sign-in page: the username and the password, which create a session, whose
// cookie then signs every request in. onSignedIn follows a sign-in, as the page leads on. A wrong
// username or password empties the password field and says so above the fields, without telling
// which of the two was wrong, as the API doesn't either.
export function SignInForm({ onSignedIn }: { onSignedIn: () => void }) {
  const createSession = useCreateSession();
  const form = useAppForm({
    defaultValues: { username: "", password: "" },
    validators: { onChange: schema },
    onSubmit: async ({ value, formApi }) => {
      try {
        await createSession.mutateAsync({ data: value });
      } catch (error) {
        if (isUnauthorized(error)) {
          formApi.setFieldValue("password", "", { dontValidate: true });
        }
        return;
      }
      onSignedIn();
    },
  });

  return (
    <form.AppForm>
      <form.Form>
        {createSession.isError && (
          <Alert>
            {isUnauthorized(createSession.error) ? m.invalidCredentials() : m.signInFailed()}
          </Alert>
        )}
        <form.AppField name="username">
          {(field) => <field.TextField label={m.username()} autoComplete="username" />}
        </form.AppField>
        <form.AppField name="password">
          {(field) => (
            <field.TextField label={m.password()} type="password" autoComplete="current-password" />
          )}
        </form.AppField>
        <form.SubmitButton label={m.signIn()} />
      </form.Form>
    </form.AppForm>
  );
}
