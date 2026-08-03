// Package core 提供 crypto-atlas 各算法包共享的底座：
// 字节/位运算辅助、十六进制编解码、XOR、PKCS#7 填充。
//
// 设计原则（对齐 consensus-atlas / go-agent-research）：
//   - 纯标准库，零外部依赖
//   - 各算法包只依赖本包，彼此互不 import
//   - 大数运算用标准库 math/big（RSA/DH 需要）
package core

// HexEncode 把字节切片转成十六进制小写字符串。
func HexEncode(data []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(data)*2)
	for i, b := range data {
		out[i*2] = hexdigits[b>>4]
		out[i*2+1] = hexdigits[b&0x0f]
	}
	return string(out)
}

// HexDecode 把十六进制字符串转回字节切片。
// 长度必须为偶数，字符必须是 0-9a-fA-F。
func HexDecode(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, errHexOdd
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		hi, err := hexVal(s[i])
		if err != nil {
			return nil, err
		}
		lo, err := hexVal(s[i+1])
		if err != nil {
			return nil, err
		}
		out[i/2] = hi<<4 | lo
	}
	return out, nil
}

func hexVal(c byte) (byte, error) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', nil
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, nil
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, nil
	}
	return 0, errHexChar(c)
}

// XorBytes 对 a/b 逐字节 XOR。长度取 min(len(a), len(b))。
func XorBytes(a, b []byte) []byte {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = a[i] ^ b[i]
	}
	return out
}

// PKCS7Pad 用 PKCS#7 规则填充 data 到 blockSize 的整数倍。
// 填充值 = 缺的字节数（1..blockSize），即使 data 已是 blockSize 倍数也补一整块。
// blockSize 必须为正，否则返回 nil（防除零 panic）。
func PKCS7Pad(data []byte, blockSize int) []byte {
	if blockSize <= 0 {
		return nil
	}
	padLen := blockSize - len(data)%blockSize
	pad := make([]byte, padLen)
	for i := range pad {
		pad[i] = byte(padLen)
	}
	return append(append([]byte{}, data...), pad...)
}

// PKCS7Unpad 去除 PKCS#7 填充。填充非法（值/长度不对）时返回 error。
// blockSize 必须为正，否则返回 error（防除零 panic）。
func PKCS7Unpad(data []byte, blockSize int) ([]byte, error) {
	if blockSize <= 0 {
		return nil, &errMsg{"PKCS#7 blockSize 必须为正"}
	}
	n := len(data)
	if n == 0 || n%blockSize != 0 {
		return nil, errPadLen
	}
	padLen := int(data[n-1])
	if padLen == 0 || padLen > blockSize || padLen > n {
		return nil, errPadVal
	}
	// 校验所有填充字节
	for i := n - padLen; i < n; i++ {
		if data[i] != byte(padLen) {
			return nil, errPadVal
		}
	}
	return data[:n-padLen], nil
}

// EncodeBase64 把字节编码为 base64 字符串。
func EncodeBase64(data []byte) string {
	return base64Encode(data)
}

// base64Encode 零依赖 base64 编码（标准字母表）。
func base64Encode(data []byte) string {
	const tbl = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result []byte
	for i := 0; i < len(data); i += 3 {
		b1 := data[i]
		var b2, b3 byte
		has2, has3 := false, false
		if i+1 < len(data) {
			b2 = data[i+1]
			has2 = true
		}
		if i+2 < len(data) {
			b3 = data[i+2]
			has3 = true
		}
		result = append(result, tbl[b1>>2])
		result = append(result, tbl[((b1&0x03)<<4)|(b2>>4)])
		if has2 {
			result = append(result, tbl[((b2&0x0f)<<2)|(b3>>6)])
		} else {
			result = append(result, '=')
		}
		if has3 {
			result = append(result, tbl[b3&0x3f])
		} else {
			result = append(result, '=')
		}
	}
	return string(result)
}
