package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Service bundles the data sources the MCP tools use.
type Service struct {
	Catalog  *Catalog
	Registry *Registry
	SecDB    *SecDB
	// SBOMDir is where save_sbom writes files. The tool is only offered
	// when it's set.
	SBOMDir string
}

func NewService() *Service {
	client := &http.Client{Timeout: 30 * time.Second}
	return &Service{
		Catalog:  NewCatalog(client),
		Registry: NewRegistry(client),
		SecDB:    NewSecDB(client),
	}
}

var (
	validImage = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)
	validTag   = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,127}$`)
)

// imageAndTag validates tool input and applies the default tag.
func imageAndTag(image, tag string) (string, string, error) {
	image = strings.TrimSpace(image)
	if !validImage.MatchString(image) {
		return "", "", fmt.Errorf("invalid image name %q: use a name from list_images, e.g. \"python\"", image)
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "latest"
	}
	if !validTag.MatchString(tag) {
		return "", "", fmt.Errorf("invalid tag %q", tag)
	}
	return image, tag, nil
}

type ListImagesInput struct {
	Query    string `json:"query,omitempty" jsonschema:"case-insensitive substring to filter image names, e.g. 'python' or 'fips'"`
	FreeOnly bool   `json:"free_only,omitempty" jsonschema:"only return free-tier images (the first unfiltered call checks every image and can take ~30s)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"maximum number of images to return (default 100, max 1000)"`
	Offset   int    `json:"offset,omitempty" jsonschema:"number of matching images to skip, for pagination"`
}

type ImageSummary struct {
	Name      string `json:"name"`
	Reference string `json:"reference"`
	Free      *bool  `json:"free,omitempty" jsonschema:"true if pullable without a subscription (omitted if the check failed)"`
}

type ListImagesOutput struct {
	Total  int            `json:"total" jsonschema:"number of images matching the query"`
	Count  int            `json:"count" jsonschema:"number of images in this page"`
	Offset int            `json:"offset"`
	Images []ImageSummary `json:"images"`
}

type ImageInput struct {
	Image string `json:"image" jsonschema:"image name as returned by list_images, e.g. 'python'"`
	Tag   string `json:"tag,omitempty" jsonschema:"tag to inspect (default 'latest')"`
}

type GetImageTagsInput struct {
	Image string `json:"image" jsonschema:"image name as returned by list_images, e.g. 'python'"`
}

type GetImageTagsOutput struct {
	Image     string   `json:"image"`
	Reference string   `json:"reference" jsonschema:"full registry reference for the image"`
	Tags      []string `json:"tags"`
}

type GetImagePackagesInput struct {
	Image string `json:"image" jsonschema:"image name as returned by list_images, e.g. 'python'"`
	Tag   string `json:"tag,omitempty" jsonschema:"tag to inspect (default 'latest')"`
	Arch  string `json:"arch,omitempty" jsonschema:"CPU architecture: 'amd64' (default) or 'arm64'"`
	Query string `json:"query,omitempty" jsonschema:"case-insensitive substring to filter package names, e.g. 'ssl'"`
}

type GetImagePackagesOutput struct {
	Image    string    `json:"image"`
	Tag      string    `json:"tag"`
	Arch     string    `json:"arch"`
	Digest   string    `json:"digest" jsonschema:"platform image digest the SBOM describes"`
	HasShell bool      `json:"has_shell" jsonschema:"whether a shell (busybox, bash or dash) is installed"`
	HasAPK   bool      `json:"has_apk" jsonschema:"whether the apk package manager is installed"`
	Total    int       `json:"total" jsonschema:"number of packages in the image"`
	Packages []Package `json:"packages" jsonschema:"packages matching the query (all if no query)"`
}

type SaveSBOMInput struct {
	Image     string `json:"image" jsonschema:"image name as returned by list_images, e.g. 'python'"`
	Tag       string `json:"tag,omitempty" jsonschema:"tag to export (default 'latest')"`
	Arch      string `json:"arch,omitempty" jsonschema:"CPU architecture: 'amd64' (default) or 'arm64'"`
	Filename  string `json:"filename,omitempty" jsonschema:"plain file name without folders (default '<image>-<tag>-<arch>.spdx.json')"`
	Overwrite bool   `json:"overwrite,omitempty" jsonschema:"replace the file if it already exists"`
}

