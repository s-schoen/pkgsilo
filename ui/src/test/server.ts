import { setupServer } from "msw/node";

// Mocks the API for the tests. It has no handlers of its own: a test registers the ones it needs with
// server.use(), and setup.ts removes them after each test. A request that no handler takes rejects
// with an MSW error (onUnhandledFrame in setup.ts).
export const server = setupServer();
