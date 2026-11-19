import { z } from "zod/v4";

// zod compiles a faster parser for each object schema with new Function, which the UI's
// Content-Security-Policy (core/spa) forbids: zod catches the error, but the browser still reports
// a violation. jitless keeps zod from trying. zod reads the setting when it creates a schema, such
// as one of src/api/gen/zod on import, so main.tsx imports this module before any other.
z.config({ jitless: true });
