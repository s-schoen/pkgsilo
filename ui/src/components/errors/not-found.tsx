import { Link } from "@/components/ui/link";
import { m } from "@/paraglide/messages.js";

// The page for a path that no route takes.
export function NotFound() {
  return (
    <>
      <h1>{m.notFound()}</h1>
      <Link href="/">{m.goHome()}</Link>
    </>
  );
}
