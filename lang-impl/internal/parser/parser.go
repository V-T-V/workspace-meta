// Package parser 实现 M 语言的语法分析：把 lexer 产出的 []core.Token
// 序列递归下降地解析成 core.AST。
//
// 解析范围：
//   - 表达式（含运算符优先级：|| < && < ==/!= < 比较 < +/- < */% < 一元 < 主）
//   - 语句：let / fn / if-else / while / for / return / 表达式语句 / 块
//   - 程序：顶层语句序列（函数声明 + 全局 let + 表达式语句）
//
// 边界：
//   - parser 不重新切词法（token 流由 lexer 产出，已含 TokEOF）
//   - parser 不做类型检查（那是 interpreter 的事），只校验"语法形状"合法
//
// 更多背景见 NOTES.md。
package parser

import (
	"strconv"
	"strings"

	"github.com/QiuShichang/lang-impl/internal/core"
)

// Parser 是递归下降语法分析器，持有 token 流与当前位置 pos。
type Parser struct {
	tokens []core.Token
	pos    int
}

// New 创建 Parser。tokens 末尾应有一个 TokEOF。
func New(tokens []core.Token) *Parser {
	return &Parser{tokens: tokens}
}

// Parse 是包级便捷入口：New(tokens).ParseProgram()。
func Parse(tokens []core.Token) (*core.Program, error) {
	return New(tokens).ParseProgram()
}

// ParseProgram 解析整个程序：顶层语句列表直到 TokEOF。
func (p *Parser) ParseProgram() (*core.Program, error) {
	var stmts []core.Stmt
	// 程序起点用第一个 token 的位置；空程序用 EOF 位置。
	var start core.SourceLoc
	if len(p.tokens) > 0 {
		start = p.tokens[0].Loc
	}
	for !p.check(core.TokEOF) {
		stmt, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, stmt)
	}
	return &core.Program{Loc: start, Stmts: stmts}, nil
}

// ===== 递归下降辅助 =====

// peek 返回当前 token（不消费）。
func (p *Parser) peek() core.Token {
	if p.pos >= len(p.tokens) {
		// 安全兜底：返回 EOF（理论上 tokens 末尾必有 EOF）
		return core.Token{Type: core.TokEOF}
	}
	return p.tokens[p.pos]
}

// peekAt 看相对当前位置偏移 k 的 token（不消费）。k=0 等价 peek()。
func (p *Parser) peekAt(k int) core.Token {
	idx := p.pos + k
	if idx >= len(p.tokens) {
		return core.Token{Type: core.TokEOF}
	}
	return p.tokens[idx]
}

// advance 消费当前 token 并返回它，pos++。
func (p *Parser) advance() core.Token {
	t := p.peek()
	if t.Type != core.TokEOF {
		p.pos++
	}
	return t
}

// check 报告当前 token 是否为指定类型。
func (p *Parser) check(t core.TokenType) bool {
	return p.peek().Type == t
}

// match 若当前 token 属于给定类型之一则消费并返回 true，否则返回 false。
func (p *Parser) match(types ...core.TokenType) bool {
	for _, t := range types {
		if p.check(t) {
			p.advance()
			return true
		}
	}
	return false
}

// expect 期望当前 token 为 t，否则返回带 SourceLoc 的错误。
func (p *Parser) expect(t core.TokenType) (core.Token, error) {
	if p.check(t) {
		return p.advance(), nil
	}
	cur := p.peek()
	return core.Token{}, core.NewError(cur.Loc, "期望 %s，实际 %s",
		core.TokenName(t), describeToken(cur))
}

// describeToken 给出 token 在错误信息中的描述（带值更友好）。
func describeToken(t core.Token) string {
	if t.Value != "" && t.Type != core.TokEOF {
		return core.TokenName(t.Type) + " " + strconv.Quote(t.Value)
	}
	return core.TokenName(t.Type)
}

// ===== 语句解析 =====

