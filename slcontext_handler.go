package slogenv

import (
	"context"
	"log/slog"
)

// AttrsFunc pulls ctx-scoped attrs, e.g. a request correlation id.
// Returning nil or an empty slice leaves the record untouched.
type AttrsFunc func(ctx context.Context) []slog.Attr

// See https://pkg.go.dev/log/slog#example-Handler-LevelHandler
// and https://github.com/golang/example/blob/master/slog-handler-guide/README.md
//
// Wraps next so every record carries the attrs fn reports for its ctx.
//
// A nil fn returns next unchanged.
func NewContextHandler(next slog.Handler, fn AttrsFunc) slog.Handler {
	if fn == nil {
		return next
	}
	return contextHandler{handler: next, attrs: fn}
}

type contextHandler struct {
	handler slog.Handler
	attrs   AttrsFunc // never nil, see NewContextHandler
}

func (h contextHandler) Handle(ctx context.Context, record slog.Record) error {
	attrs := h.attrs(ctx)
	if len(attrs) == 0 { // no attrs to inject to record
		return h.handler.Handle(ctx, record)
	}
	// The ctx attrs come first, right after the message.
	// A new record is needed because Record.AddAttrs can only append.
	out := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	out.AddAttrs(attrs...)
	record.Attrs(func(attr slog.Attr) bool {
		out.AddAttrs(attr)
		return true
	})
	return h.handler.Handle(ctx, out)
}

// Rewraps

func (h contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.handler.WithAttrs(attrs), h.attrs}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.handler.WithGroup(name), h.attrs}
}
