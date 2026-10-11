package main

import (
	"errors"
	"regexp"
	"strings"
)

var (
	// ErrUnsupportedRegistry is returned for upstream images outside Docker Hub.
	ErrUnsupportedRegistry = errors.New("only Docker Hub images can be compared")
	// ErrNoAlternative is returned when no Chainguard alternative exists for
	// an upstream image, so there is nothing to compare it with.
	ErrNoAlternative = errors.New("no Chainguard alternative found for this image")
)

// SourceError reports that a data source the comparison depends on failed.
type SourceError struct {
	Source string // "Docker Hub", "OSV.dev" or "endoflife.date"
	Err    error
}

func (e *SourceError) Error() string { return e.Source + " failed: " + e.Err.Error() }
func (e *SourceError) Unwrap() error { return e.Err }

var dockerHubHosts = map[string]bool{
	"docker.io": true, "index.docker.io": true, "registry-1.docker.io": true, "registry.hub.docker.com": true,
}

// registryHostPattern matches a registry host with an optional port.
var registryHostPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?(?::[0-9]+)?$`)

// dockerRepoPart is one path component of a Docker Hub repository name.
var dockerRepoPart = regexp.MustCompile(`^[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*$`)

// parseDockerHubRef splits an upstream reference such as "node:20",
// "bitnami/redis" or "docker.io/library/python:3.12-slim" into a Docker Hub
// repository path ("library/node") and a tag or digest. A reference on any
// other registry gets ErrUnsupportedRegistry.
func parseDockerHubRef(ref string) (repo, tag string, err error) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if before, digest, ok := strings.Cut(ref, "@"); ok {
		ref, tag = before, digest
		if !strings.HasPrefix(digest, "sha256:") || len(digest) != len("sha256:")+64 {
			return "", "", inputErrorf("invalid digest in %q", ref)
		}
	}
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		if tag != "" {
			return "", "", inputErrorf("give a tag or a digest, not both")
		}
		ref, tag = ref[:i], ref[i+1:]
		if tag == "" {
			return "", "", inputErrorf("missing tag after \":\" in %q", ref)
		}
	}
	parts := strings.Split(ref, "/")
	if len(parts) > 1 && (strings.ContainsAny(parts[0], ".:") || parts[0] == "localhost") && registryHostPattern.MatchString(parts[0]) {
		if !dockerHubHosts[parts[0]] {
			return "", "", ErrUnsupportedRegistry
		}
		parts = parts[1:]
	}
	if len(parts) == 1 {
		parts = []string{"library", parts[0]}
	}
	for _, p := range parts {
		if !dockerRepoPart.MatchString(p) {
			return "", "", inputErrorf("invalid image reference %q", ref)
		}
	}
	if tag == "" {
		tag = "latest"
	} else if !strings.HasPrefix(tag, "sha256:") && !validTag.MatchString(tag) {
		return "", "", inputErrorf("invalid tag %q", tag)
	}
	return strings.Join(parts, "/"), tag, nil
}
