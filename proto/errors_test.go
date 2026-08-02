package proto

import (
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	// errors.Is 能匹配 sentinel
	err := ErrStoreClosed
	if !errors.Is(err, ErrStoreClosed) {
		t.Error("errors.Is(ErrStoreClosed) 应为 true")
	}

	// 包装后仍能匹配
	wrapped := NewAPIError(503, ErrCodeUnavailable, "store unavailable", ErrStoreClosed)
	if !errors.Is(wrapped, ErrStoreClosed) {
		t.Error("包装后 errors.Is 仍应匹配 sentinel")
	}

	// 不匹配的 sentinel
	if errors.Is(err, ErrTaskNotFound) {
		t.Error("ErrStoreClosed 不应匹配 ErrTaskNotFound")
	}
}

func TestAPIError(t *testing.T) {
	cause := errors.New("connection reset")
	apiErr := NewAPIError(502, ErrCodeUnavailable, "上游不可用", cause)

	if apiErr.Status != 502 {
		t.Errorf("Status = %d", apiErr.Status)
	}
	if apiErr.Code != ErrCodeUnavailable {
		t.Errorf("Code = %s", apiErr.Code)
	}
	if !errors.Is(apiErr, cause) {
		t.Error("Unwrap 应返回原始 cause")
	}

	// Error() 消息含 cause
	msg := apiErr.Error()
	if msg == "" || msg == "上游不可用" {
		t.Errorf("Error() 应含 cause: %s", msg)
	}
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name   string
		err    *APIError
		status int
		code   string
	}{
		{"NotFound", ErrNotFound("任务不存在"), 404, ErrCodeNotFound},
		{"Forbidden", ErrForbidden("策略拦截"), 403, ErrCodeForbidden},
		{"Unauthorized", ErrUnauthorized("无效 token"), 401, ErrCodeUnauthorized},
		{"Conflict", ErrConflict("状态冲突"), 409, ErrCodeConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Status != tt.status {
				t.Errorf("Status = %d, 期望 %d", tt.err.Status, tt.status)
			}
			if tt.err.Code != tt.code {
				t.Errorf("Code = %s, 期望 %s", tt.err.Code, tt.code)
			}
		})
	}
}
