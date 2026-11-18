// Package spa serves a single-page application, such as the web UI, from a file system.
package spa

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"pkgsilo/core/envelope"
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

// errNotFile is returned by open for a path that names no regular file.
var errNotFile = errors.New("not a file")

// Handler returns the handler that serves the application in fsys, which holds index.html at its root, for
// GET and HEAD requests. A path that names a file in fsys gets that file. A path without an extension that
// names none, such as /identities/42, is a client-side route and gets index.html. Any other path gets a 404
// problem, so that a stale page that asks for an asset of an older build gets no HTML in its place.
//
// The files under assets/ may be cached forever, and every other file must be revalidated. The files get a
// Content-Security-Policy that lets the application load from its own origin, while problems keep the
// policy that middleware.SecurityHeaders sets. Unexpected errors of fsys are logged to logger and
// answered with a 500 problem.
func Handler(fsys fs.FS, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Add("Allow", http.MethodGet)
			w.Header().Add("Allow", http.MethodHead)
			envelope.WriteProblem(w, envelope.MethodNotAllowedProblem.New(envelope.Occurrence{}))

			return
		}

		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = indexFile
		}
		file, err := open(fsys, name)
		if errors.Is(err, errNotFile) && path.Ext(name) == "" {
			name = indexFile
			file, err = open(fsys, name)
		}
		if errors.Is(err, errNotFile) {
			envelope.WriteProblem(w, envelope.NotFoundProblem.New(envelope.Occurrence{}))

			return
		}
		if err != nil {
			logger.ErrorContext(r.Context(), "serve single-page application", "error", err)
			envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))

			return
		}
		defer func() { _ = file.closer.Close() }()

		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		if strings.HasPrefix(name, assetsDir) {
			w.Header().Set("Cache-Control", assetCacheControl)
		} else {
			w.Header().Set("Cache-Control", pageCacheControl)
		}
		// The files of an embedded build have no modification time, so they get no Last-Modified.
		http.ServeContent(w, r, name, file.info.ModTime(), file.content)
	})
}

// file is an open file of the application.
type file struct {
	info    fs.FileInfo
	content io.ReadSeeker
	closer  io.Closer
}

// open opens the regular file name of fsys. It returns errNotFile if fsys has no such file, and the error of
// fsys for any other failure.
func open(fsys fs.FS, name string) (file, error) {
	f, err := fsys.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return file{}, errNotFile
	}
	if err != nil {
		return file{}, fmt.Errorf("open %s: %w", name, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()

		return file{}, fmt.Errorf("stat %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()

		return file{}, errNotFile
	}

	content, ok := f.(io.ReadSeeker)
	if !ok {
		// http.ServeContent needs to seek, which the files of embed.FS and fstest.MapFS can.
		b, err := io.ReadAll(f)
		if err != nil {
			_ = f.Close()

			return file{}, fmt.Errorf("read %s: %w", name, err)
		}
		content = bytes.NewReader(b)
	}

	return file{info: info, content: content, closer: f}, nil
}