func (p *Parser) parseStmt() (core.Stmt, error) {
	tok := p.peek()
	switch tok.Type {
	case core.TokLet:
		return p.parseLet()
	case core.TokFn:
		return p.parseFn()
	case core.TokIf:
		return p.parseIf()
	case core.TokWhile:
		return p.parseWhile()
	case core.TokFor:
		return p.parseFor()
	case core.TokReturn:
		return p.parseReturn()
	case core.TokBreak:
		return p.parseBreak()
	case core.TokContinue:
		return p.parseContinue()
	case core.TokLBrace:
		// 块语句（注意：返回 *BlockStmt，但它本身实现 Stmt）
		return p.parseBlock()
	case core.TokIdent:
		// 裸赋值语句：ident = expr; （对已有变量重新赋值，复用 LetStmt 语义）。
		// 仅当 ident 后紧跟 = 时识别（否则按表达式语句处理，如函数调用 foo();）。
		// 这是 while 循环累加等场景必需的（M 语言支持可变状态）。
		if p.peekAt(1).Type == core.TokAssign {
			return p.parseAssign()
		}
	}
	// 否则按表达式语句处理（含函数调用 foo();、字面量 1; 等）
	return p.parseExprStmt()
}

// parseAssign: ident "=" expr ";"（裸赋值，对已有变量重新赋值）
// 复用 LetStmt 节点——interpreter 的 Set 语义是"定义或覆盖"，let 和 assign 都走它。
func (p *Parser) parseAssign() (core.Stmt, error) {
	nameTok := p.advance() // ident
	loc := nameTok.Loc
	if _, err := p.expect(core.TokAssign); err != nil {
		return nil, err
	}
	init, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(core.TokSemicolon); err != nil {
		return nil, err
	}
	return &core.LetStmt{Loc: loc, Name: nameTok.Value, Init: init, IsAssign: true}, nil
}

