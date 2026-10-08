package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// newWebHandler serves the JSON API under /api/ and the React UI (from ui)
// everywhere else. The API calls the same Service methods as the MCP tools.
func newWebHandler(svc *Service, ui fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/images", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		in := ListImagesInput{Query: q.Get("query"), FreeOnly: q.Get("free_only") == "true"}
		var err error
		if in.Limit, err = optionalInt(q.Get("limit")); err != nil {
			writeError(w, inputErrorf("invalid limit"))
			return
		}
		if in.Offset, err = optionalInt(q.Get("offset")); err != nil {
			writeError(w, inputErrorf("invalid offset"))
			return
		}
		respond(w)(svc.ListImages(r.Context(), in))
	})
	mux.HandleFunc("GET /api/images/{name}/tags", func(w http.ResponseWriter, r *http.Request) {
		respond(w)(svc.ImageTags(r.Context(), r.PathValue("name")))
	})
	mux.HandleFunc("GET /api/images/{name}/details", func(w http.ResponseWriter, r *http.Request) {
		respond(w)(svc.ImageDetails(r.Context(), r.PathValue("name"), r.URL.Query().Get("tag")))
	})
	mux.HandleFunc("GET /api/images/{name}/pin", func(w http.ResponseWriter, r *http.Request) {
		respond(w)(svc.PinImage(r.Context(), r.PathValue("name"), r.URL.Query().Get("tag")))
	})
	mux.HandleFunc("GET /api/images/{name}/packages", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		respond(w)(svc.ImagePackages(r.Context(), GetImagePackagesInput{
			Image: r.PathValue("name"), Tag: q.Get("tag"), Arch: q.Get("arch"), Query: q.Get("query"),
		}))
	})
	mux.HandleFunc("GET /api/images/{name}/vulnerabilities", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		respond(w)(svc.CheckVulnerabilities(r.Context(), CheckVulnerabilitiesInput{
			Image: r.PathValue("name"), Tag: q.Get("tag"), ID: q.Get("id"),
		}))
	})
	mux.HandleFunc("GET /api/images/{name}/sbom", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		doc, filename, err := svc.SBOM(r.Context(), r.PathValue("name"), q.Get("tag"), q.Get("arch"))
		if err != nil {
			writeError(w, err)
			return
		}
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, doc, "", "  "); err != nil {
			writeError(w, err)
			return
		}
		pretty.WriteByte('\n')
		w.Header().Set("Content-Type", "application/spdx+json")
		// filename is validated by sbomFilename, so it needs no escaping.
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		w.Write(pretty.Bytes())
	})
	mux.HandleFunc("GET /api/alternatives", func(w http.ResponseWriter, r *http.Request) {
		respond(w)(svc.FindAlternative(r.Context(), r.URL.Query().Get("image")))
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, apiError{Error: "unknown API endpoint", Code: "not_found"})
	})

	mux.Handle("/", uiHandler(ui))
	return securityHeaders(mux)
}

func optionalInt(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}

// respond returns a function that writes a Service result as JSON, so a
// handler can pass a method's (value, error) results straight to it.
func respond(w http.ResponseWriter) func(any, error) {
	return func(v any, err error) {
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// writeError maps Service errors to HTTP statuses and stable error codes
// the UI can switch on.
func writeError(w http.ResponseWriter, err error) {
	var inputErr *InputError
	switch {
	case errors.As(err, &inputErr):
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error(), Code: "invalid_input"})
	case errors.Is(err, ErrNotPublic):
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error(), Code: "not_public"})
	case errors.Is(err, ErrNotFound):
		writeJSON(w, http.StatusNotFound, apiError{Error: err.Error(), Code: "not_found"})
	case errors.Is(err, context.Canceled):
		// The client went away; there's no one to respond to.
	default:
		log.Printf("api error: %v", err)
		writeJSON(w, http.StatusBadGateway, apiError{Error: "upstream request failed: " + err.Error(), Code: "upstream_error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// contentSecurityPolicy allows only same-origin resources. Vite's build
// output uses external module scripts and stylesheets, so no inline
// script or style exceptions are needed.
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"connect-src 'self'; font-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// uiHandler serves the built React app. Paths that aren't files fall back to
// index.html so client-side routes like /images/python work on reload.
func uiHandler(ui fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if _, err := fs.Stat(ui, "index.html"); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(uiNotBuiltPage))
			return
		}

		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(ui, name); err == nil && !info.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					// Vite puts a content hash in asset file names.
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "no-cache")
				}
				http.ServeFileFS(w, r, ui, name)
				return
			}
			// A missing file with an extension is a real 404, not a route.
			if path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
		}
		index, err := fs.ReadFile(ui, "index.html")
		if err != nil {
			http.Error(w, "reading index.html failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}

const uiNotBuiltPage = `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>chainguard-mcp</title></head>
<body><h1>The web UI hasn't been built</h1><p>Run <code>npm ci &amp;&amp; npm run build</code> in the <code>web/</code> folder, then rebuild the Go binary.
The JSON API is available under <code>/api/</code> and MCP at <code>/mcp</code>.</p></body></html>`
