package core

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// ===== HexEncode / HexDecode =====

// TestHexRoundTrip 对各种字节做 HexEncode -> HexDecode 往返，并与标准库对比。
func TestHexRoundTrip(t *testing.T) {
	cases := [][]byte{
		nil,
		{},
		{0x00},
		{0xff},
		{0x0f, 0xf0},
		{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0},
		bytes.Repeat([]byte{0xab}, 32),
		{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
	}
	for i, in := range cases {
		got := HexEncode(in)
		// 与标准库对比（含大小写）
		if want := hex.EncodeToString(in); got != want {
			t.Errorf("case %d: HexEncode(%v) = %q, want %q", i, in, got, want)
		}
		// 往返：解码回来必须等值
		dec, err := HexDecode(got)
		if err != nil {
			t.Errorf("case %d: HexDecode(%q) 出错: %v", i, got, err)
			continue
		}
		if !bytes.Equal(dec, in) {
			t.Errorf("case %d: 往返不等 got %v want %v", i, dec, in)
		}
	}
}

// TestHexDecodeAcceptsUppercase 大写 hex 字符也应被接受（与标准库一致）。
func TestHexDecodeAcceptsUppercase(t *testing.T) {
	dec, err := HexDecode("DEADBEEF")
	if err != nil {
		t.Fatalf("大写 hex 解码出错: %v", err)
	}
	if !bytes.Equal(dec, []byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Errorf("大写解码结果错 got %v", dec)
	}
}

// TestHexDecodeOddLength 奇数长度必须报错。
func TestHexDecodeOddLength(t *testing.T) {
	if _, err := HexDecode("abc"); err == nil {
		t.Error("奇数长度应报错，实际 nil")
	}
	if _, err := HexDecode("a"); err == nil {
		t.Error("单字符应报错")
	}
}

// TestHexDecodeInvalidChar 非法字符必须报错。
func TestHexDecodeInvalidChar(t *testing.T) {
	bad := []string{"gg", "12g4", "xy", "0/2", "z0"}
	for _, s := range bad {
		if _, err := HexDecode(s); err == nil {
			t.Errorf("HexDecode(%q) 应报错（含非法字符），实际 nil", s)
		}
	}
}

// ===== XorBytes =====

func TestXorBytes(t *testing.T) {
	a := []byte{0xff, 0x0f, 0x55, 0xaa}
	b := []byte{0x00, 0xf0, 0x55, 0xff}
	want := []byte{0xff, 0xff, 0x00, 0x55}
	got := XorBytes(a, b)
	if !bytes.Equal(got, want) {
		t.Errorf("等长 XOR 错 got %v want %v", got, want)
	}
}

// TestXorBytesSelfXorZero 自身与自身 XOR 结果应全 0。
func TestXorBytesSelfXorZero(t *testing.T) {
	a := []byte{1, 2, 3, 4, 250}
	got := XorBytes(a, a)
	if !bytes.Equal(got, make([]byte, len(a))) {
		t.Errorf("a XOR a 应全 0，got %v", got)
	}
}

// TestXorBytesUnequalLength 不等长取 min，多余字节忽略。
func TestXorBytesUnequalLength(t *testing.T) {
	a := []byte{0x01, 0x02, 0x03, 0x04}
	b := []byte{0x10}
	got := XorBytes(a, b)
	if len(got) != 1 {
		t.Fatalf("不等长应取 min=1，实际 len=%d", len(got))
	}
	if got[0] != 0x01^0x10 {
		t.Errorf("首字节错 got %x", got[0])
	}
	// 反过来也应一致
	got2 := XorBytes(b, a)
	if !bytes.Equal(got2, got) {
		t.Errorf("XOR 不对称? %v vs %v", got2, got)
	}
}

// TestXorBytesEmpty 空 / nil 输入应返回空。
func TestXorBytesEmpty(t *testing.T) {
	if got := XorBytes(nil, nil); len(got) != 0 {
		t.Errorf("nil^nil 应为空，got %v", got)
	}
	if got := XorBytes([]byte{}, []byte{1, 2}); len(got) != 0 {
		t.Errorf("空^非空 应为空，got %v", got)
	}
}

// ===== PKCS7Pad =====

// TestPKCS7PadNormal 未对齐输入：补到 blockSize 倍数，填充值 = 缺的字节数。
func TestPKCS7PadNormal(t *testing.T) {
	const bs = 16
	in := []byte("hello") // 5 字节，应补 11 个 0x0b
	got := PKCS7Pad(in, bs)
	if len(got) != bs {
		t.Fatalf("补后长度应为 %d，实际 %d", bs, len(got))
	}
	if !bytes.Equal(got[:len(in)], in) {
		t.Errorf("原始数据被破坏")
	}
	pad := got[len(in):]
	for _, p := range pad {
		if p != byte(bs-len(in)) {
			t.Errorf("填充字节应为 0x%x，got 0x%x", bs-len(in), p)
		}
	}
}

// TestPKCS7PadAlignedBlock 已对齐输入：必须补一整块（值 = blockSize）。
func TestPKCS7PadAlignedBlock(t *testing.T) {
	const bs = 8
	in := bytes.Repeat([]byte{0xaa}, bs) // 已是 blockSize 倍数
	got := PKCS7Pad(in, bs)
	if len(got) != bs*2 {
		t.Fatalf("已对齐应补整块，期望长度 %d，实际 %d", bs*2, len(got))
	}
	for i := bs; i < bs*2; i++ {
		if got[i] != byte(bs) {
			t.Errorf("整块填充值应为 0x%x，got 0x%x (idx %d)", bs, got[i], i)
		}
	}
}

// TestPKCS7PadEmpty 空输入：补一整块（值 = blockSize）。
func TestPKCS7PadEmpty(t *testing.T) {
	const bs = 16
	got := PKCS7Pad(nil, bs)
	if len(got) != bs {
		t.Fatalf("空输入应补整块（%d 字节），实际 %d", bs, len(got))
	}
	for _, p := range got {
		if p != byte(bs) {
			t.Errorf("空输入填充值应为 0x%x，got 0x%x", bs, p)
		}
	}
	// 同样测试显式空切片
	got2 := PKCS7Pad([]byte{}, bs)
	if !bytes.Equal(got2, got) {
		t.Errorf("nil 与 []byte{} 结果应一致")
	}
}

// TestPKCS7PadDoesNotMutateInput 填充不应修改原输入切片。
func TestPKCS7PadDoesNotMutateInput(t *testing.T) {
	in := []byte{1, 2, 3, 4}
	snapshot := append([]byte{}, in...)
	_ = PKCS7Pad(in, 8)
	if !bytes.Equal(in, snapshot) {
		t.Errorf("PKCS7Pad 修改了原输入: got %v want %v", in, snapshot)
	}
}

// ===== PKCS7Unpad 往返 + 错误 =====

// TestPKCS7UnpadRoundTrip 各种长度/blockSize 的填充-去填充往返。
func TestPKCS7UnpadRoundTrip(t *testing.T) {
	cases := []struct {
		name      string
		data      []byte
		blockSize int
	}{
		{"空/bs16", []byte{}, 16},
		{"1字节/bs16", []byte{0x01}, 16},
		{"已对齐/bs16", bytes.Repeat([]byte{0x42}, 16), 16},
		{"15字节/bs16", bytes.Repeat([]byte{0x42}, 15), 16},
		{"8字节/bs8", bytes.Repeat([]byte{0x99}, 8), 8},
		{"5字节/bs8", []byte("abcde"), 8},
	}
	for _, c := range cases {
		padded := PKCS7Pad(c.data, c.blockSize)
		got, err := PKCS7Unpad(padded, c.blockSize)
		if err != nil {
			t.Errorf("%s: Unpad 出错: %v", c.name, err)
			continue
		}
		if !bytes.Equal(got, c.data) {
			t.Errorf("%s: 往返不等 got %v want %v", c.name, got, c.data)
		}
	}
}

// TestPKCS7UnpadInvalidPadValue 填充值非法（不在 1..blockSize）必须报错。
func TestPKCS7UnpadInvalidPadValue(t *testing.T) {
	const bs = 8
	// 最后一字节 0x00（非法）
	zeroPad := append(bytes.Repeat([]byte{0xaa}, bs-1), 0x00)
	if _, err := PKCS7Unpad(zeroPad, bs); err == nil {
		t.Error("填充值 0x00 应报错")
	}
	// 最后一字节 = bs+1（超出范围）
	bigPad := append(bytes.Repeat([]byte{0xaa}, bs-1), byte(bs+1))
	if _, err := PKCS7Unpad(bigPad, bs); err == nil {
		t.Errorf("填充值 0x%x（>blockSize）应报错", bs+1)
	}
}

// TestPKCS7UnpadInconsistentPad 填充字节不一致必须报错（防 padding oracle 类输入）。
func TestPKCS7UnpadInconsistentPad(t *testing.T) {
	const bs = 8
	// 末字节说补 3，但前两个填充字节不是 3
	bad := []byte{0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0x01, 0x02, 0x03}
	if _, err := PKCS7Unpad(bad, bs); err == nil {
		t.Error("填充字节不一致应报错")
	}
}

// TestPKCS7UnpadWrongLength 长度非 blockSize 倍数必须报错。
func TestPKCS7UnpadWrongLength(t *testing.T) {
	const bs = 8
	if _, err := PKCS7Unpad(bytes.Repeat([]byte{0x01}, bs-1), bs); err == nil {
		t.Error("长度非 blockSize 倍数应报错")
	}
	// 空输入也应报错
	if _, err := PKCS7Unpad(nil, bs); err == nil {
		t.Error("空输入 Unpad 应报错")
	}
}

// TestPKCS7UnpadValidBoundary 校验一个合法填充能正确去填充，验证边界值对齐。
func TestPKCS7UnpadValidBoundary(t *testing.T) {
	const bs = 4
	// 4 字节，末字节为 0x04 = 整块填充，去填充后应为空
	full := []byte{0x04, 0x04, 0x04, 0x04}
	got, err := PKCS7Unpad(full, bs)
	if err != nil {
		t.Fatalf("整块填充应可去填充，出错: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("整块填充去填充后应为空，got %v", got)
	}
}

// TestErrorsContainMessage 错误信息非空（确保用户能看到失败原因）。
func TestErrorsContainMessage(t *testing.T) {
	_, err1 := HexDecode("x")
	if err1 == nil || strings.TrimSpace(err1.Error()) == "" {
		t.Error("HexDecode 错误信息应为非空")
	}
	_, err2 := PKCS7Unpad([]byte{0x00}, 8)
	if err2 == nil || strings.TrimSpace(err2.Error()) == "" {
		t.Error("PKCS7Unpad 错误信息应为非空")
	}
}

func TestPKCS7PadZeroBlockSize(t *testing.T) {
	// blockSize=0 应返回 nil 而非 panic（除零防护）
	if got := PKCS7Pad([]byte("x"), 0); got != nil {
		t.Errorf("PKCS7Pad(blockSize=0) 应返回 nil，实际 %v", got)
	}
	if _, err := PKCS7Unpad([]byte("x"), 0); err == nil {
		t.Error("PKCS7Unpad(blockSize=0) 应返回 error")
	}
	if _, err := PKCS7Unpad([]byte("x"), -1); err == nil {
		t.Error("PKCS7Unpad(blockSize=-1) 应返回 error")
	}
}
