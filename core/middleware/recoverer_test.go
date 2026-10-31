package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/envelope"
	"pkgsilo/core/logger/loggertest"
	"pkgsilo/core/middleware"
)

func TestRecoverer(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	h := middleware.Recoverer(logger, false)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"type":"about:blank","title":"Internal Server Error","status":500}`, rec.Body.String())

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "ERROR", entries[0]["level"])
	assert.Equal(t, "panic recovered", entries[0]["msg"])
	assert.Equal(t, "boom", entries[0]["panic"])
	assert.Contains(t, entries[0]["stack"], "recoverer_test.go")
}

// writeHeaderCounter counts WriteHeader calls that reach the underlying writer.
type writeHeaderCounter struct {
	*httptest.ResponseRecorder

	calls int
}

func (w *writeHeaderCounter) WriteHeader(code int) {
	w.calls++
	w.ResponseRecorder.WriteHeader(code)
}

func TestRecoverer_KeepsStatusOfStartedResponse(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	h := middleware.Recoverer(logger, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		panic("boom")
	}))

	w := &writeHeaderCounter{ResponseRecorder: httptest.NewRecorder()}
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, 1, w.calls)
	assert.Len(t, loggertest.Entries(t, buf), 1)
}

func TestRecoverer_RepanicsErrAbortHandler(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	h := middleware.Recoverer(logger, false)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	})
	assert.Empty(t, buf.String())
}
