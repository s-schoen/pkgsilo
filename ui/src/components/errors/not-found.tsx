import { Link } from "@/components/ui/link";
import { m } from "@/paraglide/messages.js";

// The page for a path that no route takes.
export function NotFound() {
  return (
    <>
      <p>{m.notFound()}</p>
      <Link href="/">{m.goHome()}</Link>
    </>
  );
}
