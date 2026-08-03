package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

type testClientError struct {
	status  int
	message string
	data    any
}

func (e *testClientError) Error() string       { return "private provider detail" }
func (e *testClientError) HTTPStatus() int     { return e.status }
func (e *testClientError) SafeMessage() string { return e.message }
func (e *testClientError) SafeData() any       { return e.data }

func TestHandleErrorMapsPublicErrorsAndPreservesTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   int
		wantMsg    string
	}{
		{name: "not found", err: gorm.ErrRecordNotFound, wantStatus: http.StatusNotFound, wantCode: CodeNotFound, wantMsg: "资源不存在"},
		{name: "forbidden", err: &testClientError{status: http.StatusForbidden, message: "没有操作权限", data: map[string]any{"errorCode": "DENIED"}}, wantStatus: http.StatusForbidden, wantCode: CodePermissionDenied, wantMsg: "没有操作权限"},
		{name: "bad gateway", err: &testClientError{status: http.StatusBadGateway, message: "上游拒绝"}, wantStatus: http.StatusBadGateway, wantCode: CodeBadGateway, wantMsg: "上游拒绝"},
		{name: "unavailable", err: &testClientError{status: http.StatusServiceUnavailable, message: "上游不可用"}, wantStatus: http.StatusServiceUnavailable, wantCode: CodeServiceUnavailable, wantMsg: "上游不可用"},
		{name: "internal", err: errors.New("sql password=secret"), wantStatus: http.StatusInternalServerError, wantCode: CodeInternalError, wantMsg: "internal error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set(ctxkey.TraceID, "trace-public-error")
			HandleError(c, tt.err)
			if w.Code != tt.wantStatus {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			var body Envelope
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Code != tt.wantCode || body.Message != tt.wantMsg || body.TraceID != "trace-public-error" {
				t.Fatalf("unexpected envelope: %+v", body)
			}
			if body.Message == "private provider detail" || body.Message == "sql password=secret" {
				t.Fatalf("private error leaked: %s", body.Message)
			}
		})
	}
}
