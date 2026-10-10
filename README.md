# chainguard-mcp

An MCP server that helps AI assistants discover, inspect and adopt [Chainguard](https://www.chainguard.dev/) container images, plus a web UI for browsing the same data yourself.

This is an unofficial project, not affiliated with Chainguard.

## Web UI

Run the server in HTTP mode and open http://127.0.0.1:8080/:

```bash
./chainguard-mcp -http 127.0.0.1:8080
```

- **Catalog:** search all images, filter to free ones, and see which need a subscription. On the List view, ticking **Free only** first checks every image's free-tier status in batches of 250 and shows a progress bar with the time left; the first check takes about 30 seconds, then the results are cached for 24 hours and the bar no longer appears. The **Map** view draws the whole catalog as a treemap with one square per image, green for free and blue-gray for subscription. Group it by **variant** (base images, FIPS, IAM-guarded, IAM-guarded FIPS) or by **name prefix** (`crossplane`, `prometheus`, …); each group has a header with its image count, and groups too small to name are merged into Other. Inside each group, images that share a name prefix sit together in a block (`crossplane` has 220 base images), separated by a little white space. Few images are free: 59 of 3,166 on 2026-10-09, all but one of them base images. The map appears as soon as the catalog loads, and the colors fill in as free-tier status is checked in batches of 1,000 images; the first check takes about 30 seconds, then results are cached for 24 hours. Hover a square to see its name, status and group, or click it to open the image. Click a group's header (or Tab to it and press Enter) to zoom into that group: its name-prefix blocks fill the map, each labeled with its image count, and the legend and family table narrow to the group. A breadcrumb ("All groups › FIPS"), Escape or the browser's Back button zooms out, and the zoom is part of the URL (`?view=map&zoom=FIPS`), so a zoomed map can be shared. A sortable table of image families below the map (an image with its variants, such as `nginx` with `nginx-fips`) lists every image as a link, for keyboard and screen reader users. On phones a bar per group replaces the treemap, above the same table; tap a bar to zoom in, which shows the group's ten largest name-prefix blocks as bars.
- **Image details:** the pinned reference to use in a Dockerfile, the user it runs as, entrypoint, platforms and download sizes.
- **Packages & SBOM:** every OS package from the image's signed SBOM, whether it has a shell or package manager, and a download of the full SPDX SBOM. A license breakdown above the table counts packages as Permissive, Weak copyleft, Strong copyleft, Not declared or Unrecognized (an identifier outside its table); click a category to filter the table to it. Compound licenses count once: `AND` takes the most restrictive part, `OR` the least restrictive, and a `WITH` exception counts as its base license. It reflects the licenses declared in the SBOM, is informational only and isn't legal advice.
- **Security:** recorded vulnerability fixes per source package, and a CVE/GHSA lookup. A chart gives each source package with records a stacked bar of fixes included, vulnerabilities recorded as never affecting it, and fixes that a newer package version has but the image lacks. Those missing fixes are highlighted, their packages sort first, and their IDs are listed. Packages without records get no bar; the chart counts them, and the table below lists every package. The data comes from the Wolfi security database, which records fixes, not open vulnerabilities, so it isn't a full scan.
- **Compare:** two tags of the same image side by side, by default the minimal `latest` and the development `latest-dev`. Shows how much larger one is, each tag's package count, download size, shell and `apk`, a size chart, and the packages only in one tag or at different versions. Pick any two tags and amd64 or arm64; the comparison is kept in the URL, so you can share or reload it.
- **Find alternative:** enter an image you use today, like `node:20-alpine`, to get its Chainguard replacement and migration notes.

Sizes are compressed download sizes in decimal units (1 MB = 1,000,000 bytes), as Docker shows them.

The UI is built into the same binary and calls the same code as the MCP tools, through a JSON API under `/api/`. The comparison, the license breakdown and the fixes chart are computed in the browser from that API's responses. The catalog map's groups come from `GET /api/groups` (`group_by=variant` or `prefix`, and an optional `query`) and its families from `GET /api/families`; both use the same variant rules as `find_alternative`. They're the only endpoints without a matching MCP tool, since `find_alternative` already returns any image's variants.

## Tools

| Tool | Description |
|------|-------------|
| `list_images` | List images in the Chainguard directory, with free-tier status. Optional `query`, `free_only`, `limit` (default 100) and `offset`. |
| `get_image_tags` | List the tags for `cgr.dev/chainguard/<image>`. |
| `get_image_details` | Platforms and download sizes, digest, creation time, user, entrypoint, command, working directory and environment. |
| `pin_image` | Resolve a tag to its digest, e.g. `cgr.dev/chainguard/python:latest@sha256:…`, for reproducible builds. |
| `get_image_packages` | OS packages from the image's signed SBOM, with versions, licenses and source packages, plus whether it has a shell or `apk`. |
| `save_sbom` | Save the image's full SPDX SBOM as a JSON file, for compliance or scanning tools. See [Saving SBOMs](#saving-sboms). |
| `check_vulnerabilities` | Look up a CVE/GHSA ID against the image's packages, or summarize recorded fixes per package. |
| `find_alternative` | Suggest the Chainguard image to replace an upstream one, e.g. `node:20-alpine` → `cgr.dev/chainguard/node`. |

It also provides a `migrate_dockerfile` **prompt** that walks the assistant through converting a Dockerfile to Chainguard images using the tools above.

Every tool except `list_images` and `find_alternative` needs anonymous registry access, so it only works for **free-tier** images. Paid images return a clear error.

## Example questions

Once the server is connected, ask Claude questions like these. Claude picks the right tools, and often chains several together.

### A quick demo

Each question builds on the one before:

1. "What free Chainguard images are there for Python?"
2. "Tell me about the Chainguard python image. Does it run as root? What architectures does it support?"
3. "Does the python image have a shell? What about latest-dev?"
4. "Is the python image affected by CVE-2022-3602?"
5. "I'm using node:20-alpine. What's the Chainguard equivalent?"
6. Paste a Dockerfile and ask Claude to migrate it:

   ````
   Migrate this Dockerfile to Chainguard images and pin them by digest:

   ```dockerfile
   FROM python:3.12-slim
   WORKDIR /app
   COPY requirements.txt .
   RUN pip install -r requirements.txt
   COPY . .
   CMD ["python", "app.py"]
   ```
   ````

Claude Code also turns the built-in migration prompt into a slash command: type `/mcp__chainguard__migrate_dockerfile` followed by your Dockerfile.

### Discovery

- "How many Chainguard images are there in total?"
- "List all the FIPS images for Java."
- "Which Chainguard images exist for PostgreSQL?"

### Details and pinning

- "How big is the Chainguard static image to download?"
- "What environment variables does cgr.dev/chainguard/node set?"
- "Give me a digest-pinned FROM line for the Chainguard go image."

### Packages and SBOMs

- "Show me the SBOM for the Chainguard python image."
- "Give me the SBOM for the arm64 version of the Chainguard node image as a table."
- "Save the full SBOM for the Chainguard python image."
- "What version of OpenSSL is in the Chainguard python image?"
- "List the licenses of every package in the Chainguard node image."
- "Compare the packages in python:latest and python:latest-dev."

### Vulnerabilities

- "Summarize the security fixes recorded for the Chainguard python image."
- "Are there any fixes the python image is missing?"
- "Is the Chainguard node image affected by CVE-2024-12797?"

### Alternatives

- "What should I use instead of gcr.io/distroless/static-debian12?"
- "We use eclipse-temurin:21-jre and mcr.microsoft.com/dotnet/aspnet:8.0. What are the Chainguard replacements, and are they free?"
- "Replace ubuntu:24.04 with a Chainguard base image."

## Data sources

| Data | Source |
|------|--------|
| Image list | The public directory sitemap (`images.chainguard.dev/sitemap.xml`), cached for 1 hour. The cgr.dev catalog API isn't available. |
| Free-tier status | Whether cgr.dev issues an anonymous pull token, cached for 24 hours. |
| Tags, details, digests | The cgr.dev registry API, with an anonymous token. |
| Packages | The SPDX SBOM that Chainguard attaches to each image as a signed attestation (`sha256-<digest>.att`). |
| Vulnerabilities | The [Wolfi security database](https://packages.wolfi.dev/os/security.json), cached for 6 hours. |

The Wolfi database records which vulnerabilities each package version **fixes**, and which never applied. It doesn't list unfixed vulnerabilities, so `check_vulnerabilities` isn't a replacement for a scanner such as [grype](https://github.com/anchore/grype).

## Catalog history

Chainguard's endpoints only show the catalog as it is today, so a scheduled GitHub Action ([`history.yml`](.github/workflows/history.yml)) records it daily. Each day at about 06:17 UTC it appends one line to `catalog-history.jsonl` on the [`data` branch](https://github.com/KenProle/chainguard-mcp/tree/data) (not `main`, whose rules require pull requests): the date, how many images there are and how many are free, and which images were added, retired, or moved between free and subscription since the last line. The first 17 monthly lines (April 2024 to October 2026) were rebuilt from the Internet Archive's copies of the sitemap and have image names only. Free-tier status starts with the first live snapshot, so nothing is known about it before then. The file format is described in the `data` branch's README.

The binary does this itself, with two flags that run and exit without starting the server:

```bash
go run . -record-history catalog-history.jsonl     # take today's snapshot (once per UTC day; about 30 seconds)
go run . -backfill-history catalog-history.jsonl   # one-time: build an empty file from the Internet Archive
```

A snapshot is refused, and the job fails, if the sitemap is empty or would remove more than 5% of the previous snapshot's images. Nothing in the server or UI reads the history yet.

## Install

Download a binary for your platform from [Releases](https://github.com/KenProle/chainguard-mcp/releases), or build from source:

```bash
npm --prefix web ci
npm --prefix web run build   # optional: builds the web UI, which the Go binary embeds
go build -o chainguard-mcp .
```

Without the UI build, everything else still works and `/` shows how to build it.

## Run

```bash
./chainguard-mcp                     # stdio (for Claude Desktop / Claude Code)
./chainguard-mcp -http 127.0.0.1:8080  # web UI at /, MCP at /mcp, JSON API under /api/ (GET /api/ lists the endpoints), health check at /healthz
./chainguard-mcp -version
```

Add it to Claude Code:

```bash
claude mcp add -s user chainguard -- /path/to/chainguard-mcp
```

### JSON API

With `-http`, the API that the web UI uses is served under `/api/`. Every endpoint answers `GET` with JSON (the SBOM as an `application/spdx+json` download), and `GET /api/` lists them with their parameters:

```bash
curl http://127.0.0.1:8080/api/
```

| Endpoint | Parameters | Example |
| --- | --- | --- |
| `/api/images` | `query`, `free_only=true`, `limit` (default 100, max 1000), `offset` | `/api/images?query=python&free_only=true` |
| `/api/groups` | `group_by` (`variant` or `prefix`), `query` | `/api/groups?group_by=prefix&query=nginx` |
| `/api/families` | `query` | `/api/families?query=nginx` |
| `/api/images/{name}/tags` | | `/api/images/python/tags` |
| `/api/images/{name}/details` | `tag` (default `latest`) | `/api/images/python/details?tag=latest` |
| `/api/images/{name}/pin` | `tag` | `/api/images/python/pin?tag=latest-dev` |
| `/api/images/{name}/packages` | `tag`, `arch` (`amd64` or `arm64`), `query` | `/api/images/python/packages?query=ssl` |
| `/api/images/{name}/vulnerabilities` | `tag`, `id` (a CVE or GHSA ID) | `/api/images/python/vulnerabilities?id=CVE-2024-12797` |
| `/api/images/{name}/sbom` | `tag`, `arch` | `/api/images/python/sbom?tag=latest` |
| `/api/alternatives` | `image` (required) | `/api/alternatives?image=node:20-alpine` |

Errors are JSON `{"error": "...", "code": "..."}` with status 400 `invalid_input`, 403 `not_public` (subscription-only image), 404 `not_found` (also for unknown `/api/` paths) or 502 `upstream_error`. The API has no authentication; bind it to `127.0.0.1` unless you mean to share it.

### Saving SBOMs

`save_sbom` writes files named like `python-latest-amd64.spdx.json`, and never overwrites an existing file unless asked to. Where they go depends on how the server runs:

- **Stdio mode:** the folder the server was started in. For Claude Code, that's your project folder.
- **HTTP mode:** saving is turned off, because remote clients would be writing files to the server's machine.

Use `-sbom-dir` to choose a folder in either mode:

```bash
claude mcp add -s user chainguard -- /path/to/chainguard-mcp -sbom-dir /path/to/sboms
```

## Docker

The image is built on Chainguard's `node` and `go` images (build) and `static` image (runtime), and runs as a non-root user. It includes the web UI.

```bash
docker build -t chainguard-mcp .
```

HTTP mode with the web UI (the default). Bind the host port to `127.0.0.1` so it isn't reachable from your network, then open http://127.0.0.1:8080/:

```bash
docker run -d --rm -p 127.0.0.1:8080:8080 chainguard-mcp
```

Stdio mode. Passing an empty `-http=` disables HTTP:

```bash
claude mcp add chainguard -- docker run -i --rm chainguard-mcp -http=
```

To save SBOMs from the container, mount a folder and point `-sbom-dir` at it:

```bash
docker run -i --rm -v "$PWD/sboms:/sboms" chainguard-mcp -http= -sbom-dir /sboms
```

On Linux, the folder must be writable by the container's user (UID 65532).

Multi-arch build (cross-compiles, no emulation needed):

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t chainguard-mcp .
```

## Develop and test

```bash
go test ./...
```

The tests run offline against a fake Chainguard registry (`fake_test.go`). To also run the end-to-end test against the live Chainguard endpoints:

```bash
CHAINGUARD_LIVE=1 go test -run TestLive -v ./...
```

The web UI is a React + TypeScript app in `web/`, built with Vite. It needs Node.js 24:

```bash
cd web
npm ci
npm run lint && npm run typecheck && npm test
```

For live reloading while working on the UI, run the Go server and the Vite dev server together, then open http://localhost:5173/. Vite forwards `/api` requests to the Go server.

```bash
go run . -http 127.0.0.1:8080
npm --prefix web run dev
```

## Releasing

CI (`.github/workflows/ci.yml`) runs formatting, vet and tests on Linux and Windows, lints, typechecks, tests and builds the web UI, builds the Docker image and checks the UI loads, and dry-runs the release on every push to `main`.

To publish a release, push a version tag:

```bash
git tag v0.2.0
git push origin v0.2.0
```

`.github/workflows/release.yml` then uses [GoReleaser](https://goreleaser.com) to build the web UI and binaries for Linux, macOS and Windows (amd64 and arm64) and attach them to a GitHub Release with checksums.

## Project layout

| File | Contents |
|------|----------|
| `main.go` | Flags and transport setup (stdio or HTTP). |
| `service.go` | Shared logic behind both the MCP tools and the web API. |
| `tools.go` | MCP tool and prompt definitions. |
| `web.go` | JSON API under `/api/`, security headers, and serving the UI. |
| `webui.go` | Embeds the built UI from `web/dist`. |
| `catalog.go` | Image list from the directory sitemap. |
| `registry.go` | Anonymous cgr.dev client: tokens, free-tier checks, tags, manifests, blobs. |
| `details.go` | `get_image_details` and `pin_image`. |
| `sbom.go` | SBOM attestation parsing for `get_image_packages`, and saving for `save_sbom`. |
| `vulns.go` | Wolfi security database and APK version comparison. |
| `alternatives.go` | Upstream image → Chainguard image mapping, and grouping images into families of variants. |
| `groups.go` | Grouping images by variant kind or name prefix for the catalog map. |
| `history.go` | Catalog history: the daily snapshot (`-record-history`) and the Internet Archive backfill (`-backfill-history`). |
| `web/` | React + TypeScript web UI. |

## License

MIT. See [LICENSE](LICENSE).
