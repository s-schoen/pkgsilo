package health

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core"
)

func TestHealth(t *testing.T) {
	r := core.NewRouter(core.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.Mount("/health", NewHandler().Routes())

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"status":"healthy","service":"pkgsilo"}`, rec.Body.String())
}