// parseLet: "let" ident "=" expr ";"
func (p *Parser) parseLet() (core.Stmt, error) {
	kw := p.advance() // let
	nameTok, err := p.expect(core.TokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(core.TokAssign); err != nil {
		return nil, err
	}
	init, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(core.TokSemicolon); err != nil {
		return nil, err
	}
	return &core.LetStmt{Loc: kw.Loc, Name: nameTok.Value, Init: init}, nil
}

// parseFn: ident "(" params? ")" block
func (p *Parser) parseFn() (core.Stmt, error) {
	kw := p.advance() // fn
	nameTok, err := p.expect(core.TokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(core.TokLParen); err != nil {
		return nil, err
	}
	var params []string
	if !p.check(core.TokRParen) {
		for {
			pt, err := p.expect(core.TokIdent)
			if err != nil {
				return nil, err
			}
			params = append(params, pt.Value)
			if !p.match(core.TokComma) {
				break
			}
		}
	}
	if _, err := p.expect(core.TokRParen); err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &core.FnDecl{Loc: kw.Loc, Name: nameTok.Value, Params: params, Body: body}, nil
}

// parseIf: "if" "(" expr ")" block ("else" block)?
func (p *Parser) parseIf() (core.Stmt, error) {
	kw := p.advance() // if
	if _, err := p.expect(core.TokLParen); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(core.TokRParen); err != nil {
		return nil, err
	}
	thenBlk, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	var elseBlk *core.BlockStmt
	if p.match(core.TokElse) {
		// else 后跟 if 形成 else-if 链：把 if 包成 BlockStmt（单语句块）
		if p.check(core.TokIf) {
			nestedIf, err := p.parseIf()
			if err != nil {
				return nil, err
			}
			elseBlk = &core.BlockStmt{Loc: nestedIf.NodeLoc(), Stmts: []core.Stmt{nestedIf}}
		} else {
			elseBlk, err = p.parseBlock()
			if err != nil {
				return nil, err
			}
		}
	}
	return &core.IfStmt{Loc: kw.Loc, Cond: cond, Then: thenBlk, Else: elseBlk}, nil
}

// parseWhile: "while" "(" expr ")" block
func (p *Parser) parseWhile() (core.Stmt, error) {
	kw := p.advance() // while
	if _, err := p.expect(core.TokLParen); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(core.TokRParen); err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &core.WhileStmt{Loc: kw.Loc, Cond: cond, Body: body}, nil
}

// parseFor: "for" "(" init? ";" cond? ";" update? ")" block
//
// C 风格 for 循环。三段都以 ';' 分隔，每段皆可为空：
//
//	for (let i = 0; i < 10; i = i + 1) { ... }
//	for (;;) { ... }   // 无限循环（cond 为空视为真）
//
// init/update 是"无尾分号"的语句片段：支持 let（let i = 0）、裸赋值
// （i = i + 1）或表达式语句。复用 parseSimpleStmtNoSemi 解析，避免与
// parseStmt 的 ";" 消费冲突（for 子句的 ';' 由本函数显式 expect）。
func (p *Parser) parseFor() (core.Stmt, error) {
	kw := p.advance() // for
	if _, err := p.expect(core.TokLParen); err != nil {
		return nil, err
	}
	// init（可空）
	var init core.Stmt
	if !p.check(core.TokSemicolon) {
		s, err := p.parseSimpleStmtNoSemi()
		if err != nil {
			return nil, err
		}
		init = s
	}
	if _, err := p.expect(core.TokSemicolon); err != nil {
		return nil, err
	}
	// cond（可空 → 恒真）
	var cond core.Expr
	if !p.check(core.TokSemicolon) {
		c, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		cond = c
	}
	if _, err := p.expect(core.TokSemicolon); err != nil {
		return nil, err
	}
	// update（可空）
	var update core.Stmt
	if !p.check(core.TokRParen) {
		s, err := p.parseSimpleStmtNoSemi()
		if err != nil {
			return nil, err
		}
		update = s
	}
	if _, err := p.expect(core.TokRParen); err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &core.ForStmt{Loc: kw.Loc, Init: init, Cond: cond, Update: update, Body: body}, nil
}

// parseSimpleStmtNoSemi 解析"单条语句但不消费尾分号"，用于 for 子句：
// 支持 let 绑定、裸赋值（ident = expr）或表达式语句。
// 不消费 ';'——分隔符由调用方（parseFor）显式处理。
func (p *Parser) parseSimpleStmtNoSemi() (core.Stmt, error) {
	tok := p.peek()
	switch tok.Type {
	case core.TokLet:
		// 复用 parseLet 的逻辑但不期望尾分号：这里就地实现一份精简版。
		kw := p.advance() // let
		nameTok, err := p.expect(core.TokIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(core.TokAssign); err != nil {
			return nil, err
		}
		init, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &core.LetStmt{Loc: kw.Loc, Name: nameTok.Value, Init: init}, nil
	case core.TokIdent:
		// 裸赋值：ident = expr
		if p.peekAt(1).Type == core.TokAssign {
			nameTok := p.advance() // ident
			loc := nameTok.Loc
			if _, err := p.expect(core.TokAssign); err != nil {
				return nil, err
			}
			init, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			return &core.LetStmt{Loc: loc, Name: nameTok.Value, Init: init, IsAssign: true}, nil
		}
	}
	// 否则按表达式语句（如函数调用 foo() ）
	e, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &core.ExprStmt{Loc: tok.Loc, Expr: e}, nil
}

// parseReturn: "return" expr? ";"
func (p *Parser) parseReturn() (core.Stmt, error) {
	kw := p.advance() // return
	var val core.Expr
	// return; 直接结束；否则解析表达式
	if !p.check(core.TokSemicolon) {
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		val = e
	}
	if _, err := p.expect(core.TokSemicolon); err != nil {
		return nil, err
	}
	return &core.ReturnStmt{Loc: kw.Loc, Value: val}, nil
}

// parseBreak: "break" ";"
// 跳出最近的 while/for 循环。合法性（是否在循环内）由 interpreter 在
// 捕获 breakSignal 时隐式保证：循环外的 break 会让信号冒泡到顶层 Run，
// Run 的 recover 不识别它从而重新 panic（表现为运行时错误）。
func (p *Parser) parseBreak() (core.Stmt, error) {
	kw := p.advance() // break
	if _, err := p.expect(core.TokSemicolon); err != nil {
		return nil, err
	}
	return &core.BreakStmt{Loc: kw.Loc}, nil
}

// parseContinue: "continue" ";"
// 跳过本轮循环剩余语句，进入下一轮（最近的 while/for）。
func (p *Parser) parseContinue() (core.Stmt, error) {
	kw := p.advance() // continue
	if _, err := p.expect(core.TokSemicolon); err != nil {
		return nil, err
	}
	return &core.ContinueStmt{Loc: kw.Loc}, nil
}

// parseExprStmt: expr ";"
func (p *Parser) parseExprStmt() (core.Stmt, error) {
	tok := p.peek()
	e, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(core.TokSemicolon); err != nil {
		return nil, err
	}
	return &core.ExprStmt{Loc: tok.Loc, Expr: e}, nil
}

// parseBlock: "{" stmt* "}"
// 返回 *BlockStmt（它同时实现 Stmt）。
func (p *Parser) parseBlock() (*core.BlockStmt, error) {
	lbrace, err := p.expect(core.TokLBrace)
	if err != nil {
		return nil, err
	}
	var stmts []core.Stmt
	for !p.check(core.TokRBrace) && !p.check(core.TokEOF) {
		s, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, s)
	}
	if _, err := p.expect(core.TokRBrace); err != nil {
		return nil, err
	}
	return &core.BlockStmt{Loc: lbrace.Loc, Stmts: stmts}, nil
}

// ===== 表达式解析（优先级阶梯，从低到高）=====
//
// 每级一个函数：左结合（left-associative），靠 while 循环卷起同优先级序列。
//   parseExpr → parseOr（入口，统一对外）

// parseExpr 是表达式入口。
func (p *Parser) parseExpr() (core.Expr, error) {
	return p.parseOr()
}

// parseOr: parseAnd ( "||" parseAnd )*
func (p *Parser) parseOr() (core.Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.check(core.TokOr) {
		op := p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &core.BinaryExpr{Loc: op.Loc, Op: core.TokOr, Left: left, Right: right}
	}
	return left, nil
}

// parseAnd: parseEquality ( "&&" parseEquality )*
func (p *Parser) parseAnd() (core.Expr, error) {
	left, err := p.parseEquality()
	if err != nil {
		return nil, err
	}
	for p.check(core.TokAnd) {
		op := p.advance()
		right, err := p.parseEquality()
		if err != nil {
			return nil, err
		}
		left = &core.BinaryExpr{Loc: op.Loc, Op: core.TokAnd, Left: left, Right: right}
	}
	return left, nil
}

// parseEquality: parseComparison ( ("=="|"!=") parseComparison )*
func (p *Parser) parseEquality() (core.Expr, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.check(core.TokEQ) || p.check(core.TokNE) {
		op := p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = &core.BinaryExpr{Loc: op.Loc, Op: op.Type, Left: left, Right: right}
	}
	return left, nil
}

// parseComparison: parseTerm ( (">"|"<"|">="|"<=") parseTerm )*
func (p *Parser) parseComparison() (core.Expr, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for p.check(core.TokGT) || p.check(core.TokLT) || p.check(core.TokGE) || p.check(core.TokLE) {
		op := p.advance()
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		left = &core.BinaryExpr{Loc: op.Loc, Op: op.Type, Left: left, Right: right}
	}
	return left, nil
}

// parseTerm: parseFactor ( ("+"|"-") parseFactor )*   （加法级）
func (p *Parser) parseTerm() (core.Expr, error) {
	left, err := p.parseFactor()
	if err != nil {
		return nil, err
	}
	for p.check(core.TokPlus) || p.check(core.TokMinus) {
		op := p.advance()
		right, err := p.parseFactor()
		if err != nil {
			return nil, err
		}
		left = &core.BinaryExpr{Loc: op.Loc, Op: op.Type, Left: left, Right: right}
	}
	return left, nil
}

// parseFactor: parseUnary ( ("*"|"/"|"%") parseUnary )*   （乘法级）
func (p *Parser) parseFactor() (core.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.check(core.TokStar) || p.check(core.TokSlash) || p.check(core.TokPercent) {
		op := p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &core.BinaryExpr{Loc: op.Loc, Op: op.Type, Left: left, Right: right}
	}
	return left, nil
}

// parseUnary: ("!"|"-") parseUnary | parsePrimary   （右结合，故递归）
func (p *Parser) parseUnary() (core.Expr, error) {
	if p.check(core.TokNot) || p.check(core.TokMinus) {
		op := p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &core.UnaryExpr{Loc: op.Loc, Op: op.Type, Right: right}, nil
	}
	return p.parsePrimary()
}

// parsePrimary: 字面量 | ident | ident "(" args ")" | "(" expr ")"
func (p *Parser) parsePrimary() (core.Expr, error) {
	tok := p.peek()
	switch tok.Type {
	case core.TokNumber:
		p.advance()
		v, err := strconv.ParseInt(tok.Value, 10, 64)
		if err != nil {
			return nil, core.NewError(tok.Loc, "非法数字字面量 %q: %v", tok.Value, err)
		}
		return p.parsePostfixIndex(&core.NumberExpr{Loc: tok.Loc, Value: v})
	case core.TokString:
		p.advance()
		return p.parsePostfixIndex(&core.StringExpr{Loc: tok.Loc, Value: tok.Value})
	case core.TokTrue:
		p.advance()
		return p.parsePostfixIndex(&core.BoolExpr{Loc: tok.Loc, Value: true})
	case core.TokFalse:
		p.advance()
		return p.parsePostfixIndex(&core.BoolExpr{Loc: tok.Loc, Value: false})
	case core.TokIdent:
		p.advance()
		// 函数调用 ident "(" args ")"
		if p.check(core.TokLParen) {
			p.advance() // (
			var args []core.Expr
			if !p.check(core.TokRParen) {
				for {
					a, err := p.parseExpr()
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					if !p.match(core.TokComma) {
						break
					}
				}
			}
			if _, err := p.expect(core.TokRParen); err != nil {
				return nil, err
			}
			return &core.CallExpr{Loc: tok.Loc, Callee: tok.Value, Args: args}, nil
		}
		return p.parsePostfixIndex(&core.IdentExpr{Loc: tok.Loc, Name: tok.Value})
	case core.TokLParen:
		p.advance() // (
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(core.TokRParen); err != nil {
			return nil, err
		}
		return p.parsePostfixIndex(e)
	case core.TokLBracket:
		// 数组字面量 [expr, expr, ...]
		p.advance() // [
		var elems []core.Expr
		if !p.check(core.TokRBracket) {
			for {
				e, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				elems = append(elems, e)
				if !p.match(core.TokComma) {
					break
				}
			}
		}
		if _, err := p.expect(core.TokRBracket); err != nil {
			return nil, err
		}
		return p.parsePostfixIndex(&core.ArrayExpr{Loc: tok.Loc, Elements: elems})
	case core.TokFn:
		// 匿名函数表达式（一等函数）：fn(params) { body }
		// 注意：fn 作为语句开头时走 parseFn（命名函数声明 fn name(){}）；
		// 走到 parsePrimary 的 fn 一定是表达式上下文里的匿名函数。
		fn, err := p.parseFnExpr()
		if err != nil {
			return nil, err
		}
		return p.parsePostfixIndex(fn)
	}
	return nil, core.NewError(tok.Loc, "意外的 token %s（期望表达式）", describeToken(tok))
}

// parseFnExpr 解析匿名函数表达式：fn "(" params? ")" block。
// 与 parseFn（命名函数声明）共用参数列表/块解析逻辑，但不消费函数名，
// 也不要求尾分号（表达式本身不带分号，由外层 parseLet/parseExprStmt 处理）。
//
// 例子：fn(x) { return x + 1; }
func (p *Parser) parseFnExpr() (*core.FnExpr, error) {
	kw := p.advance() // fn
	if _, err := p.expect(core.TokLParen); err != nil {
		return nil, err
	}
	var params []string
	if !p.check(core.TokRParen) {
		for {
			pt, err := p.expect(core.TokIdent)
			if err != nil {
				return nil, err
			}
			params = append(params, pt.Value)
			if !p.match(core.TokComma) {
				break
			}
		}
	}
	if _, err := p.expect(core.TokRParen); err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &core.FnExpr{Loc: kw.Loc, Params: params, Body: body}, nil
}

// parsePostfixIndex 处理数组索引后缀 expr[index]（可链式：arr[0][1]）。
func (p *Parser) parsePostfixIndex(expr core.Expr) (core.Expr, error) {
	for p.check(core.TokLBracket) {
		loc := p.peek().Loc
		p.advance() // [
		idx, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(core.TokRBracket); err != nil {
			return nil, err
		}
		expr = &core.IndexExpr{Loc: loc, Array: expr, Index: idx}
	}
	return expr, nil
}

// ===== AST Printer =====
//
// Print 把任意 AST 节点（含 *core.Program）转成缩进树形字符串，用于 demo/调试。
// 不带 position 信息（纯结构展示），叶子带值。
//
// 注意：参数类型用 any 而非 core.Node，因为 *core.Program 不实现 Node 接口
// （它是顶层容器），但仍需要能被打印。

// Print 返回节点 n 的缩进树形表示。n 可以是 *core.Program / core.Stmt / core.Expr。
func Print(n any) string {
	var sb strings.Builder
	printNode(&sb, n, 0)
	return sb.String()
}

// printNode 递归输出，indent 是当前缩进层数。
func printNode(sb *strings.Builder, n any, indent int) {
	writeIndent(sb, indent)
	if n == nil {
		sb.WriteString("(nil)\n")
		return
	}
	switch e := n.(type) {
	case *core.Program:
		sb.WriteString("Program\n")
		for _, s := range e.Stmts {
			printNode(sb, s, indent+1)
		}
	case *core.BlockStmt:
		sb.WriteString("Block\n")
		for _, s := range e.Stmts {
			printNode(sb, s, indent+1)
		}
	case *core.LetStmt:
		sb.WriteString("Let ")
		sb.WriteString(strconv.Quote(e.Name))
		sb.WriteByte('\n')
		printNode(sb, e.Init, indent+1)
	case *core.FnDecl:
		sb.WriteString("Fn ")
		sb.WriteString(strconv.Quote(e.Name))
		sb.WriteString("(")
		sb.WriteString(strings.Join(e.Params, ", "))
		sb.WriteString(")\n")
		printNode(sb, e.Body, indent+1)
	case *core.IfStmt:
		sb.WriteString("If\n")
		printNode(sb, e.Cond, indent+1)
		writeIndent(sb, indent+1)
		sb.WriteString("Then:\n")
		printNode(sb, e.Then, indent+2)
		if e.Else != nil {
			writeIndent(sb, indent+1)
			sb.WriteString("Else:\n")
			printNode(sb, e.Else, indent+2)
		}
	case *core.WhileStmt:
		sb.WriteString("While\n")
		printNode(sb, e.Cond, indent+1)
		writeIndent(sb, indent+1)
		sb.WriteString("Body:\n")
		printNode(sb, e.Body, indent+2)
	case *core.ForStmt:
		sb.WriteString("For\n")
		if e.Init != nil {
			writeIndent(sb, indent+1)
			sb.WriteString("Init:\n")
			printNode(sb, e.Init, indent+2)
		}
		if e.Cond != nil {
			writeIndent(sb, indent+1)
			sb.WriteString("Cond:\n")
			printNode(sb, e.Cond, indent+2)
		}
		if e.Update != nil {
			writeIndent(sb, indent+1)
			sb.WriteString("Update:\n")
			printNode(sb, e.Update, indent+2)
		}
		writeIndent(sb, indent+1)
		sb.WriteString("Body:\n")
		printNode(sb, e.Body, indent+2)
	case *core.ReturnStmt:
		sb.WriteString("Return\n")
		if e.Value != nil {
			printNode(sb, e.Value, indent+1)
		}
	case *core.BreakStmt:
		sb.WriteString("Break\n")
	case *core.ContinueStmt:
		sb.WriteString("Continue\n")
	case *core.ExprStmt:
		sb.WriteString("ExprStmt\n")
		printNode(sb, e.Expr, indent+1)
	case *core.NumberExpr:
		sb.WriteString("Number ")
		sb.WriteString(strconv.FormatInt(e.Value, 10))
		sb.WriteByte('\n')
	case *core.StringExpr:
		sb.WriteString("String ")
		sb.WriteString(strconv.Quote(e.Value))
		sb.WriteByte('\n')
	case *core.BoolExpr:
		sb.WriteString("Bool ")
		sb.WriteString(strconv.FormatBool(e.Value))
		sb.WriteByte('\n')
	case *core.IdentExpr:
		sb.WriteString("Ident ")
		sb.WriteString(strconv.Quote(e.Name))
		sb.WriteByte('\n')
	case *core.BinaryExpr:
		sb.WriteString("Binary ")
		sb.WriteString(core.TokenName(e.Op))
		sb.WriteByte('\n')
		printNode(sb, e.Left, indent+1)
		printNode(sb, e.Right, indent+1)
	case *core.UnaryExpr:
		sb.WriteString("Unary ")
		sb.WriteString(core.TokenName(e.Op))
		sb.WriteByte('\n')
		printNode(sb, e.Right, indent+1)
	case *core.CallExpr:
		sb.WriteString("Call ")
		sb.WriteString(strconv.Quote(e.Callee))
		sb.WriteByte('\n')
		for _, a := range e.Args {
			printNode(sb, a, indent+1)
		}
	case *core.FnExpr:
		sb.WriteString("FnExpr(")
		sb.WriteString(strings.Join(e.Params, ", "))
		sb.WriteString(")\n")
		printNode(sb, e.Body, indent+1)
	default:
		sb.WriteString("(unknown node)\n")
	}
}

// writeIndent 写入 indent*2 个空格。
func writeIndent(sb *strings.Builder, indent int) {
	for i := 0; i < indent; i++ {
		sb.WriteString("  ")
	}
}
