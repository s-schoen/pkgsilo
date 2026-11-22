# The production images, built from one multi-stage file: the default distroless image and a Debian
# trixie variant with a shell. Both share the Node and Go stages. The distroless stage is last, so that
# `docker build .` builds the default image and `--target trixie` the other one.
#
# The build stages run on the build platform and the Go compiler cross-compiles to the target, so a
# multi-platform build doesn't run Go or Node under QEMU. Only the Debian setup does.

# The web UI, which the Go binary embeds. Mirrors the dependencies of `task ui:build` (gen:api, i18n
# and routes) and its build.
FROM --platform=$BUILDPLATFORM node:26.13.1 AS ui
WORKDIR /src/ui
# Node 26 doesn't bundle pnpm, and the version is the one pinned in mise.toml.
RUN npm install --global --no-audit --no-fund pnpm@12.21.0
COPY ui/package.json ui/pnpm-lock.yaml ui/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY ui/ ./
# The OpenAPI document that `task ui:gen:api` generates the client from.
COPY app/testdata/openapi.json /src/app/testdata/openapi.json
RUN pnpm run gen:api && pnpm run i18n && pnpm run routes && pnpm run build

# The server: a static binary with the web UI embedded.
FROM --platform=$BUILDPLATFORM golang:1.27.4 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# ui/dist is ignored by .dockerignore, so it only comes from the UI stage.
COPY --from=ui /src/ui/dist ./ui/dist
ARG VERSION=dev
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -buildvcs=false -trimpath -tags webui \
    -ldflags "-s -w -X main.version=${VERSION}" -o /out/pkgsilo ./cmd/

# The migrate CLI for the schema, with only the Postgres driver: about 6 MB instead of 59 MB.
# Keep MIGRATE_VERSION in step with gomigrate in mise.toml; Renovate doesn't update ARG values.
FROM --platform=$BUILDPLATFORM golang:1.27.4 AS migrate
ARG MIGRATE_VERSION=4.20.1
ARG TARGETOS TARGETARCH
# A temporary module builds the CLI with go build, which cross-compiles and writes the binary where the
# runtime stage takes it; the CLI is not a dependency of pkgsilo.
RUN mkdir -p /tmp/migrate && cd /tmp/migrate \
    && go mod init migrate \
    && go get github.com/golang-migrate/migrate/v4/cmd/migrate@v${MIGRATE_VERSION} \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
       go build -tags postgres -ldflags "-s -w -X main.Version=v${MIGRATE_VERSION}" \
       -o /out/migrate github.com/golang-migrate/migrate/v4/cmd/migrate

# The image for users who want a shell in the container. Like the distroless image, it runs as
# distroless's nonroot, so files see the same owner in both.
FROM debian:trixie-slim AS trixie
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 65532 nonroot \
    && useradd --uid 65532 --gid 65532 --no-create-home --home-dir /nonexistent --shell /bin/bash nonroot
ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.title="pkgsilo" \
      org.opencontainers.image.description="Self-hosted APT and DNF repositories" \
      org.opencontainers.image.source="https://github.com/s-schoen/pkgsilo" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"
COPY --from=build /out/pkgsilo /usr/local/bin/pkgsilo
COPY --from=migrate /out/migrate /usr/local/bin/migrate
COPY migrations /migrations
USER 65532:65532
EXPOSE 8080 9090
# No HEALTHCHECK: the distroless image has no shell or curl; probe GET /api/v1/health instead.
ENTRYPOINT ["/usr/local/bin/pkgsilo"]

# The default image: no shell, no package manager, non-root.
FROM gcr.io/distroless/static-debian13:nonroot AS distroless
ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.title="pkgsilo" \
      org.opencontainers.image.description="Self-hosted APT and DNF repositories" \
      org.opencontainers.image.source="https://github.com/s-schoen/pkgsilo" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"
COPY --from=build /out/pkgsilo /usr/local/bin/pkgsilo
COPY --from=migrate /out/migrate /usr/local/bin/migrate
COPY migrations /migrations
USER 65532:65532
EXPOSE 8080 9090
ENTRYPOINT ["/usr/local/bin/pkgsilo"]
