// Package lexer 实现 M 语言的词法分析：把源码字符串切成 Token 序列。
//
// 支持的 token：
//   - 字面量：数字（十进制整数）、字符串（双引号）、true/false
//   - 标识符与关键字：let / fn / if / else / while / for / return
//   - 运算符：+ - * / % > < >= <= == != && || ! =
//   - 分隔符：( ) { } , ;
//   - 注释：// 行注释（跳过）
//
// 跳过空白字符（空格/tab/换行/回车）。
package lexer

import (
	"strings"

	"github.com/QiuShichang/lang-impl/internal/core"
)

// keywords 是关键字表（识别标识符后判断是否为关键字）。
var keywords = map[string]core.TokenType{
	"let":      core.TokLet,
	"fn":       core.TokFn,
	"if":       core.TokIf,
	"else":     core.TokElse,
	"while":    core.TokWhile,
	"for":      core.TokFor,
	"return":   core.TokReturn,
	"break":    core.TokBreak,
	"continue": core.TokContinue,
	"true":     core.TokTrue,
	"false":    core.TokFalse,
}

// Lexer 是词法分析器，逐字符消费源码。
type Lexer struct {
	src    string
	pos    int // 当前字节位置
	line   int // 当前行号（1-based）
	col    int // 当前列号（1-based）
	tokens []core.Token
}

// New 创建 Lexer。
func New(src string) *Lexer {
	return &Lexer{src: src, line: 1, col: 1}
}

// Tokenize 执行完整的词法分析，返回 token 序列或错误。
func (l *Lexer) Tokenize() ([]core.Token, error) {
	for {
		l.skipWhitespaceAndComments()
		if l.eof() {
			break
		}
		tok, err := l.next()
		if err != nil {
			return nil, err
		}
		l.tokens = append(l.tokens, tok)
	}
	// 末尾加 EOF token
	l.tokens = append(l.tokens, core.Token{Type: core.TokEOF, Loc: core.SourceLoc{Line: l.line, Column: l.col}})
	return l.tokens, nil
}

// eof 报告是否到结尾。
func (l *Lexer) eof() bool { return l.pos >= len(l.src) }

// peek 看当前字符（不消费）。eof 返回 0。
func (l *Lexer) peek() byte {
	if l.eof() {
		return 0
	}
	return l.src[l.pos]
}

// peek2 看下一个字符（不消费）。
func (l *Lexer) peek2() byte {
	if l.pos+1 >= len(l.src) {
		return 0
	}
	return l.src[l.pos+1]
}

// advance 消费当前字符，返回它，并更新行列号。
func (l *Lexer) advance() byte {
	c := l.src[l.pos]
	l.pos++
	if c == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return c
}

// loc 返回当前位置。
func (l *Lexer) loc() core.SourceLoc { return core.SourceLoc{Line: l.line, Column: l.col} }

// skipWhitespaceAndComments 跳过空白和 // 注释。
func (l *Lexer) skipWhitespaceAndComments() {
	for !l.eof() {
		c := l.peek()
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			l.advance()
			continue
		}
		// 行注释 //
		if c == '/' && l.peek2() == '/' {
			for !l.eof() && l.peek() != '\n' {
				l.advance()
			}
			continue
		}
		// 块注释 /* ... */
		if c == '/' && l.peek2() == '*' {
			l.advance() // 消费 /
			l.advance() // 消费 *
			for !l.eof() {
				if l.peek() == '*' && l.peek2() == '/' {
					l.advance() // 消费 *
					l.advance() // 消费 /
					break
				}
				l.advance()
			}
			continue
		}
		break
	}
}

// next 读取下一个 token（调用前已跳过空白/注释，且未到 eof）。
func (l *Lexer) next() (core.Token, error) {
	loc := l.loc()
	c := l.peek()

	// 数字
	if isDigit(c) {
		return l.readNumber(loc)
	}
	// 标识符/关键字
	if isIdentStart(c) {
		return l.readIdent(loc)
	}
	// 字符串
	if c == '"' {
		return l.readString(loc)
	}
	// 运算符/分隔符
	return l.readOperator(loc)
}

// readNumber 读取十进制整数。
// 注意：M 语言当前只支持整数（int64）。浮点数是 M3 候选（需改 lexer/parser/ast/interpreter/wasm 全栈）。
func (l *Lexer) readNumber(loc core.SourceLoc) (core.Token, error) {
	start := l.pos
	for !l.eof() && isDigit(l.peek()) {
		l.advance()
	}
	val := l.src[start:l.pos]
	return core.Token{Type: core.TokNumber, Value: val, Loc: loc}, nil
}

