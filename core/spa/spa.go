// Package spa serves a single-page application, such as the web UI, from a file system.
package spa

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	// indexFile is the application's entry page, served for / and for every client-side route.
	indexFile = "index.html"
	// assetsDir holds the build's assets, whose names carry a hash of their content.
	assetsDir = "assets/"
	// assetCacheControl lets a browser keep an asset forever: a new build gives a changed asset a new name.
	assetCacheControl = "public, max-age=31536000, immutable"
	// pageCacheControl makes a browser revalidate every other file, so that a new build's index.html, which
	// names the new assets, is seen at once.
	pageCacheControl = "no-cache"
	// contentSecurityPolicy replaces the API's policy, which blocks every script, for the application's files.
	// The build has no inline scripts or styles, so the application loads only from its own origin.
	contentSecurityPolicy = "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; " +
		"form-action 'self'"
)

// Handler returns the handler that serves the application in fsys, which holds index.html at its root. The
// caller routes only GET and HEAD requests to it.
//
// A path that names a file in fsys gets that file. Any other path gets index.html if the request is a
// browser's navigation, such as to the client-side route /identities/42 or /packages/lodash.merge, and
// notFound otherwise, such as a stale page's request for an asset of an older build, or an API client's for a
// path that no route takes. A navigation is a request that accepts text/html by name, as browsers ask for a
// page but not for a script, a style or an image.
//
// The files under assets/ may be cached forever, and every other file must be revalidated, which its ETag
// answers. A file is sent compressed with gzip to a client that accepts it, if that makes it smaller. The files
// get a Content-Security-Policy that lets the application load from its own origin, while notFound's
// responses keep the policy that middleware.SecurityHeaders sets.
//
// Handler reads and compresses every file of fsys when it's called, and returns an error if it can't, or if fsys
// has no index.html.
func Handler(fsys fs.FS, notFound http.Handler) (http.Handler, error) {
	files, err := readFiles(fsys)
	if err != nil {
		return nil, err
	}
	index, ok := files[indexFile]
	if !ok {
		return nil, errors.New("read the application: no " + indexFile)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = indexFile
		}
		if f, ok := files[name]; ok {
			f.serve(w, r)

			return
		}
		// Whether a missing path gets index.html depends on the request's Accept header.
		w.Header().Add("Vary", "Accept")
		if !accepts(r.Header.Values("Accept"), "text/html") {
			notFound.ServeHTTP(w, r)

			return
		}
		index.serve(w, r)
	}), nil
}

// readFiles returns the regular files of fsys by their names.
func readFiles(fsys fs.FS) (map[string]*file, error) {
	var names []string
	err := fs.WalkDir(fsys, ".", func(name string, entry fs.DirEntry, err error) error {
		if err == nil && entry.Type().IsRegular() {
			names = append(names, name)
		}

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("read the application: %w", err)
	}

	files := make(map[string]*file, len(names))
	for _, name := range names {
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read the application: %w", err)
		}
		f, err := newFile(name, content)
		if err != nil {
			return nil, err
		}
		files[name] = f
	}

	return files, nil
}

// file is a regular file of the application, as the handler serves it.
type file struct {
	contentType  string
	cacheControl string
	plain        representation
	// gzipped is nil if gzip doesn't make the file smaller.
	gzipped *representation
}

// representation is the content of a file in one encoding, with its ETag.
type representation struct {
	content []byte
	etag    string
}

// newFile returns the file name of the application, with content.
func newFile(name string, content []byte) (*file, error) {
	f := &file{
		contentType:  mime.TypeByExtension(path.Ext(name)),
		cacheControl: pageCacheControl,
		plain:        newRepresentation(content),
	}
	if f.contentType == "" {
		f.contentType = http.DetectContentType(content)
	}
	if strings.HasPrefix(name, assetsDir) {
		f.cacheControl = assetCacheControl
	}

	var gzipped bytes.Buffer
	zw, err := gzip.NewWriterLevel(&gzipped, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("compress %s: %w", name, err)
	}
	if _, err := zw.Write(content); err != nil {
		return nil, fmt.Errorf("compress %s: %w", name, err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("compress %s: %w", name, err)
	}
	if gzipped.Len() < len(content) {
		r := newRepresentation(gzipped.Bytes())
		f.gzipped = &r
	}

	return f, nil
}

// newRepresentation returns content with a strong ETag of its hash.
func newRepresentation(content []byte) representation {
	sum := sha256.Sum256(content)

	return representation{content: content, etag: `"` + base64.RawURLEncoding.EncodeToString(sum[:16]) + `"`}
}

// serve writes f to w, compressed if r accepts gzip, and answers r's conditional and range headers.
func (f *file) serve(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Content-Security-Policy", contentSecurityPolicy)
	header.Set("Cache-Control", f.cacheControl)
	header.Set("Content-Type", f.contentType)
	rep := f.plain
	if f.gzipped != nil {
		header.Add("Vary", "Accept-Encoding")
		if accepts(r.Header.Values("Accept-Encoding"), "gzip") {
			rep = *f.gzipped
			header.Set("Content-Encoding", "gzip")
		}
	}
	header.Set("ETag", rep.etag)
	// The files of an embedded build have no modification time, so they get no Last-Modified: the ETag
	// validates them.
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(rep.content))
}

// accepts reports whether the values of an Accept or Accept-Encoding header list want with a weight above 0. A
// wildcard, such as */*, doesn't count: a client that accepts anything didn't ask for want.
func accepts(values []string, want string) bool {
	for _, value := range values {
		for element := range strings.SplitSeq(value, ",") {
			// It parses an element of Accept-Encoding too, which is a token with parameters.
			name, params, err := mime.ParseMediaType(element)
			if err != nil || name != want {
				continue
			}
			if q, ok := params["q"]; ok {
				if weight, err := strconv.ParseFloat(q, 64); err != nil || weight <= 0 {
					continue
				}
			}

			return true
		}
	}

	return false
}
