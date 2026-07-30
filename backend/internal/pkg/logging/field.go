package logging

import (
	"log/slog"
	"time"
)

// Field is a structured log attribute.
type Field struct {
	Key   string
	Value any
}

// F builds a Field.
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

func fieldsToAttrs(fields []Field) []any {
	if len(fields) == 0 {
		return nil
	}
	out := make([]any, 0, len(fields)*2)
	for _, f := range fields {
		out = append(out, f.Key, f.Value)
	}
	return out
}

// DurationMs returns a duration_ms field.
func DurationMs(d time.Duration) Field {
	return F("duration_ms", d.Milliseconds())
}

// ErrorCode returns an error_code field.
func ErrorCode(code string) Field {
	return F("error_code", code)
}

// Module returns a module field.
func Module(name string) Field {
	return F("module", name)
}

// Operation returns an operation field.
func Operation(name string) Field {
	return F("operation", name)
}

// Result returns a result field.
func Result(name string) Field {
	return F("result", name)
}

// StringAttr converts to slog.Attr for handler use.
func StringAttr(key, value string) slog.Attr {
	return slog.String(key, value)
}
