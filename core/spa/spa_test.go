package spa_test

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core/envelope"
	"pkgsilo/core/spa"
	"pkgsilo/test/loggertest"
)

const (
	index = "<!doctype html><title>app</title>"
	asset = "console.log('app')"
	csp   = "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'"
)

// app is an application's build, as Vite writes it.
var app = fstest.MapFS{
	"index.html":           {Data: []byte(index)},
	"favicon.svg":          {Data: []byte("<svg/>")},
	"assets/index-abc.js":  {Data: []byte(asset)},
	"assets/_chunk-def.js": {Data: []byte(asset)},
}

func TestHandler(t *testing.T) {
	t.Parallel()
	h := spa.Handler(app, loggertest.Discard())

	tests := []struct {
		name             string
		path             string
		wantBody         string
		wantContentType  string
		wantCacheControl string
	}{
		{
			name: "root", path: "/", wantBody: index,
			wantContentType: "text/html; charset=utf-8", wantCacheControl: "no-cache",
		},
		{
			name: "index", path: "/index.html", wantBody: index,
			wantContentType: "text/html; charset=utf-8", wantCacheControl: "no-cache",
		},
		{
			name: "client-side route", path: "/identities/42", wantBody: index,
			wantContentType: "text/html; charset=utf-8", wantCacheControl: "no-cache",
		},
		{
			name: "directory", path: "/assets/", wantBody: index,
			wantContentType: "text/html; charset=utf-8", wantCacheControl: "no-cache",
		},
		{
			name: "file outside the assets", path: "/favicon.svg", wantBody: "<svg/>",
			wantContentType: "image/svg+xml", wantCacheControl: "no-cache",
		},
		{
			name: "asset", path: "/assets/index-abc.js", wantBody: asset,
			wantContentType: "text/javascript; charset=utf-8", wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name: "asset starting with an underscore", path: "/assets/_chunk-def.js", wantBody: asset,
			wantContentType: "text/javascript; charset=utf-8", wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name: "path escaping the root", path: "/../../assets/index-abc.js", wantBody: asset,
			wantContentType: "text/javascript; charset=utf-8", wantCacheControl: "public, max-age=31536000, immutable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.wantBody, rec.Body.String())
			assert.Equal(t, tt.wantContentType, rec.Header().Get("Content-Type"))
			assert.Equal(t, tt.wantCacheControl, rec.Header().Get("Cache-Control"))
			assert.Equal(t, csp, rec.Header().Get("Content-Security-Policy"))
		})
	}
}

func TestHandler_Head(t *testing.T) {
	t.Parallel()
	h := spa.Handler(app, loggertest.Discard())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, "/assets/index-abc.js", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
}

// TestHandler_MissingFile checks that a missing path with an extension, such as an asset of an older build,
// gets a 404 problem instead of index.html, and keeps the API's Content-Security-Policy.
func TestHandler_MissingFile(t *testing.T) {
	t.Parallel()
	h := spa.Handler(app, loggertest.Discard())

	for _, path := range []string{"/assets/index-old.js", "/robots.txt"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		assert.Equal(t, http.StatusNotFound, rec.Code, path)
		assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"), path)
		assert.JSONEq(t, `{"type":"about:blank","title":"Not Found","status":404}`, rec.Body.String(), path)
		assert.Empty(t, rec.Header().Get("Content-Security-Policy"), path)
		assert.Empty(t, rec.Header().Get("Cache-Control"), path)
	}
}

func TestHandler_MissingIndex(t *testing.T) {
	t.Parallel()
	h := spa.Handler(fstest.MapFS{}, loggertest.Discard())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	h := spa.Handler(app, loggertest.Discard())

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/", nil))

		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, method)
		assert.Equal(t, []string{http.MethodGet, http.MethodHead}, rec.Header().Values("Allow"), method)
		assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"), method)
		assert.Empty(t, rec.Header().Get("Content-Security-Policy"), method)
	}
}

// failingFS fails every Open with an error other than fs.ErrNotExist.
type failingFS struct{}

func (failingFS) Open(string) (fs.File, error) {
	return nil, errors.New("disk on fire")
}

func TestHandler_FileSystemError(t *testing.T) {
	t.Parallel()
	logger, logs := loggertest.NewJSON()
	h := spa.Handler(failingFS{}, logger)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
	assert.Contains(t, logs.String(), "disk on fire")
}
