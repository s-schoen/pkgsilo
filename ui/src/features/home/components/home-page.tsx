import { m } from "@/paraglide/messages.js";

// The start page, at /.
export function HomePage() {
  return <p>{m.welcome()}</p>;
}
