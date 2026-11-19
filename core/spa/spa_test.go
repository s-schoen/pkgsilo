package spa_test

import (
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/spa"
)

const (
	index = "<!doctype html><title>app</title>"
	asset = "console.log('app')"
	csp   = "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'"
	// navigation is the Accept header of a browser's navigation.
	navigation = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
)

// bigAsset is an asset that gzip makes smaller, unlike the others.
var bigAsset = strings.Repeat("console.log('app');\n", 100)

// app is an application's build, as Vite writes it.
var app = fstest.MapFS{
	"index.html":           {Data: []byte(index)},
	"favicon.svg":          {Data: []byte("<svg/>")},
	"assets/index-abc.js":  {Data: []byte(asset)},
	"assets/_chunk-def.js": {Data: []byte(asset)},
	"assets/big-ghi.js":    {Data: []byte(bigAsset)},
}

// notFound marks the responses of the handler's notFound.
var notFound = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusTeapot)
})

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	h, err := spa.Handler(app, notFound)
	require.NoError(t, err)

	return h
}

// serve sends a request with method to target with header to h.
func serve(h http.Handler, method, target string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for name, value := range header {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func TestHandler(t *testing.T) {
	t.Parallel()
	h := newHandler(t)

	tests := []struct {
		name             string
		path             string
		accept           string
		wantBody         string
		wantContentType  string
		wantCacheControl string
		wantVary         []string
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
			name: "client-side route", path: "/identities/42", accept: navigation, wantBody: index,
			wantContentType: "text/html; charset=utf-8", wantCacheControl: "no-cache", wantVary: []string{"Accept"},
		},
		{
			name: "client-side route with a dot", path: "/packages/lodash.merge", accept: navigation, wantBody: index,
			wantContentType: "text/html; charset=utf-8", wantCacheControl: "no-cache", wantVary: []string{"Accept"},
		},
		{
			name: "client-side route with invalid UTF-8", path: "/identities/%FF", accept: "text/html", wantBody: index,
			wantContentType: "text/html; charset=utf-8", wantCacheControl: "no-cache", wantVary: []string{"Accept"},
		},
		{
			name: "directory", path: "/assets/", accept: navigation, wantBody: index,
			wantContentType: "text/html; charset=utf-8", wantCacheControl: "no-cache", wantVary: []string{"Accept"},
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
		{
			name: "compressible asset to a client without gzip", path: "/assets/big-ghi.js", wantBody: bigAsset,
			wantContentType: "text/javascript; charset=utf-8", wantCacheControl: "public, max-age=31536000, immutable",
			wantVary: []string{"Accept-Encoding"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := serve(h, http.MethodGet, tt.path, map[string]string{"Accept": tt.accept})

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.wantBody, rec.Body.String())
			assert.Equal(t, tt.wantContentType, rec.Header().Get("Content-Type"))
			assert.Equal(t, tt.wantCacheControl, rec.Header().Get("Cache-Control"))
			assert.Equal(t, csp, rec.Header().Get("Content-Security-Policy"))
			assert.Equal(t, tt.wantVary, rec.Header().Values("Vary"))
			assert.Empty(t, rec.Header().Get("Content-Encoding"))
			assert.NotEmpty(t, rec.Header().Get("ETag"))
		})
	}
}

// TestHandler_NotFound checks that a missing path gets notFound unless the request is a navigation: a stale
// page's request for an asset of an older build, and an API client's request for a path that no route takes,
// such as the metadata that an MCP client looks for, get no HTML in place of what they asked for.
func TestHandler_NotFound(t *testing.T) {
	t.Parallel()
	h := newHandler(t)

	tests := []struct {
		name   string
		path   string
		accept string
	}{
		{name: "asset of an older build", path: "/assets/index-old.js", accept: "*/*"},
		{name: "style of an older build", path: "/assets/index-old.css", accept: "text/css,*/*;q=0.1"},
		{name: "missing file", path: "/robots.txt"},
		{name: "API client", path: "/.well-known/oauth-protected-resource", accept: "application/json"},
		{name: "client that accepts anything", path: "/identities/42", accept: "*/*"},
		{name: "client that refuses HTML", path: "/identities/42", accept: "text/html;q=0, */*"},
		{name: "invalid UTF-8", path: "/%FF"},
		{name: "directory", path: "/assets/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := serve(h, http.MethodGet, tt.path, map[string]string{"Accept": tt.accept})

			assert.Equal(t, http.StatusTeapot, rec.Code)
			assert.Equal(t, []string{"Accept"}, rec.Header().Values("Vary"))
			assert.Empty(t, rec.Header().Get("Content-Security-Policy"))
			assert.Empty(t, rec.Header().Get("Cache-Control"))
		})
	}
}

