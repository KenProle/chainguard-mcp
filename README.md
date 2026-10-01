# chainguard-mcp

An MCP server that helps AI assistants discover, inspect and adopt [Chainguard](https://www.chainguard.dev/) container images.

## Tools

| Tool | Description |
|------|-------------|
| `list_images` | List images in the Chainguard directory, with free-tier status. Optional `query`, `free_only`, `limit` (default 100) and `offset`. |
| `get_image_tags` | List the tags for `cgr.dev/chainguard/<image>`. |
| `get_image_details` | Platforms and download sizes, digest, creation time, user, entrypoint, command, working directory and environment. |
| `pin_image` | Resolve a tag to its digest, e.g. `cgr.dev/chainguard/python:latest@sha256:…`, for reproducible builds. |
| `get_image_packages` | OS packages from the image's signed SBOM, with versions, licenses and source packages, plus whether it has a shell or `apk`. |
| `check_vulnerabilities` | Look up a CVE/GHSA ID against the image's packages, or summarize recorded fixes per package. |
| `find_alternative` | Suggest the Chainguard image to replace an upstream one, e.g. `node:20-alpine` → `cgr.dev/chainguard/node`. |

It also provides a `migrate_dockerfile` **prompt** that walks the assistant through converting a Dockerfile to Chainguard images using the tools above.

Every tool except `list_images` and `find_alternative` needs anonymous registry access, so it only works for **free-tier** images. Paid images return a clear error.

## Data sources

| Data | Source |
|------|--------|
| Image list | The public directory sitemap (`images.chainguard.dev/sitemap.xml`), cached for 1 hour. The cgr.dev catalog API isn't available. |
| Free-tier status | Whether cgr.dev issues an anonymous pull token, cached for 24 hours. |
| Tags, details, digests | The cgr.dev registry API, with an anonymous token. |
| Packages | The SPDX SBOM that Chainguard attaches to each image as a signed attestation (`sha256-<digest>.att`). |
| Vulnerabilities | The [Wolfi security database](https://packages.wolfi.dev/os/security.json), cached for 6 hours. |

The Wolfi database records which vulnerabilities each package version **fixes**, and which never applied. It doesn't list unfixed vulnerabilities, so `check_vulnerabilities` isn't a replacement for a scanner such as [grype](https://github.com/anchore/grype).

## Install

Download a binary for your platform from [Releases](https://github.com/KenProle/chainguard-mcp/releases), or build from source:

```bash
go build -o chainguard-mcp .
```

## Run

```bash
./chainguard-mcp                     # stdio (for Claude Desktop / Claude Code)
./chainguard-mcp -http 127.0.0.1:8080  # Streamable HTTP at /mcp, health check at /healthz
./chainguard-mcp -version
```

Add it to Claude Code:

```bash
claude mcp add -s user chainguard -- /path/to/chainguard-mcp
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

```bash
go test ./...
```

The tests run offline against a fake Chainguard registry (`fake_test.go`). To also run the end-to-end test against the live Chainguard endpoints:

```bash
CHAINGUARD_LIVE=1 go test -run TestLive -v ./...
```

## Releasing

CI (`.github/workflows/ci.yml`) runs formatting, vet and tests on Linux and Windows, builds the Docker image, and dry-runs the release on every push to `main`.

To publish a release, push a version tag:

```bash
git tag v0.2.0
git push origin v0.2.0
```

`.github/workflows/release.yml` then uses [GoReleaser](https://goreleaser.com) to build binaries for Linux, macOS and Windows (amd64 and arm64) and attach them to a GitHub Release with checksums.

## Project layout

| File | Contents |
|------|----------|
| `main.go` | Flags and transport setup (stdio or HTTP). |
| `tools.go` | MCP tool and prompt definitions. |
| `catalog.go` | Image list from the directory sitemap. |
| `registry.go` | Anonymous cgr.dev client: tokens, free-tier checks, tags, manifests, blobs. |
| `details.go` | `get_image_details` and `pin_image`. |
| `sbom.go` | SBOM attestation parsing for `get_image_packages`. |
| `vulns.go` | Wolfi security database and APK version comparison. |
| `alternatives.go` | Upstream image → Chainguard image mapping. |

## License

MIT. See [LICENSE](LICENSE).
