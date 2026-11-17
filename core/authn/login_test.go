package authn_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/authn"
	"pkgsilo/test/telemetrytest"
)

// recordLogin records one login that returns err, or panics with it if panicking, in a parent span, and
// returns what it recorded, the span that login ran in and what Record returned.
func recordLogin(t *testing.T, err error, panicking bool) (served, trace.SpanContext, error) {
	t.Helper()

	got := served{telemetry: telemetrytest.New(t)}
	recorder := authn.NewLoginRecorder(got.telemetry.MeterProvider(), got.telemetry.TracerProvider(), "password")
	ctx, parent := got.telemetry.TracerProvider().Tracer("pkgsilo/test").Start(t.Context(), "parent")
	got.parent = parent.SpanContext()
	var loginSpan trace.SpanContext
	login := func(ctx context.Context) error {
		loginSpan = trace.SpanFromContext(ctx).SpanContext()
		if panicking {
			panic(err)
		}

		return err
	}

	var recorded error
	if panicking {
		assert.PanicsWithValue(t, err, func() { _ = recorder.Record(ctx, login) })
	} else {
		recorded = recorder.Record(ctx, login)
	}
	parent.End()

	return got, loginSpan, recorded
}

func TestLoginRecorder_Record(t *testing.T) {
	t.Parallel()
	errDB := errors.New("database down")
	tests := []struct {
		name      string
		err       error
		panicking bool
		want      map[string]string
		status    codes.Code
	}{
		{
			name: "success",
			want: map[string]string{"pkgsilo.authn.outcome": "success", "pkgsilo.authn.method": "password"},
		},
		{
			name: "unknown",
			err:  fmt.Errorf("%w: no such identity", authn.ErrUnknownCredentials),
			want: map[string]string{
				"pkgsilo.authn.outcome": "rejected", "pkgsilo.authn.method": "password", "pkgsilo.authn.reason": "unknown",
			},
		},
		{
			name: "invalid",
			err:  fmt.Errorf("%w: wrong password", authn.ErrInvalidCredentials),
			want: map[string]string{
				"pkgsilo.authn.outcome": "rejected", "pkgsilo.authn.method": "password", "pkgsilo.authn.reason": "invalid",
			},
		},
		{
			name:   "failed",
			err:    errDB,
			want:   map[string]string{"pkgsilo.authn.outcome": "error", "pkgsilo.authn.method": "password"},
			status: codes.Error,
		},
		{
			name:      "panicked",
			err:       errDB,
			panicking: true,
			want:      map[string]string{"pkgsilo.authn.outcome": "error", "pkgsilo.authn.method": "password"},
			status:    codes.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, loginSpan, err := recordLogin(t, tt.err, tt.panicking)

			if !tt.panicking {
				require.Equal(t, tt.err, err, "the login's error is returned unchanged")
			}
			assertAttempts(t, got, tt.want)
			span := got.authnSpan(t)
			assert.Equal(t, tt.status, span.Status().Code)
			assert.Equal(t, span.SpanContext(), loginSpan, "the login runs in the span, so that its queries nest under it")
		})
	}
}
