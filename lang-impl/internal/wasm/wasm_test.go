package wasm

import (
	"testing"

	"github.com/QiuShichang/lang-impl/internal/lexer"
	"github.com/QiuShichang/lang-impl/internal/parser"
)

// compileSrc 是测试辅助：源码 → AST → WASM 模块。
func compileSrc(t *testing.T, src string) *Module {
	t.Helper()
	tokens, err := lexer.Tokenize(src)
	if err != nil {
		t.Fatalf("lex 失败: %v", err)
	}
	prog, err := parser.Parse(tokens)
	if err != nil {
		t.Fatalf("parse 失败: %v", err)
	}
	mod, err := Compile(prog)
	if err != nil {
		t.Fatalf("compile 失败: %v", err)
	}
	return mod
}

func TestWasmMagicAndVersion(t *testing.T) {
	mod := compileSrc(t, `fn f() { return 1; }`)
	bytes := mod.Bytes()
	// WASM magic: \0asm
	if bytes[0] != 0x00 || bytes[1] != 0x61 || bytes[2] != 0x73 || bytes[3] != 0x6D {
		t.Error("magic 不对")
	}
	// Version: 1
	if bytes[4] != 0x01 || bytes[5] != 0x00 || bytes[6] != 0x00 || bytes[7] != 0x00 {
		t.Error("version 不对")
	}
}

func TestSimpleFunction(t *testing.T) {
	mod := compileSrc(t, `fn fortyTwo() { return 42; }`)
	if mod.FunctionCount() != 1 {
		t.Errorf("应有 1 个函数，实际 %d", mod.FunctionCount())
	}
	exports := mod.ExportedFunctions()
	if len(exports) != 1 || exports[0] != "fortyTwo" {
		t.Errorf("应导出 fortyTwo，实际 %v", exports)
	}
}

func TestArithmetic(t *testing.T) {
	// fn add(a, b) { return a + b; }
	mod := compileSrc(t, `fn add(a, b) { return a + b; }`)
	bytes := mod.Bytes()
	if len(bytes) < 8 {
		t.Error("wasm 模块应非空")
	}
	// 应含 i32.add 指令 (0x6A)
	hasAdd := false
	for _, b := range bytes {
		if b == opI32Add {
			hasAdd = true
			break
		}
	}
	if !hasAdd {
		t.Error("应含 i32.add 指令")
	}
}

func TestMul(t *testing.T) {
	mod := compileSrc(t, `fn m(a, b) { return a * b; }`)
	bytes := mod.Bytes()
	hasMul := false
	for _, b := range bytes {
		if b == opI32Mul {
			hasMul = true
			break
		}
	}
	if !hasMul {
		t.Error("应含 i32.mul 指令")
	}
}

func TestComparison(t *testing.T) {
	mod := compileSrc(t, `fn lt(a, b) { return a < b; }`)
	bytes := mod.Bytes()
	hasLt := false
	for _, b := range bytes {
		if b == opI32LtS {
			hasLt = true
			break
		}
	}
	if !hasLt {
		t.Error("应含 i32.lt_s 指令")
	}
}

func TestFunctionCall(t *testing.T) {
	src := `fn double(x) { return x * 2; }
fn quad(x) { return double(double(x)); }`
	mod := compileSrc(t, src)
	if mod.FunctionCount() != 2 {
		t.Errorf("应有 2 个函数，实际 %d", mod.FunctionCount())
	}
	// 应含 call 指令 (0x10)
	bytes := mod.Bytes()
	hasCall := false
	for _, b := range bytes {
		if b == opCall {
			hasCall = true
			break
		}
	}
	if !hasCall {
		t.Error("应含 call 指令")
	}
}

func TestIfStatement(t *testing.T) {
	src := `fn sign(x) { if (x < 0) { return -1; } return 1; }`
	mod := compileSrc(t, src)
	bytes := mod.Bytes()
	hasIf := false
	for _, b := range bytes {
		if b == opIf {
			hasIf = true
			break
		}
	}
	if !hasIf {
		t.Error("应含 if 指令")
	}
}

func TestLetBinding(t *testing.T) {
	src := `fn f() { let x = 10; return x; }`
	mod := compileSrc(t, src)
	bytes := mod.Bytes()
	// 应含 local.set 和 local.get
	hasSet, hasGet := false, false
	for _, b := range bytes {
		if b == opLocalSet {
			hasSet = true
		}
		if b == opLocalGet {
			hasGet = true
		}
	}
	if !hasSet {
		t.Error("应含 local.set 指令")
	}
	if !hasGet {
		t.Error("应含 local.get 指令")
	}
}

func TestHexString(t *testing.T) {
	mod := compileSrc(t, `fn f() { return 1; }`)
	hex := mod.HexString()
	if len(hex) < 16 {
		t.Error("hex 字符串应至少 16 字符（8 字节 magic+version）")
	}
	// 开头应是 "0061736d"（\0asm）
	if hex[:8] != "0061736d" {
		t.Errorf("hex 开头应是 0061736d，实际 %s", hex[:8])
	}
}

func TestLEB128(t *testing.T) {
	// 测试 LEB128 编码
	cases := []struct {
		input  int64
		expect []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{63, []byte{0x3F}},        // 6 位正数上限（1 字节）
		{64, []byte{0xC0, 0x00}},  // 7 位需 2 字节（符号位）
		{127, []byte{0xFF, 0x00}}, // 127 有符号需 2 字节
		{-1, []byte{0x7F}},        // -1 是 0x7F（1 字节）
	}
	for _, c := range cases {
		got := appendLEB128(nil, c.input)
		if len(got) != len(c.expect) {
			t.Errorf("LEB128(%d) 长度 %d，期望 %d", c.input, len(got), len(c.expect))
			continue
		}
		for i := range got {
			if got[i] != c.expect[i] {
				t.Errorf("LEB128(%d)[%d] = %#x，期望 %#x", c.input, i, got[i], c.expect[i])
			}
		}
	}
}

func TestU32LEB(t *testing.T) {
	// 测试无符号 LEB128
	cases := []struct {
		input  uint32
		expect []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{127, []byte{0x7F}},
		{128, []byte{0x80, 0x01}},
	}
	for _, c := range cases {
		got := appendU32LEB(nil, c.input)
		if len(got) != len(c.expect) {
			t.Errorf("U32LEB(%d) 长度 %d，期望 %d", c.input, len(got), len(c.expect))
			continue
		}
		for i := range got {
			if got[i] != c.expect[i] {
				t.Errorf("U32LEB(%d)[%d] = %#x，期望 %#x", c.input, i, got[i], c.expect[i])
			}
		}
	}
}

func TestMultipleFunctions(t *testing.T) {
	src := `fn a() { return 1; }
fn b() { return 2; }
fn c() { return 3; }`
	mod := compileSrc(t, src)
	if mod.FunctionCount() != 3 {
		t.Errorf("应有 3 个函数，实际 %d", mod.FunctionCount())
	}
	if len(mod.ExportedFunctions()) != 3 {
		t.Error("应导出 3 个函数")
	}
}
