package openapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"slices"
	"time"
)

// Document is an OpenAPI document that Spec.Build returned, as an http.Handler that serves it.
type Document struct {
	body   []byte
	etag   string
	routes []Route
}

var _ http.Handler = (*Document)(nil)

// Route is the method and path of an operation of the document, such as GET /api/v1/identities/{id}.
type Route struct {
	Method string
	Path   string
}

func newDocument(body []byte, routes []Route) *Document {
	sum := sha256.Sum256(body)

	return &Document{body: body, etag: `"` + hex.EncodeToString(sum[:16]) + `"`, routes: routes}
}

// JSON returns the document in JSON.
func (d *Document) JSON() []byte {
	return slices.Clone(d.body)
}

// Routes returns the method and path of every operation of the document, in the order that the spec
// added them.
func (d *Document) Routes() []Route {
	return slices.Clone(d.routes)
}

// ServeHTTP answers a GET or HEAD request with the document in JSON. It honors conditional requests, with an
// ETag of the document's content.
func (d *Document) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", d.etag)
	// Revalidated on every use, since a new version of pkgsilo changes the document at the same URL.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(d.body))
}
