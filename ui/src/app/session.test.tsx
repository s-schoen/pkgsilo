import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import { describe, expect, it } from "vitest";

import {
  getCreateSessionMockHandler,
  getDeleteCurrentSessionMockHandler,
} from "@/api/gen/client/sessions/sessions.msw.ts";
import { m } from "@/paraglide/messages.js";
import { renderRoute } from "@/test/render.tsx";
import { server } from "@/test/server.ts";
import { signedIn, signedOut, testIdentity, unauthorized } from "@/test/session.ts";

// Signs in on the sign-in page that is shown, as a user would.
async function signIn(user: ReturnType<typeof userEvent.setup>) {
  await user.type(await screen.findByLabelText(m.username()), "ada");
  await user.type(screen.getByLabelText(m.password()), "correct horse");
  await user.click(screen.getByRole("button", { name: m.signIn() }));
}

// The sign-in, the guard of the pages that need a session, and the sign-out, through the whole app.
describe("session", () => {
  it("leads a user who isn't signed in to the sign-in page, with the page as the redirect", async () => {
    server.use(signedOut());
    const { router } = renderRoute("/does-not-exist?tab=1");

    expect(
      await screen.findByRole("heading", { level: 1, name: m.signInTitle() }),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/signin");
    expect(router.state.location.search).toEqual({ redirect: "/does-not-exist?tab=1" });
    expect(screen.queryByRole("navigation")).not.toBeInTheDocument();
  });

  it("leads back to the redirect after a sign-in", async () => {
    const user = userEvent.setup();
    // Not signed in until the sign-in succeeds.
    let session = false;
    server.use(
      http.get("/api/v1/sessions/current", () => (session ? undefined : unauthorized())),
      getCreateSessionMockHandler(() => {
        session = true;
        return { data: { id: "00000000-0000-4000-8000-000000000001" } };
      }),
      ...signedIn(),
    );
    const { router } = renderRoute("/signin?redirect=%2Fdoes-not-exist");

    await signIn(user);

    expect(
      await screen.findByRole("heading", { level: 1, name: m.notFound() }),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/does-not-exist");
  });

  it("leads to the start page for a redirect to another host", async () => {
    const user = userEvent.setup();
    let session = false;
    server.use(
      http.get("/api/v1/sessions/current", () => (session ? undefined : unauthorized())),
      getCreateSessionMockHandler(() => {
        session = true;
        return { data: { id: "00000000-0000-4000-8000-000000000001" } };
      }),
      ...signedIn(),
    );
    const { router } = renderRoute("/signin?redirect=%2F%2Fevil.example%2F");

    await signIn(user);

    expect(await screen.findByText(m.welcome())).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
  });

  it("says that the username or password is wrong, and empties the password", async () => {
    const user = userEvent.setup();
    server.use(signedOut(), http.post("/api/v1/sessions", unauthorized));
    const { router } = renderRoute("/signin");

    await signIn(user);

    expect(await screen.findByRole("alert")).toHaveTextContent(m.invalidCredentials());
    expect(screen.getByLabelText(m.password())).toHaveValue("");
    expect(screen.getByLabelText(m.username())).toHaveValue("ada");
    expect(router.state.location.pathname).toBe("/signin");
  });

  it("leads a user who is signed in from the sign-in page to the redirect", async () => {
    server.use(...signedIn());
    const { router } = renderRoute("/signin?redirect=%2Fdoes-not-exist");

    expect(
      await screen.findByRole("heading", { level: 1, name: m.notFound() }),
    ).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/does-not-exist");
  });

  it("shows the identity that is signed in", async () => {
    const user = userEvent.setup();
    server.use(...signedIn());
    renderRoute("/");

    const account = await screen.findByRole("button", { name: testIdentity.displayName });
    await user.click(account);

    const menu = await screen.findByRole("menu");
    expect(
      within(menu).getByText(m.signedInAs({ username: testIdentity.username })),
    ).toBeInTheDocument();
  });

  it("signs out, and leads to the sign-in page", async () => {
    const user = userEvent.setup();
    let deleted = false;
    server.use(
      http.get("/api/v1/sessions/current", () => (deleted ? unauthorized() : undefined)),
      getDeleteCurrentSessionMockHandler(() => {
        deleted = true;
      }),
      ...signedIn(),
    );
    const { router } = renderRoute("/");

    await user.click(await screen.findByRole("button", { name: testIdentity.displayName }));
    await user.click(await screen.findByRole("menuitem", { name: m.signOut() }));

    expect(
      await screen.findByRole("heading", { level: 1, name: m.signInTitle() }),
    ).toBeInTheDocument();
    expect(deleted).toBe(true);
    expect(router.state.location.pathname).toBe("/signin");
    await waitFor(() => {
      expect(router.options.context.queryClient.getQueryCache().getAll()).toEqual([]);
    });
  });

  it("leads to the sign-in page once the session ends on a page", async () => {
    server.use(...signedIn());
    const { router } = renderRoute("/does-not-exist");
    await screen.findByRole("heading", { level: 1, name: m.notFound() });

    // The session ends, as after a sign-out in another tab: the next request answers 401.
    server.use(signedOut());
    await router.options.context.queryClient.refetchQueries();

    expect(
      await screen.findByRole("heading", { level: 1, name: m.signInTitle() }),
    ).toBeInTheDocument();
    expect(router.state.location.search).toEqual({ redirect: "/does-not-exist" });
  });
});
