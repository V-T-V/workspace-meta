package core

import "fmt"

// Error 是编译器/解释器的统一错误类型，带源码位置。
type Error struct {
	Loc SourceLoc
	Msg string
}

// Error 实现 error 接口。
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Loc, e.Msg)
}

// NewError 在指定位置构造错误。
func NewError(loc SourceLoc, format string, args ...any) *Error {
	return &Error{Loc: loc, Msg: fmt.Sprintf(format, args...)}
}
