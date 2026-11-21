import { m } from "@/paraglide/messages.js";

// The start page, at /.
export function HomePage() {
  return <h1>{m.welcome()}</h1>;
}
