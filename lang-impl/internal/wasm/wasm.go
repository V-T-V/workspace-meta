// Package wasm 把 M 语言的 AST 编译成 WebAssembly 字节码。
//
// 这是 lang-impl 的 M2 后端：除了树遍历解释器外，还能把程序编译成 .wasm
// 二进制模块，产出的 wasm 模块可以被 Node.js / 浏览器 / wasmtime 执行。
//
// M1 支持的子集（教学向，逐步扩展）：
//   - 整数算术表达式：1 + 2 * 3、(1+2)*3、x - 1
//   - 变量引用（let 绑定的 i32）
//   - 函数定义 + 调用（如 fn fib(n) { ... }）
//   - if/while/return 语句
//
// WASM 二进制格式（核心 sections）：
//
//	magic + version
//	Type section（函数签名）
//	Function section（函数索引）
//	Export section（导出函数名）
//	Code section（函数体字节码）
//
// 参考：https://webassembly.github.io/spec/core/binary/
package wasm

import (
	"fmt"

	"github.com/QiuShichang/lang-impl/internal/core"
)

// Module 是一个 WASM 模块的构建器。
type Module struct {
	types   []wasmFuncType
	funcs   []wasmFuncRef // 每个 function 的 type 索引
	exports []exportEntry
	codes   []codeEntry // 每个 function 的本地变量 + 字节码

	// 函数名 → 索引（用于调用）
	funcIndices map[string]uint32
}

// wasmFuncType 是 WASM 函数签名（参数/返回值类型）。
type wasmFuncType struct {
	params  []byte // valtype：0x7F=i32, 0x7E=i64, 0x7D=f32, 0x7C=f64
	results []byte
}

// wasmFuncRef 是 function section 的一条（type 索引）。
type wasmFuncRef struct {
	typeIdx uint32
}

// exportEntry 是 export section 的一条。
type exportEntry struct {
	name string
	kind byte // 0x00=func, 0x01=table, 0x02=mem, 0x03=global
	idx  uint32
}

// codeEntry 是 code section 的一条（函数体）。
type codeEntry struct {
	locals []localEntry
	body   []byte // 字节码（以 0x0B end 结尾）
}

// localEntry 声明函数的本地变量。
type localEntry struct {
	count uint32
	typ   byte
}

// 编译器上下文（编译单个函数时用）。
type funcCompiler struct {
	mod      *Module
	locals   map[string]uint32 // 变量名 → local 索引
	localIdx uint32            // 下一个分配的 local 索引
	body     []byte            // 累积的字节码
}

// NewModule 创建空模块。
func NewModule() *Module {
	return &Module{funcIndices: map[string]uint32{}}
}

// WASM 字节码常量（指令）。
const (
	opUnreachable = 0x00
	opBlock       = 0x02
	opLoop        = 0x03
	opIf          = 0x04
	opElse        = 0x05
	opEnd         = 0x0B
	opReturn      = 0x0F
	opCall        = 0x10
	opLocalGet    = 0x20
	opLocalSet    = 0x21
	opI32Const    = 0x41
	opI32Add      = 0x6A
	opI32Sub      = 0x6B
	opI32Mul      = 0x6C
	opI32DivS     = 0x6D
	opI32RemS     = 0x70
	opI32LtS      = 0x48
	opI32LeS      = 0x4C
	opI32GtS      = 0x4A
	opI32GeS      = 0x4E
	opI32Eq       = 0x46
	opI32Ne       = 0x47

	valI32 = 0x7F
)

// Compile 把整个 Program 编译成 WASM 模块。
// 目前支持：函数定义（fn）、let 绑定、return、if、表达式。
func Compile(prog *core.Program) (*Module, error) {
	mod := NewModule()
	for _, stmt := range prog.Stmts {
		fn, ok := stmt.(*core.FnDecl)
		if !ok {
			continue // 顶层非函数声明暂跳过（M2 逐步支持）
		}
		if err := mod.compileFunc(fn); err != nil {
			return nil, err
		}
	}
	return mod, nil
}