func TestHandler_Gzip(t *testing.T) {
	t.Parallel()
	h := newHandler(t)
	plain := serve(h, http.MethodGet, "/assets/big-ghi.js", nil)

	rec := serve(h, http.MethodGet, "/assets/big-ghi.js", map[string]string{"Accept-Encoding": "gzip, deflate, br"})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Equal(t, []string{"Accept-Encoding"}, rec.Header().Values("Vary"))
	assert.Equal(t, "text/javascript; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Less(t, rec.Body.Len(), len(bigAsset))
	assert.NotEqual(t, plain.Header().Get("ETag"), rec.Header().Get("ETag"))
	zr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	content, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.Equal(t, bigAsset, string(content))
}

func TestHandler_NoGzip(t *testing.T) {
	t.Parallel()
	h := newHandler(t)

	tests := []struct {
		name           string
		path           string
		acceptEncoding string
		wantVary       []string
	}{
		{name: "refused", path: "/assets/big-ghi.js", acceptEncoding: "gzip;q=0, br", wantVary: []string{"Accept-Encoding"}},
		{name: "file that gzip doesn't make smaller", path: "/assets/index-abc.js", acceptEncoding: "gzip"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := serve(h, http.MethodGet, tt.path, map[string]string{"Accept-Encoding": tt.acceptEncoding})

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Empty(t, rec.Header().Get("Content-Encoding"))
			assert.Equal(t, tt.wantVary, rec.Header().Values("Vary"))
		})
	}
}

// TestHandler_NotModified checks that a browser revalidating index.html with its ETag gets a 304, for / and
// for a client-side route, and for the compressed index.html too.
func TestHandler_NotModified(t *testing.T) {
	t.Parallel()
	h, err := spa.Handler(fstest.MapFS{"index.html": {Data: []byte(bigAsset)}}, notFound)
	require.NoError(t, err)

	for _, encoding := range []string{"", "gzip"} {
		for _, path := range []string{"/", "/identities/42"} {
			header := map[string]string{"Accept": navigation, "Accept-Encoding": encoding}
			etag := serve(h, http.MethodGet, path, header).Header().Get("ETag")
			require.NotEmpty(t, etag)
			header["If-None-Match"] = etag

			rec := serve(h, http.MethodGet, path, header)

			assert.Equal(t, http.StatusNotModified, rec.Code, path, encoding)
			assert.Empty(t, rec.Body.String(), path, encoding)
			assert.Equal(t, etag, rec.Header().Get("ETag"), path, encoding)
		}
	}
}

func TestHandler_Head(t *testing.T) {
	t.Parallel()
	h := newHandler(t)

	rec := serve(h, http.MethodHead, "/assets/index-abc.js", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
	assert.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
}

// failingFS fails every Open with an error other than fs.ErrNotExist.
type failingFS struct{}

func (failingFS) Open(string) (fs.File, error) {
	return nil, errors.New("disk on fire")
}

func TestHandler_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fsys    fs.FS
		wantErr string
	}{
		{name: "file system error", fsys: failingFS{}, wantErr: "read the application: disk on fire"},
		{name: "no index.html", fsys: fstest.MapFS{"favicon.svg": {Data: []byte("<svg/>")}}, wantErr: "no index.html"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := spa.Handler(tt.fsys, notFound)

			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
