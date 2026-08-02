// Package proto — 统一错误体系(M3 神级提升)。
//
// 设计:
//   - sentinel error 供 errors.Is 判断(替代字符串匹配)
//   - 所有错误用 fmt.Errorf("%w", err) 包装(保留链)
//   - 用户友好消息(中文)+ 机器可读 code
package proto

import "errors"

// 错误码常量(机器可读)。
const (
	ErrCodeUnknown     = "UNKNOWN"
	ErrCodeNotFound    = "NOT_FOUND"
	ErrCodeUnauthorized = "UNAUTHORIZED"
	ErrCodeForbidden   = "FORBIDDEN"
	ErrCodeConflict    = "CONFLICT"  // 状态机冲突
	ErrCodeTimeout     = "TIMEOUT"
	ErrCodeUnavailable = "UNAVAILABLE"
)

// sentinel errors(relay/agent/cli 共用,errors.Is 判断)。
var (
	// ErrStoreClosed 存储已关闭。
	ErrStoreClosed = errors.New("store closed")
	// ErrTaskNotFound 任务不存在。
	ErrTaskNotFound = errors.New("task not found")
	// ErrAgentOffline Agent 不在线。
	ErrAgentOffline = errors.New("agent offline")
	// ErrAgentNotFound Agent 不存在。
	ErrAgentNotFound = errors.New("agent not found")
	// ErrInvalidTransition 非法状态转换(状态机冲突)。
	ErrInvalidTransition = errors.New("invalid state transition")
	// ErrPolicyBlocked 命令/路径策略拦截。
	ErrPolicyBlocked = errors.New("blocked by policy")
	// ErrVersionMismatch 协议版本不匹配。
	ErrVersionMismatch = errors.New("protocol version mismatch")
	// ErrCapabilityNotSupported Agent 不支持该任务类型。
	ErrCapabilityNotSupported = errors.New("capability not supported")
)

// APIError API 层错误(含 HTTP status + code + 用户消息)。
type APIError struct {
	Status  int    `json:"-"`              // HTTP status code
	Code    string `json:"code"`           // 机器可读错误码
	Message string `json:"message"`        // 用户可读消息
	Cause   error  `json:"-"`              // 原始错误
}

func (e *APIError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *APIError) Unwrap() error { return e.Cause }

// NewAPIError 构造 APIError。
func NewAPIError(status int, code, message string, cause error) *APIError {
	return &APIError{Status: status, Code: code, Message: message, Cause: cause}
}

// 常用 APIError 构造器。
func ErrNotFound(msg string) *APIError {
	return NewAPIError(404, ErrCodeNotFound, msg, ErrTaskNotFound)
}

func ErrForbidden(msg string) *APIError {
	return NewAPIError(403, ErrCodeForbidden, msg, ErrPolicyBlocked)
}

func ErrUnauthorized(msg string) *APIError {
	return NewAPIError(401, ErrCodeUnauthorized, msg, nil)
}

func ErrConflict(msg string) *APIError {
	return NewAPIError(409, ErrCodeConflict, msg, ErrInvalidTransition)
}
