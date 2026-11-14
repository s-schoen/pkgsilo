// Drives the MCP endpoint of pkgsilo with the official TypeScript SDK and prints what it saw as one JSON
// object, which TestMCP_Compatibility in app compares against what it expects.
//
// Environment: MCP_URL is the endpoint. ADMIN_KEY is the API key of admin, HIDDEN_KEY that of an identity
// whose policies hide its own username, and DENIED_KEY that of an identity whose policies deny it reading
// itself.
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';

const url = new URL(process.env.MCP_URL);

// The test opens the tunnel to the app once the container is ready, which it learns from this line.
process.stderr.write('waiting for pkgsilo\n');
await waitForApp();

// waitForApp returns once the app answers at all, or throws after 30 seconds.
async function waitForApp() {
  const deadline = Date.now() + 30_000;
  for (;;) {
    try {
      await fetch(url, { method: 'POST' });
      return;
    } catch (err) {
      if (Date.now() > deadline) {
        throw err;
      }
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
  }
}

// connect returns a client connected with the given API key, or with none if key is undefined.
async function connect(key) {
  const headers = key === undefined ? {} : { 'X-API-Key': key };
  const transport = new StreamableHTTPClientTransport(url, { requestInit: { headers } });
  const client = new Client({ name: 'pkgsilo-mcp-compat', version: '1.0.0' });
  await client.connect(transport);
  return { client, transport };
}

// call calls the tool with the given name as the identity of key, and returns the parts of its result that
// the test compares.
async function call(key, name, args = {}) {
  const { client } = await connect(key);
  try {
    const result = await client.callTool({ name, arguments: args });
    return {
      isError: result.isError ?? false,
      text: result.content.map((c) => c.text).join(''),
      structuredContent: result.structuredContent ?? null,
    };
  } finally {
    await client.close();
  }
}

const out = {};

const { client, transport } = await connect(process.env.ADMIN_KEY);
out.initialize = {
  serverName: client.getServerVersion()?.name,
  protocolVersion: transport.protocolVersion,
  capabilities: client.getServerCapabilities(),
};
const { tools } = await client.listTools();
out.tools = tools.map((t) => ({ name: t.name, annotations: t.annotations }));
await client.close();

out.whoami = await call(process.env.ADMIN_KEY, 'identity_whoami');
out.toolError = await call(process.env.DENIED_KEY, 'identity_whoami');
out.hidden = await call(process.env.HIDDEN_KEY, 'identity_whoami');

try {
  await connect(undefined);
  out.unauthorized = { connected: true };
} catch (err) {
  out.unauthorized = { name: err.constructor.name, code: err.code };
}

process.stdout.write(JSON.stringify(out) + '\n');