// compileFunc 编译一个函数定义。
func (m *Module) compileFunc(fn *core.FnDecl) error {
	// 构造函数签名
	var params []byte
	for range fn.Params {
		params = append(params, valI32) // 所有参数都是 i32
	}
	results := []byte{valI32} // 所有函数返回 i32
	typeIdx := uint32(len(m.types))
	m.types = append(m.types, wasmFuncType{params: params, results: results})

	funcIdx := uint32(len(m.funcs))
	m.funcs = append(m.funcs, wasmFuncRef{typeIdx: typeIdx})
	m.funcIndices[fn.Name] = funcIdx

	// 编译函数体
	fc := &funcCompiler{
		mod:    m,
		locals: map[string]uint32{},
	}
	// 参数作为前几个 local
	for i, p := range fn.Params {
		fc.locals[p] = uint32(i)
	}
	fc.localIdx = uint32(len(fn.Params))

	// 编译 body
	if fn.Body != nil {
		for _, s := range fn.Body.Stmts {
			if err := fc.compileStmt(s); err != nil {
				return err
			}
		}
	}
	fc.body = append(fc.body, opEnd) // 函数体以 end 结尾

	// 存入 code section
	m.codes = append(m.codes, codeEntry{body: fc.body})
	// 导出函数
	m.exports = append(m.exports, exportEntry{name: fn.Name, kind: 0x00, idx: funcIdx})
	return nil
}

// compileStmt 编译一条语句。
func (fc *funcCompiler) compileStmt(stmt core.Stmt) error {
	switch s := stmt.(type) {
	case *core.ReturnStmt:
		if s.Value != nil {
			if err := fc.compileExpr(s.Value); err != nil {
				return err
			}
		} else {
			fc.body = append(fc.body, opI32Const, 0)
		}
		fc.body = append(fc.body, opReturn)
	case *core.LetStmt:
		if err := fc.compileExpr(s.Init); err != nil {
			return err
		}
		idx, ok := fc.locals[s.Name]
		if !ok {
			idx = fc.localIdx
			fc.locals[s.Name] = idx
			fc.localIdx++
		}
		fc.emitLocalSet(idx)
	case *core.ExprStmt:
		// 表达式语句：编译后丢弃结果（drop）
		if err := fc.compileExpr(s.Expr); err != nil {
			return err
		}
		fc.body = append(fc.body, 0x1A) // opDrop
	case *core.IfStmt:
		if err := fc.compileExpr(s.Cond); err != nil {
			return err
		}
		fc.body = append(fc.body, opIf, valI32) // if 返回 i32
		if s.Then != nil {
			for _, st := range s.Then.Stmts {
				if err := fc.compileStmt(st); err != nil {
					return err
				}
			}
		}
		if s.Else != nil {
			fc.body = append(fc.body, opElse)
			for _, st := range s.Else.Stmts {
				if err := fc.compileStmt(st); err != nil {
					return err
				}
			}
		}
		fc.body = append(fc.body, opEnd)
	default:
		// 其他语句类型暂不支持
	}
	return nil
}

// compileExpr 编译一个表达式（结果留在栈顶）。
func (fc *funcCompiler) compileExpr(expr core.Expr) error {
	switch e := expr.(type) {
	case *core.NumberExpr:
		fc.emitI32Const(e.Value)
	case *core.IdentExpr:
		idx, ok := fc.locals[e.Name]
		if !ok {
			return fmt.Errorf("未定义变量 %q", e.Name)
		}
		fc.emitLocalGet(idx)
	case *core.BinaryExpr:
		if err := fc.compileExpr(e.Left); err != nil {
			return err
		}
		if err := fc.compileExpr(e.Right); err != nil {
			return err
		}
		fc.emitBinaryOp(e.Op)
	case *core.UnaryExpr:
		if e.Op == core.TokMinus {
			fc.emitI32Const(0) // 0 - x = -x
			if err := fc.compileExpr(e.Right); err != nil {
				return err
			}
			fc.body = append(fc.body, opI32Sub)
		}
	case *core.CallExpr:
		idx, ok := fc.mod.funcIndices[e.Callee]
		if !ok {
			return fmt.Errorf("未定义函数 %q", e.Callee)
		}
		for _, arg := range e.Args {
			if err := fc.compileExpr(arg); err != nil {
				return err
			}
		}
		fc.emitU32(opCall, idx)
	default:
		return fmt.Errorf("WASM 后端暂不支持的表达式类型 %T", e)
	}
	return nil
}

// ===== 字节码发射辅助 =====

func (fc *funcCompiler) emitI32Const(v int64) {
	fc.body = append(fc.body, opI32Const)
	fc.body = appendLEB128(fc.body, v)
}

func (fc *funcCompiler) emitLocalGet(idx uint32) {
	fc.body = append(fc.body, opLocalGet)
	fc.body = appendU32LEB(fc.body, idx)
}

func (fc *funcCompiler) emitLocalSet(idx uint32) {
	fc.body = append(fc.body, opLocalSet)
	fc.body = appendU32LEB(fc.body, idx)
}

func (fc *funcCompiler) emitU32(op byte, v uint32) {
	fc.body = append(fc.body, op)
	fc.body = appendU32LEB(fc.body, v)
}

