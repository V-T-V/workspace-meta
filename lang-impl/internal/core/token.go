// Package core 提供 lang-impl 的"M 语言"编译器共享底座：
// Token（词法单元）、SourceLoc（源码位置）、Error（统一错误）、AST 节点定义。
//
// M 语言是一门教学玩具语言，设计目标：小而完整，覆盖编译器核心概念。
//
// 语法示例：
//
//	fn fib(n) {
//	  if (n < 2) { return n; }
//	  return fib(n - 1) + fib(n - 2);
//	}
//	let r = fib(10);
//
// 类型：i32（整数）/ bool / string（动态类型，运行时按值判断）。
package core

// SourceLoc 标记源码位置（1-based 行列），用于错误信息定位。
type SourceLoc struct {
	Line   int
	Column int
}

// String 返回 "行:列" 格式。
func (l SourceLoc) String() string {
	return itoa(l.Line) + ":" + itoa(l.Column)
}

// TokenType 标识 token 类型。
type TokenType int

const (
	// 字面量
	TokNumber TokenType = iota
	TokString
	TokTrue
	TokFalse

	// 标识符与关键字
	TokIdent
	TokLet    // let
	TokFn     // fn
	TokIf     // if
	TokElse   // else
	TokWhile  // while
	TokFor    // for
	TokReturn // return

	// 运算符
	TokPlus    // +
	TokMinus   // -
	TokStar    // *
	TokSlash   // /
	TokPercent // %
	TokGT      // >
	TokLT      // <
	TokGE      // >=
	TokLE      // <=
	TokEQ      // ==
	TokNE      // !=
	TokAnd     // &&
	TokOr      // ||
	TokNot     // !
	TokAssign  // =

	// 分隔符
	TokLParen    // (
	TokRParen    // )
	TokLBrace    // {
	TokRBrace    // }
	TokComma     // ,
	TokSemicolon // ;
	TokLBracket  // [
	TokRBracket  // ]

	TokEOF
)

// tokenNames 是各 TokenType 的可读名（错误信息用）。
var tokenNames = map[TokenType]string{
	TokNumber: "number", TokString: "string", TokTrue: "true", TokFalse: "false",
	TokIdent: "ident", TokLet: "let", TokFn: "fn", TokIf: "if", TokElse: "else",
	TokWhile: "while", TokFor: "for", TokReturn: "return",
	TokPlus: "+", TokMinus: "-", TokStar: "*", TokSlash: "/", TokPercent: "%",
	TokGT: ">", TokLT: "<", TokGE: ">=", TokLE: "<=", TokEQ: "==", TokNE: "!=",
	TokAnd: "&&", TokOr: "||", TokNot: "!", TokAssign: "=",
	TokLParen: "(", TokRParen: ")", TokLBrace: "{", TokRBrace: "}",
	TokComma: ",", TokSemicolon: ";",
	TokLBracket: "[", TokRBracket: "]",
	TokEOF: "EOF",
}

// Token 是词法分析产生的单元。
type Token struct {
	Type  TokenType
	Value string // 字面量值或标识符名（运算符/分隔符为符号本身）
	Loc   SourceLoc
}

// TokenName 返回 token 类型的可读名。
func TokenName(t TokenType) string {
	if n, ok := tokenNames[t]; ok {
		return n
	}
	return "unknown"
}

// String 返回 token 的可读描述。
func (t Token) String() string {
	return TokenName(t.Type) + " " + quote(t.Value) + " @ " + t.Loc.String()
}

// quote 给字符串加引号（错误信息用）。
func quote(s string) string {
	return "\"" + s + "\""
}

// itoa 是 int → string 的轻量实现（避免 import strconv 在 doc 包头部造成混乱）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
