import { Link } from "@tanstack/react-router";

import { m } from "@/paraglide/messages.js";

// The page for a path that no route takes.
export function NotFound() {
  return (
    <>
      <p>{m.notFound()}</p>
      <Link to="/">{m.goHome()}</Link>
    </>
  );
}
