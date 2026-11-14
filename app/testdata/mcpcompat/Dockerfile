# The client of TestMCP_Compatibility: the official TypeScript MCP SDK at the version of the lockfile, on a
# pinned Node. The packages are installed when the image is built, so that running the test needs no network.
FROM node:24.15.0-bookworm-slim

WORKDIR /compat
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund
COPY compat.mjs ./

USER node
ENTRYPOINT ["node", "compat.mjs"]