// readIdent 读取标识符（含关键字判断）。
func (l *Lexer) readIdent(loc core.SourceLoc) (core.Token, error) {
	start := l.pos
	for !l.eof() && isIdentPart(l.peek()) {
		l.advance()
	}
	name := l.src[start:l.pos]
	// 关键字判断
	if kw, ok := keywords[name]; ok {
		return core.Token{Type: kw, Value: name, Loc: loc}, nil
	}
	return core.Token{Type: core.TokIdent, Value: name, Loc: loc}, nil
}

// readString 读取双引号字符串（支持 \" \\ \n \t 转义）。
func (l *Lexer) readString(loc core.SourceLoc) (core.Token, error) {
	l.advance() // 消费开头的 "
	var sb strings.Builder
	for !l.eof() {
		c := l.peek()
		if c == '"' {
			l.advance() // 消费结尾的 "
			return core.Token{Type: core.TokString, Value: sb.String(), Loc: loc}, nil
		}
		if c == '\\' {
			l.advance() // 消费 \
			if l.eof() {
				return core.Token{}, core.NewError(loc, "字符串未闭合（转义后遇到结尾）")
			}
			esc := l.advance()
			switch esc {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			default:
				sb.WriteByte(esc) // 未知转义原样保留
			}
			continue
		}
		sb.WriteByte(l.advance())
	}
	return core.Token{}, core.NewError(loc, "字符串未闭合（缺少结尾 \"）")
}

// readOperator 读取运算符/分隔符（处理多字符：>= <= == != && ||）。
func (l *Lexer) readOperator(loc core.SourceLoc) (core.Token, error) {
	c := l.advance()
	switch c {
	case '+':
		return tok(core.TokPlus, "+", loc), nil
	case '-':
		return tok(core.TokMinus, "-", loc), nil
	case '*':
		return tok(core.TokStar, "*", loc), nil
	case '/':
		return tok(core.TokSlash, "/", loc), nil
	case '%':
		return tok(core.TokPercent, "%", loc), nil
	case '(':
		return tok(core.TokLParen, "(", loc), nil
	case ')':
		return tok(core.TokRParen, ")", loc), nil
	case '{':
		return tok(core.TokLBrace, "{", loc), nil
	case '}':
		return tok(core.TokRBrace, "}", loc), nil
	case ',':
		return tok(core.TokComma, ",", loc), nil
	case ';':
		return tok(core.TokSemicolon, ";", loc), nil
	case '[':
		return tok(core.TokLBracket, "[", loc), nil
	case ']':
		return tok(core.TokRBracket, "]", loc), nil
	case '=':
		if l.peek() == '=' {
			l.advance()
			return tok(core.TokEQ, "==", loc), nil
		}
		return tok(core.TokAssign, "=", loc), nil
	case '!':
		if l.peek() == '=' {
			l.advance()
			return tok(core.TokNE, "!=", loc), nil
		}
		return tok(core.TokNot, "!", loc), nil
	case '>':
		if l.peek() == '=' {
			l.advance()
			return tok(core.TokGE, ">=", loc), nil
		}
		return tok(core.TokGT, ">", loc), nil
	case '<':
		if l.peek() == '=' {
			l.advance()
			return tok(core.TokLE, "<=", loc), nil
		}
		return tok(core.TokLT, "<", loc), nil
	case '&':
		if l.peek() == '&' {
			l.advance()
			return tok(core.TokAnd, "&&", loc), nil
		}
		return core.Token{}, core.NewError(loc, "未知字符 '&'（期望 '&&'）")
	case '|':
		if l.peek() == '|' {
			l.advance()
			return tok(core.TokOr, "||", loc), nil
		}
		return core.Token{}, core.NewError(loc, "未知字符 '|'（期望 '||'）")
	}
	return core.Token{}, core.NewError(loc, "未知字符 %q", string(c))
}

// tok 是构造 token 的简写。
func tok(t core.TokenType, val string, loc core.SourceLoc) core.Token {
	return core.Token{Type: t, Value: val, Loc: loc}
}

// isDigit 判断是否是十进制数字。
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// isIdentStart 判断是否是标识符首字符（字母/下划线）。
func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

// isIdentPart 判断是否是标识符后续字符（字母/数字/下划线）。
func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c)
}

// Tokenize 是包级便捷函数：New(src).Tokenize()。
func Tokenize(src string) ([]core.Token, error) {
	return New(src).Tokenize()
}
