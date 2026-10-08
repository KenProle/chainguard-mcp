# UI stage: build the React app with Chainguard's Node image. The output is
# plain static files, so it's built once on the build machine's platform.
FROM --platform=$BUILDPLATFORM cgr.dev/chainguard/node:latest-dev AS web
WORKDIR /app
COPY --chown=node:node web/package.json web/package-lock.json ./
RUN npm ci --no-fund --no-audit
COPY --chown=node:node web/ ./
RUN npm run build

# Go build stage: Chainguard's Go toolchain image. It runs on the build
# machine's native platform and cross-compiles for the target, so multi-arch
# builds (docker buildx --platform linux/amd64,linux/arm64) don't need emulation.
FROM --platform=$BUILDPLATFORM cgr.dev/chainguard/go:latest AS build
ARG TARGETOS TARGETARCH
WORKDIR /src

# Download dependencies first so this layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
# The binary embeds the built UI from web/dist (see webui.go).
COPY --from=web /app/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/chainguard-mcp .

# Runtime stage: minimal image with CA certificates, no shell or package
# manager, running as a non-root user.
FROM cgr.dev/chainguard/static:latest
COPY --from=build /out/chainguard-mcp /chainguard-mcp

EXPOSE 8080
ENTRYPOINT ["/chainguard-mcp"]
# Serve the web UI and Streamable HTTP MCP by default. Inside a container the
# server must listen on all interfaces; restrict exposure with the host port
# mapping instead.
CMD ["-http", ":8080"]
