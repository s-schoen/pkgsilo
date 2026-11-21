import { HttpResponse, delay, http } from "msw";
import { expect, fn, waitFor } from "storybook/test";

import preview from "#.storybook/preview";
import { getCreateSessionMockHandler } from "@/api/gen/client/sessions/sessions.msw.ts";
import { SignInForm } from "@/features/session/components/sign-in-form.tsx";
import { m } from "@/paraglide/messages.js";

// The credentials that the stories type.
const credentials = { username: "ada", password: "correct horse" };

// The API's answer to a login that failed with status.
function problem(status: number, title: string) {
  return HttpResponse.json(
    { type: "about:blank", title, status },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

const meta = preview.meta({
  component: SignInForm,
  args: { onSignedIn: fn() },
  decorators: [
    (Story) => (
      <div style={{ maxInlineSize: "22rem" }}>
        <Story />
      </div>
    ),
  ],
});

// The user signs in: the form creates the session, and onSignedIn leads on.
export const SignedIn = meta.story({
  beforeEach: ({ msw }) => {
    msw.use(getCreateSessionMockHandler({ data: { id: "00000000-0000-4000-8000-000000000001" } }));
  },
  play: async ({ args, canvas, userEvent }) => {
    await expect(canvas.getByLabelText(m.username())).toHaveAttribute("autocomplete", "username");
    await expect(canvas.getByLabelText(m.password())).toHaveAttribute(
      "autocomplete",
      "current-password",
    );

    await userEvent.type(canvas.getByLabelText(m.username()), credentials.username);
    await userEvent.type(canvas.getByLabelText(m.password()), `${credentials.password}{Enter}`);

    await waitFor(async () => {
      await expect(args.onSignedIn).toHaveBeenCalledOnce();
    });
    await expect(canvas.queryByRole("alert")).not.toBeInTheDocument();
  },
});

// Both fields are required: a submission without them says so below each, and sends nothing.
export const Empty = meta.story({
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: m.signIn() }));

    await expect(canvas.getByLabelText(m.username())).toHaveAccessibleDescription(
      m.usernameRequired(),
    );
    await expect(canvas.getByLabelText(m.password())).toHaveAccessibleDescription(
      m.passwordRequired(),
    );
    await expect(args.onSignedIn).not.toHaveBeenCalled();
  },
});

// A wrong username or password: the alert says so, without telling which, and the password field
// is empty again.
export const InvalidCredentials = meta.story({
  beforeEach: ({ msw }) => {
    msw.use(http.post("/api/v1/sessions", () => problem(401, "Unauthorized")));
  },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.type(canvas.getByLabelText(m.username()), credentials.username);
    await userEvent.type(canvas.getByLabelText(m.password()), credentials.password);
    await userEvent.click(canvas.getByRole("button", { name: m.signIn() }));

    await expect(await canvas.findByRole("alert")).toHaveTextContent(m.invalidCredentials());
    await expect(canvas.getByLabelText(m.password())).toHaveValue("");
    await expect(canvas.getByLabelText(m.username())).toHaveValue(credentials.username);
    await expect(args.onSignedIn).not.toHaveBeenCalled();
  },
});

// Any other failure, such as the server's: the alert asks to try again, and the fields stay.
export const Failed = meta.story({
  beforeEach: ({ msw }) => {
    msw.use(http.post("/api/v1/sessions", () => problem(500, "Internal Server Error")));
  },
  play: async ({ args, canvas, userEvent }) => {
    await userEvent.type(canvas.getByLabelText(m.username()), credentials.username);
    await userEvent.type(canvas.getByLabelText(m.password()), credentials.password);
    await userEvent.click(canvas.getByRole("button", { name: m.signIn() }));

    await expect(await canvas.findByRole("alert")).toHaveTextContent(m.signInFailed());
    await expect(canvas.getByLabelText(m.password())).toHaveValue(credentials.password);
    await expect(args.onSignedIn).not.toHaveBeenCalled();
  },
});

// While the API answers, the button is pending.
export const Pending = meta.story({
  beforeEach: ({ msw }) => {
    msw.use(
      http.post("/api/v1/sessions", async () => {
        await delay("infinite");
        return new HttpResponse(null);
      }),
    );
  },
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(canvas.getByLabelText(m.username()), credentials.username);
    await userEvent.type(canvas.getByLabelText(m.password()), credentials.password);
    await userEvent.click(canvas.getByRole("button", { name: m.signIn() }));

    await waitFor(async () => {
      await expect(canvas.getByRole("button", { name: m.signIn() })).toHaveAttribute(
        "data-pending",
      );
    });
  },
});

// The form's texts in German.
export const German = meta.story({
  globals: { locale: "de" },
  play: async ({ canvas }) => {
    await expect(canvas.getByLabelText(m.username({}, { locale: "de" }))).toBeVisible();
    await expect(canvas.getByLabelText(m.password({}, { locale: "de" }))).toBeVisible();
    await expect(
      canvas.getByRole("button", { name: m.signIn({}, { locale: "de" }) }),
    ).toBeVisible();
  },
});
