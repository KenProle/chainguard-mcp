package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newServer registers the MCP tools and prompt. Each tool is a thin wrapper
// around a Service method (service.go), which the web API also uses.
func newServer(svc *Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "chainguard-images", Version: version}, nil)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_images",
		Description: "List available Chainguard container images from the Chainguard Images directory, optionally filtered by name. Each result says whether the image is free (pullable without a subscription).",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListImagesInput) (*mcp.CallToolResult, ListImagesOutput, error) {
		out, err := svc.ListImages(ctx, in)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "catalog_changes",
		Description: "Show what changed in Chainguard's image catalog and when, from this project's catalog history: images added, removed, newly free or newly subscription-only, with the catalog size at `since` and now. " +
			"Daily snapshots start on 2026-10-10; earlier dates come from Internet Archive captures, so their changes are ranges between two captures, and an image missing from a capture isn't necessarily removed. " +
			"The result gives the age of the data and says if the daily collector may have stalled.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in HistoryInput) (*mcp.CallToolResult, *HistoryReport, error) {
		out, err := svc.CatalogHistory(ctx, in)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_image_tags",
		Description: "List the tags of a Chainguard image on cgr.dev. Only works for free-tier images.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetImageTagsInput) (*mcp.CallToolResult, GetImageTagsOutput, error) {
		out, err := svc.ImageTags(ctx, in.Image)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_image_details",
		Description: "Get details for a Chainguard image tag: supported platforms with download sizes, digest, creation time, the user it runs as, entrypoint, command, working directory and environment. Only works for free-tier images.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ImageInput) (*mcp.CallToolResult, *ImageDetails, error) {
		out, err := svc.ImageDetails(ctx, in.Image, in.Tag)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "pin_image",
		Description: "Resolve a Chainguard image tag to its current digest, returning a pinned reference (tag@sha256:…) for reproducible Dockerfiles. Only works for free-tier images.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ImageInput) (*mcp.CallToolResult, *PinResult, error) {
		out, err := svc.PinImage(ctx, in.Image, in.Tag)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_image_packages",
		Description: "List the OS packages installed in a Chainguard image, read from its signed SBOM, with versions, licenses and source packages. Also reports whether the image has a shell or the apk package manager. Only works for free-tier images.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetImagePackagesInput) (*mcp.CallToolResult, GetImagePackagesOutput, error) {
		out, err := svc.ImagePackages(ctx, in)
		return nil, out, err
	})

	if svc.SBOMDir != "" {
		mcp.AddTool(server, &mcp.Tool{
			Name: "save_sbom",
			Description: "Save the full SPDX SBOM (software bill of materials) of a Chainguard image as a JSON file, for compliance or scanning tools. " +
				"Files are saved to the server's SBOM folder (" + svc.SBOMDir + "); the result includes the full path. Only works for free-tier images.",
			Annotations: &mcp.ToolAnnotations{IdempotentHint: true, DestructiveHint: new(false)},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, in SaveSBOMInput) (*mcp.CallToolResult, *SavedSBOM, error) {
			out, err := svc.SaveSBOM(ctx, in)
			return nil, out, err
		})
	}

	mcp.AddTool(server, &mcp.Tool{
		Name: "check_vulnerabilities",
		Description: "Check a Chainguard image's packages. Reports recorded fixes from the Wolfi security database, and the known open advisories Chainguard has published for the image's packages (found via OSV.dev, status from Chainguard's advisory feed), counted by vulnerability with a status each, such as waiting on an upstream fix. " +
			"With an id (CVE, GHSA or CGA), reports whether each affected package is fixed, never affected, or still vulnerable, and whether the id is an open advisory. Without an id, summarizes fixes per package and lists any fixes not yet in the image. " +
			"Neither source is a full scan, so finding nothing doesn't mean an image has no vulnerabilities. Only works for free-tier images.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in CheckVulnerabilitiesInput) (*mcp.CallToolResult, VulnReport, error) {
		out, err := svc.SecurityReport(ctx, in)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "find_alternative",
		Description: "Suggest the Chainguard image to replace an upstream image such as one from Docker Hub, MCR or gcr.io/distroless (e.g. 'node:20-alpine' → cgr.dev/chainguard/node), with related variants, free-tier status and migration notes.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in FindAlternativeInput) (*mcp.CallToolResult, *AlternativesResult, error) {
		out, err := svc.FindAlternative(ctx, in.Image)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "compare_with_upstream",
		Description: "Compare an upstream Docker Hub image, exactly as you run it (e.g. 'node:20'), with its Chainguard replacement's latest tag by download size, build date, installed OS packages and known vulnerabilities, split into fix available, no fix yet and won't fix, with an end-of-life note when the release is out of support. " +
			"Vulnerabilities come from OSV.dev per each distribution's published advisories, with the status of Chainguard's advisories read from Chainguard's own advisory feed and those already fixed in the installed version left out (the Linux kernel is excluded): the counts are known vulnerabilities, not a guarantee about either image, and each side is graded by its own maintainers. " +
			"Only Docker Hub images that publish an SBOM can be compared in full, and only free-tier Chainguard images. A first comparison takes about 20 seconds.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in CompareUpstreamInput) (*mcp.CallToolResult, *UpstreamComparison, error) {
		out, err := svc.CompareUpstream(ctx, in)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "map_packages",
		Description: "Map Debian/Ubuntu (apt) package names, such as those in a Dockerfile's apt-get install, to the Wolfi packages to apk add. " +
			"Every returned package is checked against Wolfi's live package index. Each result says where it came from: `exact` (the same name exists), `mapped` (from the dfc mapping file), `candidates` (found by a rule, such as a pkg-config module or shared library; check them) or `unknown` (nothing found, never guessed). " +
			"When there are several version streams (e.g. nodejs-24, postgresql-18-dev) the newest actively built one is chosen and the others are listed as alternatives; match it to the version the Dockerfile used. " +
			"A mapped name doesn't by itself mean the Dockerfile will build. The first call downloads Wolfi's package index and can take a few seconds.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in MapPackagesInput) (*mcp.CallToolResult, *PackageMappingResult, error) {
		out, err := svc.MapPackages(ctx, in)
		return nil, out, err
	})

	server.AddPrompt(&mcp.Prompt{
		Name:        "migrate_dockerfile",
		Title:       "Migrate a Dockerfile to Chainguard images",
		Description: "Rewrite a Dockerfile to use Chainguard images, using this server's tools to pick and pin images.",
		Arguments: []*mcp.PromptArgument{
			{Name: "dockerfile", Description: "contents of the Dockerfile to migrate", Required: true},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		dockerfile := req.Params.Arguments["dockerfile"]
		if strings.TrimSpace(dockerfile) == "" {
			return nil, fmt.Errorf("dockerfile argument is required")
		}
		return &mcp.GetPromptResult{
			Description: "Migrate a Dockerfile to Chainguard images",
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: migratePrompt + "\n\n```dockerfile\n" + dockerfile + "\n```"},
			}},
		}, nil
	})

	return server
}

const migratePrompt = `Migrate the Dockerfile below to Chainguard images (cgr.dev/chainguard/*).

Steps:
1. For each FROM line, call find_alternative to choose the Chainguard image.
2. Call get_image_details on each chosen image to check its user, entrypoint and working directory.
3. Use a multi-stage build where it helps: build in the :latest-dev variant (it has a shell and apk), and copy artifacts into the minimal :latest variant for runtime.
4. Replace apt-get/yum/dnf commands with apk add. Call map_packages with the apt package names (do not guess Wolfi names) and use the packages it returns. For a result with status candidates, use the packages but list the package in the final list of things to check, with its source and any version note; for unknown, say the package has no known replacement and list it too.
5. The minimal images have no shell, so the runtime stage must use exec-form ENTRYPOINT/CMD (JSON arrays) and no RUN steps.
6. Chainguard images run as a non-root user (usually UID 65532). Fix file ownership with COPY --chown and avoid binding ports below 1024.
7. Call pin_image for each final image and use the pinned reference in FROM.
8. If an image requires a subscription, say so and suggest the closest free option.

Return the migrated Dockerfile, then a short list of the changes and anything that needs manual testing.`
