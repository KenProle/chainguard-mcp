package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// maxConfigBytes caps image config blobs; real configs are a few KB.
const maxConfigBytes = 1 << 20

type PlatformInfo struct {
	Platform string `json:"platform" jsonschema:"os/architecture, e.g. linux/amd64"`
	Digest   string `json:"digest"`
	// SizeBytes is the compressed download size (config + layers).
	SizeBytes int64 `json:"size_bytes" jsonschema:"compressed download size in bytes"`
}

type ImageDetails struct {
	Image           string         `json:"image"`
	Tag             string         `json:"tag"`
	Reference       string         `json:"reference"`
	PinnedReference string         `json:"pinned_reference" jsonschema:"tag plus digest; use this in Dockerfiles for reproducible builds"`
	Digest          string         `json:"digest" jsonschema:"digest of the multi-platform index"`
	Created         string         `json:"created,omitempty"`
	Source          string         `json:"source,omitempty" jsonschema:"URL of the image's source definition"`
	Platforms       []PlatformInfo `json:"platforms"`
	User            string         `json:"user" jsonschema:"user the container runs as; empty means root"`
	RunsAsRoot      bool           `json:"runs_as_root"`
	Entrypoint      []string       `json:"entrypoint,omitempty"`
	Cmd             []string       `json:"cmd,omitempty"`
	WorkingDir      string         `json:"working_dir,omitempty"`
	Env             []string       `json:"env,omitempty"`
	ConfigPlatform  string         `json:"config_platform" jsonschema:"platform the user/entrypoint/env fields were read from"`
}

// Details resolves image:tag and summarizes its platforms and runtime config.
func (r *Registry) Details(ctx context.Context, image, tag string) (*ImageDetails, error) {
	top, topDigest, err := r.Manifest(ctx, image, tag, acceptAnyManifest)
	if err != nil {
		return nil, err
	}
	d := &ImageDetails{
		Image:           image,
		Tag:             tag,
		Reference:       r.Ref(image) + ":" + tag,
		PinnedReference: r.Ref(image) + ":" + tag + "@" + topDigest,
		Digest:          topDigest,
		Created:         top.Annotations["org.opencontainers.image.created"],
		Source:          top.Annotations["org.opencontainers.image.source"],
		Platforms:       []PlatformInfo{},
	}

	// Collect each platform's manifest; a single-platform tag is its own manifest.
	type platformManifest struct {
		platform string
		digest   string
		m        *Manifest
	}
	var manifests []platformManifest
	if top.IsIndex() {
		for _, desc := range top.Manifests {
			if desc.Platform == nil || desc.Platform.OS == "unknown" {
				continue
			}
			m, _, err := r.Manifest(ctx, image, desc.Digest, acceptManifestOnly)
			if err != nil {
				return nil, err
			}
			manifests = append(manifests, platformManifest{desc.Platform.String(), desc.Digest, m})
		}
	} else {
		manifests = append(manifests, platformManifest{"", topDigest, top})
	}
	if len(manifests) == 0 {
		return nil, fmt.Errorf("%s:%s has no platform manifests", image, tag)
	}

	// Read runtime config from linux/amd64 when present, else the first platform.
	cfgIdx := 0
	for i, pm := range manifests {
		if pm.platform == "linux/amd64" {
			cfgIdx = i
		}
	}
	var cfg struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
		Created      string `json:"created"`
		Config       struct {
			User       string   `json:"User"`
			Entrypoint []string `json:"Entrypoint"`
			Cmd        []string `json:"Cmd"`
			WorkingDir string   `json:"WorkingDir"`
			Env        []string `json:"Env"`
		} `json:"config"`
	}
	raw, err := r.Blob(ctx, image, manifests[cfgIdx].m.Config.Digest, maxConfigBytes)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decoding image config: %w", err)
	}

	for i := range manifests {
		pm := &manifests[i]
		if pm.platform == "" {
			pm.platform = cfg.OS + "/" + cfg.Architecture
		}
		size := pm.m.Config.Size
		for _, l := range pm.m.Layers {
			size += l.Size
		}
		d.Platforms = append(d.Platforms, PlatformInfo{Platform: pm.platform, Digest: pm.digest, SizeBytes: size})
	}

	d.User = cfg.Config.User
	d.RunsAsRoot = isRootUser(cfg.Config.User)
	d.Entrypoint = cfg.Config.Entrypoint
	d.Cmd = cfg.Config.Cmd
	d.WorkingDir = cfg.Config.WorkingDir
	d.Env = cfg.Config.Env
	d.ConfigPlatform = manifests[cfgIdx].platform
	if d.Created == "" {
		d.Created = cfg.Created
	}
	return d, nil
}

// isRootUser reports whether an image config User value means root.
// The value may be "", "root", "0", "user:group" or "uid:gid".
func isRootUser(user string) bool {
	user, _, _ = strings.Cut(user, ":")
	return user == "" || user == "root" || user == "0"
}

type PinResult struct {
	Image           string `json:"image"`
	Tag             string `json:"tag"`
	Digest          string `json:"digest"`
	PinnedReference string `json:"pinned_reference" jsonschema:"reference with both tag and digest, e.g. for a Dockerfile FROM line"`
	DigestReference string `json:"digest_reference" jsonschema:"reference with only the digest"`
}

// Pin resolves image:tag to its current multi-platform digest.
func (r *Registry) Pin(ctx context.Context, image, tag string) (*PinResult, error) {
	_, digest, err := r.Manifest(ctx, image, tag, acceptAnyManifest)
	if err != nil {
		return nil, err
	}
	if digest == "" {
		return nil, fmt.Errorf("registry did not return a digest for %s:%s", image, tag)
	}
	ref := r.Ref(image)
	return &PinResult{
		Image:           image,
		Tag:             tag,
		Digest:          digest,
		PinnedReference: ref + ":" + tag + "@" + digest,
		DigestReference: ref + "@" + digest,
	}, nil
}
