# chainguard-mcp

An MCP server that lets clients discover Chainguard container images.

## Tools

| Tool | Description |
|------|-------------|
| `list_images` | List the images in the Chainguard Images directory. Optional `query` (substring filter), `limit` (default 100, max 1000) and `offset`. |
| `get_image_tags` | List the tags for `cgr.dev/chainguard/<image>`. Works only for free-tier (publicly pullable) images. |

## Data sources

- **Image list:** the public directory sitemap (`https://images.chainguard.dev/sitemap.xml`), cached for one hour. The cgr.dev catalog API isn't available.
- **Tags:** the cgr.dev registry API with an anonymous pull token. Signature, attestation and SBOM tags (`sha256-*`) are filtered out.

## Run

```bash
go build -o chainguard-mcp .
./chainguard-mcp              # stdio (for Claude Desktop / Claude Code)
./chainguard-mcp -http :8080  # Streamable HTTP at http://localhost:8080/mcp
```

Add it to Claude Code:

```bash
claude mcp add chainguard -- /path/to/chainguard-mcp
```

## Docker

The image is built on Chainguard's `go` (build) and `static` (runtime) images, and runs as a non-root user.

```bash
docker build -t chainguard-mcp .
```

HTTP mode (the default). Bind the host port to `127.0.0.1` so it isn't reachable from your network:

```bash
docker run -d --rm -p 127.0.0.1:8080:8080 chainguard-mcp
```

Stdio mode. Passing an empty `-http=` disables HTTP:

```bash
claude mcp add chainguard -- docker run -i --rm chainguard-mcp -http=
```

Multi-arch build (cross-compiles, no emulation needed):

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t chainguard-mcp .
```

## Test

`go test ./...` runs an end-to-end test against the live endpoints. Use `-short` to skip it when offline.
