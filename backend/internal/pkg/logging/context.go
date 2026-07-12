package logging

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	ctxRequestID   contextKey = "request_id"
	ctxShopID      contextKey = "shop_id"
	ctxTaskID      contextKey = "task_id"
	ctxExecutionID contextKey = "execution_id"
	ctxJobType     contextKey = "job_type"
	ctxProvider    contextKey = "provider"
	ctxPlatform    contextKey = "platform"
	ctxModule      contextKey = "module"
	ctxOperation   contextKey = "operation"
)

// WithRequestID stores request_id in context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxRequestID, strings.TrimSpace(id))
}

// RequestIDFromContext returns request_id.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxRequestID).(string); ok {
		return v
	}
	if v, ok := ctx.Value(ctxkey.TraceID).(string); ok {
		return v
	}
	return ""
}

// WithShopID stores shop_id in context (logs only, not metrics).
func WithShopID(ctx context.Context, shopID string) context.Context {
	return context.WithValue(ctx, ctxShopID, strings.TrimSpace(shopID))
}

// ShopIDFromContext returns shop_id.
func ShopIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxShopID).(string); ok {
		return v
	}
	return ""
}

// WithTaskID stores task_id in context.
func WithTaskID(ctx context.Context, taskID string) context.Context {
	return context.WithValue(ctx, ctxTaskID, strings.TrimSpace(taskID))
}

// TaskIDFromContext returns task_id.
func TaskIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxTaskID).(string); ok {
		return v
	}
	return ""
}

// WithExecutionID stores execution_id in context.
func WithExecutionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxExecutionID, strings.TrimSpace(id))
}

// ExecutionIDFromContext returns execution_id.
func ExecutionIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxExecutionID).(string); ok {
		return v
	}
	return ""
}

// WithJobType stores job_type in context.
func WithJobType(ctx context.Context, jobType string) context.Context {
	return context.WithValue(ctx, ctxJobType, strings.TrimSpace(jobType))
}

// JobTypeFromContext returns job_type.
func JobTypeFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxJobType).(string); ok {
		return v
	}
	return ""
}

// WithProvider stores provider in context.
func WithProvider(ctx context.Context, provider string) context.Context {
	return context.WithValue(ctx, ctxProvider, strings.TrimSpace(provider))
}

// ProviderFromContext returns provider.
func ProviderFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxProvider).(string); ok {
		return v
	}
	return ""
}

// WithPlatform stores platform in context.
func WithPlatform(ctx context.Context, platform string) context.Context {
	return context.WithValue(ctx, ctxPlatform, strings.TrimSpace(platform))
}

// PlatformFromContext returns platform.
func PlatformFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxPlatform).(string); ok {
		return v
	}
	return ""
}

// WithModule stores module in context.
func WithModule(ctx context.Context, module string) context.Context {
	return context.WithValue(ctx, ctxModule, strings.TrimSpace(module))
}

// ModuleFromContext returns module.
func ModuleFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxModule).(string); ok {
		return v
	}
	return ""
}

// WithOperation stores operation in context.
func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, ctxOperation, strings.TrimSpace(operation))
}

// OperationFromContext returns operation.
func OperationFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxOperation).(string); ok {
		return v
	}
	return ""
}

// TenantIDFromContext returns tenant_id from ctxkey.
func TenantIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxkey.TenantID).(string); ok {
		return v
	}
	if v, ok := ctx.Value(ctxkey.TenantID).(int64); ok {
		return fmt.Sprintf("%d", v)
	}
	return ""
}

// UserIDHashFromContext returns a safe user_id_hash for logs.
func UserIDHashFromContext(ctx context.Context) string {
	adminID, _ := ctx.Value(ctxkey.AdminID).(string)
	if adminID == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(adminID))
	return hex.EncodeToString(sum[:8])
}

// SessionIDHashFromContext returns a safe session_id_hash for logs.
func SessionIDHashFromContext(ctx context.Context) string {
	sid, _ := ctx.Value(ctxkey.SessionID).(string)
	if sid == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(sid))
	return hex.EncodeToString(sum[:8])
}

// TraceIDsFromContext returns trace_id and span_id from OTel span context.
func TraceIDsFromContext(ctx context.Context) (traceID, spanID string) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}

// CorrelationFields extracts standard correlation fields from context.
func CorrelationFields(ctx context.Context) []Field {
	fields := make([]Field, 0, 12)
	if rid := RequestIDFromContext(ctx); rid != "" {
		fields = append(fields, F("request_id", rid))
	}
	if tid, sid := TraceIDsFromContext(ctx); tid != "" {
		fields = append(fields, F("trace_id", tid))
		if sid != "" {
			fields = append(fields, F("span_id", sid))
		}
	} else if rid := RequestIDFromContext(ctx); rid != "" {
		fields = append(fields, F("trace_id", rid))
	}
	if t := TenantIDFromContext(ctx); t != "" {
		fields = append(fields, F("tenant_id", t))
	}
	if s := ShopIDFromContext(ctx); s != "" {
		fields = append(fields, F("shop_id", s))
	}
	if u := UserIDHashFromContext(ctx); u != "" {
		fields = append(fields, F("user_id_hash", u))
	}
	if sh := SessionIDHashFromContext(ctx); sh != "" {
		fields = append(fields, F("session_id_hash", sh))
	}
	if task := TaskIDFromContext(ctx); task != "" {
		fields = append(fields, F("task_id", task))
	}
	if ex := ExecutionIDFromContext(ctx); ex != "" {
		fields = append(fields, F("execution_id", ex))
	}
	if jt := JobTypeFromContext(ctx); jt != "" {
		fields = append(fields, F("job_type", jt))
	}
	if p := ProviderFromContext(ctx); p != "" {
		fields = append(fields, F("provider", p))
	}
	if pl := PlatformFromContext(ctx); pl != "" {
		fields = append(fields, F("platform", pl))
	}
	if m := ModuleFromContext(ctx); m != "" {
		fields = append(fields, F("module", m))
	}
	if op := OperationFromContext(ctx); op != "" {
		fields = append(fields, F("operation", op))
	}
	return fields
}
