package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
)

// Sensitive keys that must always be redacted if logged
var sensitiveKeys = map[string]bool{
	"password":      true,
	"secret":        true,
	"token":         true,
	"api_key":       true,
	"apikey":        true,
	"private_key":   true,
	"jwt":           true,
	"authorization": true,
}

// Regex to capture embedded secret patterns in unstructured text or JSON values
var (
	bearerTokenRegex = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9\-\._~\+\/]+=*`)
	privateKeyRegex  = regexp.MustCompile(`-----BEGIN [A-Z ]+ PRIVATE KEY-----[\s\S]*?-----END [A-Z ]+ PRIVATE KEY-----`)
	kvSecretRegex    = regexp.MustCompile(`(?i)(password|secret|token|api_key|private_key)\s*[:=]\s*["']?([^"'\s]+)["']?`)
)

// RedactString sanitizes strings by stripping sensitive tokens and keys.
func RedactString(input string) string {
	if input == "" {
		return ""
	}
	out := privateKeyRegex.ReplaceAllString(input, "[REDACTED_PRIVATE_KEY]")
	out = bearerTokenRegex.ReplaceAllString(out, "$1[REDACTED_TOKEN]")
	out = kvSecretRegex.ReplaceAllString(out, "$1=[REDACTED]")
	return out
}

// RedactingHandler is a slog.Handler wrapper that redacts sensitive values.
type RedactingHandler struct {
	handler slog.Handler
}

// NewRedactingHandler wraps an existing slog.Handler.
func NewRedactingHandler(h slog.Handler) *RedactingHandler {
	return &RedactingHandler{handler: h}
}

func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	newRecord := slog.NewRecord(r.Time, r.Level, RedactString(r.Message), r.PC)

	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(h.redactAttr(a))
		return true
	})

	return h.handler.Handle(ctx, newRecord)
}

func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = h.redactAttr(a)
	}
	return &RedactingHandler{handler: h.handler.WithAttrs(redacted)}
}

func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{handler: h.handler.WithGroup(name)}
}

func (h *RedactingHandler) redactAttr(a slog.Attr) slog.Attr {
	keyLower := strings.ToLower(a.Key)

	if sensitiveKeys[keyLower] {
		return slog.String(a.Key, "[REDACTED]")
	}

	if a.Value.Kind() == slog.KindString {
		return slog.String(a.Key, RedactString(a.Value.String()))
	}

	return a
}

var (
	globalLogger *slog.Logger
	loggerOnce   sync.Once
)

// InitLogger initializes the global structured logger.
func InitLogger(w io.Writer, level slog.Level) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	baseHandler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
	})
	redactHandler := NewRedactingHandler(baseHandler)
	l := slog.New(redactHandler)
	slog.SetDefault(l)
	globalLogger = l
	return l
}

// GetLogger returns the configured logger.
func GetLogger() *slog.Logger {
	loggerOnce.Do(func() {
		if globalLogger == nil {
			globalLogger = InitLogger(os.Stdout, slog.LevelInfo)
		}
	})
	return globalLogger
}
