package authn

import (
	"context"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// LoginRecorder records the logins of a method that authenticates a request outside Require, such as a login
// with a password that creates a session: in the same metric and span as Require's attempts, so that every
// authentication is counted in one place.
type LoginRecorder struct {
	attempts attempts
	tracer   trace.Tracer
}

// NewLoginRecorder returns a LoginRecorder for the logins of method, which records with meterProvider and
// tracerProvider.
func NewLoginRecorder(
	meterProvider metric.MeterProvider, tracerProvider trace.TracerProvider, method string,
) *LoginRecorder {
	return &LoginRecorder{
		attempts: newAttempts(meterProvider, []string{method}), tracer: tracerProvider.Tracer(tracerName),
	}
}

// Record runs login in the span pkgsilo.authn.authenticate and counts it in pkgsilo.authn.attempts, with the
// attributes of Require's attempts. login returns nil if it accepts the credentials, an error wrapping
// ErrInvalidCredentials with the reason if it rejects them (see ErrInvalidCredentials), and any other error
// but ErrNoCredentials, which a login that holds credentials never returns, if it can't tell. Record returns
// that error unchanged, and logs nothing: the caller decides what a rejection logs. The span's status is
// Error only when login fails or panics. A login that panics counts as failed, and its panic goes on.
func (l *LoginRecorder) Record(ctx context.Context, login func(ctx context.Context) error) error {
	ctx, span := l.tracer.Start(ctx, authenticateSpanName)
	defer span.End()

	return l.attempts.observe(ctx, span, 0, func() error { return login(ctx) })
}