type CheckVulnerabilitiesInput struct {
	Image string `json:"image" jsonschema:"image name as returned by list_images, e.g. 'python'"`
	Tag   string `json:"tag,omitempty" jsonschema:"tag to inspect (default 'latest')"`
	ID    string `json:"id,omitempty" jsonschema:"optional vulnerability ID to look up, e.g. 'CVE-2024-12797' or a GHSA ID; omit for a per-package summary"`
}

type FindAlternativeInput struct {
	Image string `json:"image" jsonschema:"upstream image reference, e.g. 'node:20-alpine', 'docker.io/library/python:3.12-slim' or 'mcr.microsoft.com/dotnet/aspnet:8.0'"`
}

func notPublicHint(image string, err error) error {
	return fmt.Errorf("%s: %w. Only free-tier images can be inspected anonymously", image, err)
}

func newServer(svc *Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "chainguard-images", Version: version}, nil)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_images",
		Description: "List available Chainguard container images from the Chainguard Images directory, optionally filtered by name. Each result says whether the image is free (pullable without a subscription).",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListImagesInput) (*mcp.CallToolResult, ListImagesOutput, error) {
		all, err := svc.Catalog.Images(ctx)
		if err != nil {
			return nil, ListImagesOutput{}, err
		}
		q := strings.ToLower(strings.TrimSpace(in.Query))
		var matches []string
		for _, name := range all {
			if q == "" || strings.Contains(name, q) {
				matches = append(matches, name)
			}
		}

		var free map[string]bool
		if in.FreeOnly {
			free = svc.Registry.PublicStatus(ctx, matches)
			var freeMatches []string
			for _, name := range matches {
				if free[name] {
					freeMatches = append(freeMatches, name)
				}
			}
			matches = freeMatches
		}

		limit := in.Limit
		if limit <= 0 {
			limit = 100
		}
		limit = min(limit, 1000)
		offset := min(max(in.Offset, 0), len(matches))
		page := matches[offset:min(offset+limit, len(matches))]
		if free == nil {
			free = svc.Registry.PublicStatus(ctx, page)
		}

		images := make([]ImageSummary, 0, len(page))
		for _, name := range page {
			s := ImageSummary{Name: name, Reference: svc.Registry.Ref(name)}
			if f, ok := free[name]; ok {
				s.Free = &f
			}
			images = append(images, s)
		}
		return nil, ListImagesOutput{Total: len(matches), Count: len(images), Offset: offset, Images: images}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_image_tags",
		Description: "List the tags of a Chainguard image on cgr.dev. Only works for free-tier images.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetImageTagsInput) (*mcp.CallToolResult, GetImageTagsOutput, error) {
		image, _, err := imageAndTag(in.Image, "")
		if err != nil {
			return nil, GetImageTagsOutput{}, err
		}
		tags, err := svc.Registry.Tags(ctx, image)
		if err != nil {
			return nil, GetImageTagsOutput{}, notPublicHint(image, err)
		}
		return nil, GetImageTagsOutput{Image: image, Reference: svc.Registry.Ref(image), Tags: tags}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_image_details",
		Description: "Get details for a Chainguard image tag: supported platforms with download sizes, digest, creation time, the user it runs as, entrypoint, command, working directory and environment. Only works for free-tier images.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ImageInput) (*mcp.CallToolResult, *ImageDetails, error) {
		image, tag, err := imageAndTag(in.Image, in.Tag)
		if err != nil {
			return nil, nil, err
		}
		d, err := svc.Registry.Details(ctx, image, tag)
		if err != nil {
			return nil, nil, notPublicHint(image, err)
		}
		return nil, d, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "pin_image",
		Description: "Resolve a Chainguard image tag to its current digest, returning a pinned reference (tag@sha256:…) for reproducible Dockerfiles. Only works for free-tier images.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ImageInput) (*mcp.CallToolResult, *PinResult, error) {
		image, tag, err := imageAndTag(in.Image, in.Tag)
		if err != nil {
			return nil, nil, err
		}
		p, err := svc.Registry.Pin(ctx, image, tag)
		if err != nil {
			return nil, nil, notPublicHint(image, err)
		}
		return nil, p, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_image_packages",
		Description: "List the OS packages installed in a Chainguard image, read from its signed SBOM, with versions, licenses and source packages. Also reports whether the image has a shell or the apk package manager. Only works for free-tier images.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetImagePackagesInput) (*mcp.CallToolResult, GetImagePackagesOutput, error) {
		image, tag, err := imageAndTag(in.Image, in.Tag)
		if err != nil {
			return nil, GetImagePackagesOutput{}, err
		}
		arch := strings.TrimSpace(in.Arch)
		if arch == "" {
			arch = "amd64"
		}
		pkgs, digest, err := svc.Registry.Packages(ctx, image, tag, arch)
		if err != nil {
			return nil, GetImagePackagesOutput{}, notPublicHint(image, err)
		}
		out := GetImagePackagesOutput{Image: image, Tag: tag, Arch: arch, Digest: digest, Total: len(pkgs), Packages: []Package{}}
		q := strings.ToLower(strings.TrimSpace(in.Query))
		for _, p := range pkgs {
			switch p.Name {
			case "busybox", "bash", "dash":
				out.HasShell = true
			case "apk-tools":
				out.HasAPK = true
			}
			if q == "" || strings.Contains(p.Name, q) || strings.Contains(p.Origin, q) {
				out.Packages = append(out.Packages, p)
			}
		}
		return nil, out, nil
	})

	if svc.SBOMDir != "" {
		mcp.AddTool(server, &mcp.Tool{
			Name: "save_sbom",
			Description: "Save the full SPDX SBOM (software bill of materials) of a Chainguard image as a JSON file, for compliance or scanning tools. " +
				"Files are saved to the server's SBOM folder (" + svc.SBOMDir + "); the result includes the full path. Only works for free-tier images.",
			Annotations: &mcp.ToolAnnotations{IdempotentHint: true, DestructiveHint: new(false)},
		}, func(ctx context.Context, _ *mcp.CallToolRequest, in SaveSBOMInput) (*mcp.CallToolResult, *SavedSBOM, error) {
			image, tag, err := imageAndTag(in.Image, in.Tag)
			if err != nil {
				return nil, nil, err
			}
			arch := strings.TrimSpace(in.Arch)
			if arch == "" {
				arch = "amd64"
			}
			saved, err := svc.Registry.SaveSBOM(ctx, svc.SBOMDir, image, tag, arch, in.Filename, in.Overwrite)
			if err != nil {
				return nil, nil, notPublicHint(image, err)
			}
			return nil, saved, nil
		})
	}

	mcp.AddTool(server, &mcp.Tool{
		Name: "check_vulnerabilities",
		Description: "Check a Chainguard image's packages against the Wolfi security database. With an id (CVE or GHSA), reports whether each affected package is fixed, never affected, or still vulnerable. " +
			"Without an id, summarizes fixes per package and lists any fixes not yet in the image. The database records fixes, not open vulnerabilities, so this is not a full scan. Only works for free-tier images.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in CheckVulnerabilitiesInput) (*mcp.CallToolResult, VulnReport, error) {
		image, tag, err := imageAndTag(in.Image, in.Tag)
		if err != nil {
			return nil, VulnReport{}, err
		}
		pkgs, digest, err := svc.Registry.Packages(ctx, image, tag, "amd64")
		if err != nil {
			return nil, VulnReport{}, notPublicHint(image, err)
		}
		fixes, err := svc.SecDB.Fixes(ctx)
		if err != nil {
			return nil, VulnReport{}, err
		}
		report := analyzeVulns(pkgs, fixes, in.ID)
		report.Image, report.Tag, report.Digest = image, tag, digest
		return nil, report, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "find_alternative",
		Description: "Suggest the Chainguard image to replace an upstream image such as one from Docker Hub, MCR or gcr.io/distroless (e.g. 'node:20-alpine' → cgr.dev/chainguard/node), with related variants, free-tier status and migration notes.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in FindAlternativeInput) (*mcp.CallToolResult, *AlternativesResult, error) {
		if strings.TrimSpace(in.Image) == "" {
			return nil, nil, fmt.Errorf("image is required")
		}
		r, err := svc.FindAlternatives(ctx, in.Image)
		if err != nil {
			return nil, nil, err
		}
		return nil, r, nil
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
4. Replace apt-get/yum/dnf commands with apk add, and look up package names where they differ.
5. The minimal images have no shell, so the runtime stage must use exec-form ENTRYPOINT/CMD (JSON arrays) and no RUN steps.
6. Chainguard images run as a non-root user (usually UID 65532). Fix file ownership with COPY --chown and avoid binding ports below 1024.
7. Call pin_image for each final image and use the pinned reference in FROM.
8. If an image requires a subscription, say so and suggest the closest free option.

Return the migrated Dockerfile, then a short list of the changes and anything that needs manual testing.`
