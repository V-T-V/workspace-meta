package core

import "fmt"

// 错误定义（避免多处重复）。
type errMsg struct{ msg string }

func (e *errMsg) Error() string { return e.msg }

var (
	errHexOdd = &errMsg{"hex 字符串长度必须为偶数"}
	errPadLen = &errMsg{"PKCS#7 去填充失败：数据长度非 blockSize 倍数"}
	errPadVal = &errMsg{"PKCS#7 去填充失败：填充值非法"}
)

// errHexChar 返回"非法 hex 字符"错误。
func errHexChar(c byte) error {
	return &errMsg{fmt.Sprintf("非法 hex 字符 %q", string(c))}
}
