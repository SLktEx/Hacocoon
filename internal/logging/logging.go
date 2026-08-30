package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
)

const (
	FormatText = "text"
	FormatJSON = "json"
	Redacted   = "[REDACTED]"
)

type Config struct {
	Writer io.Writer
	Level  slog.Level
	Format string
}

type contextKey struct{}

var (
	rootLogger atomic.Pointer[slog.Logger]
	discard    = slog.New(slog.NewTextHandler(io.Discard, nil))

	authorizationPattern = regexp.MustCompile(`(?i)\b(authorization|proxy-authorization|cookie|set-cookie)\s*[:=]\s*[^\r\n]+`)
	urlCredentialPattern = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/\s@]+@`)
	secretQueryPattern   = regexp.MustCompile(`(?i)([?&](?:access[_-]?token|refresh[_-]?token|client[_-]?secret|token|password|passwd|secret|api[_-]?key|credential)=)[^&#\s]+`)
	secretAssignPattern  = regexp.MustCompile(`(?i)([A-Za-z0-9_.-]*(?:password|passwd|token|secret|api[_-]?key|credential)[A-Za-z0-9_.-]*)\s*=\s*([^\s,;]+)`)
)

func New(config Config) (*slog.Logger, error) {
	writer := config.Writer
	if writer == nil {
		writer = io.Discard
	}
	format := strings.ToLower(strings.TrimSpace(config.Format))
	if format == "" {
		format = FormatText
	}

	options := &slog.HandlerOptions{Level: config.Level}
	var handler slog.Handler
	switch format {
	case FormatText:
		handler = slog.NewTextHandler(writer, options)
	case FormatJSON:
		handler = slog.NewJSONHandler(writer, options)
	default:
		return nil, fmt.Errorf("unsupported log format %q", config.Format)
	}
	return slog.New(redactingHandler{inner: handler}), nil
}

func NewFromEnv(writer io.Writer) (*slog.Logger, error) {
	level, err := ParseLevel(os.Getenv("HACO_LOG_LEVEL"))
	if err != nil {
		return nil, err
	}
	format := strings.TrimSpace(os.Getenv("HACO_LOG_FORMAT"))
	return New(Config{Writer: writer, Level: level, Format: format})
}

func ParseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported log level %q", raw)
	}
}

func SetRoot(logger *slog.Logger) {
	rootLogger.Store(logger)
}

func Root() *slog.Logger {
	if logger := rootLogger.Load(); logger != nil {
		return logger
	}
	return discard
}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = Root()
	}
	return context.WithValue(ctx, contextKey{}, logger)
}

func FromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if logger, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	return Root()
}

func With(ctx context.Context, args ...any) context.Context {
	return WithLogger(ctx, FromContext(ctx).With(args...))
}

func SanitizeArgs(args []string) []string {
	out := make([]string, len(args))
	redactNext := false
	for i, arg := range args {
		if redactNext {
			out[i] = Redacted
			redactNext = false
			continue
		}

		trimmed := strings.TrimSpace(arg)
		if strings.HasPrefix(trimmed, "--") {
			if key, value, ok := strings.Cut(trimmed, "="); ok {
				if sensitiveKey(strings.TrimPrefix(key, "--")) {
					out[i] = key + "=" + Redacted
					continue
				}
				out[i] = key + "=" + RedactString(value)
				continue
			}
			if sensitiveKey(strings.TrimPrefix(trimmed, "--")) {
				out[i] = trimmed
				redactNext = true
				continue
			}
		}

		if key, value, ok := strings.Cut(trimmed, "="); ok && !strings.ContainsAny(key, "/: ") && sensitiveKey(key) {
			out[i] = key + "=" + Redacted
			continue
		} else if ok && !strings.ContainsAny(key, "/: ") {
			out[i] = key + "=" + RedactString(value)
			continue
		}

		out[i] = RedactString(arg)
	}
	return out
}

func RedactString(value string) string {
	value = authorizationPattern.ReplaceAllString(value, "$1: "+Redacted)
	value = urlCredentialPattern.ReplaceAllString(value, "$1"+Redacted+"@")
	value = secretQueryPattern.ReplaceAllString(value, "$1"+Redacted)
	value = secretAssignPattern.ReplaceAllString(value, "$1="+Redacted)
	return value
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.TrimSpace(key)))
	for _, marker := range []string{
		"password",
		"passwd",
		"token",
		"secret",
		"authorization",
		"proxyauthorization",
		"cookie",
		"credential",
		"privatekey",
		"apikey",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

type redactingHandler struct {
	inner slog.Handler
}

func (h redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, RedactString(record.Message), record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clean.AddAttrs(redactAttr(attr))
		return true
	})
	return h.inner.Handle(ctx, clean)
}

func (h redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		clean[i] = redactAttr(attr)
	}
	return redactingHandler{inner: h.inner.WithAttrs(clean)}
}

func (h redactingHandler) WithGroup(name string) slog.Handler {
	return redactingHandler{inner: h.inner.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if sensitiveKey(attr.Key) {
		attr.Value = slog.StringValue(Redacted)
		return attr
	}

	switch attr.Value.Kind() {
	case slog.KindString:
		attr.Value = slog.StringValue(RedactString(attr.Value.String()))
	case slog.KindAny:
		switch value := attr.Value.Any().(type) {
		case error:
			attr.Value = slog.StringValue(RedactString(value.Error()))
		case fmt.Stringer:
			attr.Value = slog.StringValue(RedactString(value.String()))
		}
	case slog.KindGroup:
		group := attr.Value.Group()
		clean := make([]slog.Attr, len(group))
		for i, child := range group {
			clean[i] = redactAttr(child)
		}
		attr.Value = slog.GroupValue(clean...)
	}
	return attr
}