func (fc *funcCompiler) emitBinaryOp(op core.TokenType) {
	switch op {
	case core.TokPlus:
		fc.body = append(fc.body, opI32Add)
	case core.TokMinus:
		fc.body = append(fc.body, opI32Sub)
	case core.TokStar:
		fc.body = append(fc.body, opI32Mul)
	case core.TokSlash:
		fc.body = append(fc.body, opI32DivS)
	case core.TokPercent:
		fc.body = append(fc.body, opI32RemS)
	case core.TokLT:
		fc.body = append(fc.body, opI32LtS)
	case core.TokLE:
		fc.body = append(fc.body, opI32LeS)
	case core.TokGT:
		fc.body = append(fc.body, opI32GtS)
	case core.TokGE:
		fc.body = append(fc.body, opI32GeS)
	case core.TokEQ:
		fc.body = append(fc.body, opI32Eq)
	case core.TokNE:
		fc.body = append(fc.body, opI32Ne)
	}
}

// ===== LEB128 编码（WASM 变长整数）=====

// appendLEB128 追加有符号 LEB128（用于 i32.const 的立即数）。
func appendLEB128(buf []byte, v int64) []byte {
	more := true
	for more {
		b := byte(v & 0x7F)
		v >>= 7
		if (v == 0 && b&0x40 == 0) || (v == -1 && b&0x40 != 0) {
			more = false
		} else {
			b |= 0x80
		}
		buf = append(buf, b)
	}
	return buf
}

// appendU32LEB 追加无符号 LEB128。
func appendU32LEB(buf []byte, v uint32) []byte {
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if v == 0 {
			break
		}
	}
	return buf
}

// ===== 二进制序列化 =====

// Bytes 把模块序列化成 WASM 二进制格式（.wasm 文件内容）。
func (m *Module) Bytes() []byte {
	var buf []byte
	// Magic + Version
	buf = append(buf, 0x00, 0x61, 0x73, 0x6D) // \0asm
	buf = append(buf, 0x01, 0x00, 0x00, 0x00) // version 1

	// Type section (id=1)
	buf = appendSection(buf, 1, func(s []byte) []byte {
		s = appendU32LEB(s, uint32(len(m.types)))
		for _, t := range m.types {
			s = append(s, 0x60) // func type
			s = appendU32LEB(s, uint32(len(t.params)))
			s = append(s, t.params...)
			s = appendU32LEB(s, uint32(len(t.results)))
			s = append(s, t.results...)
		}
		return s
	})

	// Function section (id=3)
	buf = appendSection(buf, 3, func(s []byte) []byte {
		s = appendU32LEB(s, uint32(len(m.funcs)))
		for _, f := range m.funcs {
			s = appendU32LEB(s, f.typeIdx)
		}
		return s
	})

	// Export section (id=7)
	buf = appendSection(buf, 7, func(s []byte) []byte {
		s = appendU32LEB(s, uint32(len(m.exports)))
		for _, e := range m.exports {
			s = appendU32LEB(s, uint32(len(e.name)))
			s = append(s, []byte(e.name)...)
			s = append(s, e.kind)
			s = appendU32LEB(s, e.idx)
		}
		return s
	})

	// Code section (id=10)
	buf = appendSection(buf, 10, func(s []byte) []byte {
		s = appendU32LEB(s, uint32(len(m.codes)))
		for _, c := range m.codes {
			// 每个 code entry：size + locals_count + locals + body
			var entry []byte
			entry = appendU32LEB(entry, uint32(len(c.locals)))
			for _, l := range c.locals {
				entry = appendU32LEB(entry, l.count)
				entry = append(entry, l.typ)
			}
			entry = append(entry, c.body...)
			s = appendU32LEB(s, uint32(len(entry)))
			s = append(s, entry...)
		}
		return s
	})

	return buf
}

// appendSection 追加一个 section（id + 内容）。
func appendSection(buf []byte, id byte, build func([]byte) []byte) []byte {
	content := build(nil)
	buf = append(buf, id)
	buf = appendU32LEB(buf, uint32(len(content)))
	buf = append(buf, content...)
	return buf
}

// HexString 返回模块的十六进制表示（调试用）。
func (m *Module) HexString() string {
	bytes := m.Bytes()
	const hex = "0123456789abcdef"
	out := make([]byte, len(bytes)*2)
	for i, b := range bytes {
		out[i*2] = hex[b>>4]
		out[i*2+1] = hex[b&0x0f]
	}
	return string(out)
}

// FunctionCount 返回模块中的函数数。
func (m *Module) FunctionCount() int { return len(m.funcs) }

// ExportedFunctions 返回导出的函数名列表。
func (m *Module) ExportedFunctions() []string {
	out := make([]string, 0, len(m.exports))
	for _, e := range m.exports {
		out = append(out, e.name)
	}
	return out
}
