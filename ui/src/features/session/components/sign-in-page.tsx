import { useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";

import { getGetCurrentSessionQueryKey } from "@/api/gen/client/sessions/sessions.ts";
import { SignInForm } from "@/features/session/components/sign-in-form.tsx";
import { safeRedirectTarget } from "@/features/session/utils/redirect-target.ts";
import { m } from "@/paraglide/messages.js";

import styles from "@/features/session/components/sign-in-page.module.css";

// The sign-in page, at /signin: a card on the canvas with the form, outside the app's frame. A
// sign-in leads to the page of the redirect search parameter, where the user was sent from, or to
// the start page.
export function SignInPage() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { redirect } = useSearch({ from: "/signin" });

  return (
    <main className={styles.page}>
      <div className={styles.card}>
        <p className={styles.brand}>pkgsilo</p>
        <h1 className={styles.title}>{m.signInTitle()}</h1>
        <SignInForm
          onSignedIn={() => {
            // The check of the session before the sign-in failed: the guard of the next page
            // fetches it again.
            queryClient.removeQueries({ queryKey: getGetCurrentSessionQueryKey() });
            void navigate({ href: safeRedirectTarget(redirect), replace: true });
          }}
        />
      </div>
    </main>
  );
}
