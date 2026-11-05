package middleware

import (
	"context"
	"log/slog"
)

type logAttrsKey struct{}

// logAttrs holds the attributes that AddLogAttrs added to a request.
type logAttrs struct {
	attrs []slog.Attr
}

// withLogAttrs returns ctx with an empty holder for the attributes of AddLogAttrs.
func withLogAttrs(ctx context.Context) context.Context {
	return context.WithValue(ctx, logAttrsKey{}, &logAttrs{})
}

// AddLogAttrs adds attrs to every record logged from now on with a context of the request. That
// includes the record of RequestLogger, which a value in the context wouldn't reach, since contexts
// only pass inward. It does nothing for a context that didn't pass through RequestLogger.
//
// It must not run concurrently with logging, so call it before the handler starts goroutines.
func AddLogAttrs(ctx context.Context, attrs ...slog.Attr) {
	if holder, ok := ctx.Value(logAttrsKey{}).(*logAttrs); ok {
		holder.attrs = append(holder.attrs, attrs...)
	}
}

// LogAttrsFromContext returns the attributes that AddLogAttrs added to the request, or nil if there
// are none.
func LogAttrsFromContext(ctx context.Context) []slog.Attr {
	holder, ok := ctx.Value(logAttrsKey{}).(*logAttrs)
	if !ok {
		return nil
	}

	return holder.attrs
}
