// Package parser 实现正则表达式字符串到 AST 的解析（递归下降）。
//
// 语法优先级（从低到高）：
//
//	alternate : concat ('|' concat)*
//	concat    : quantified+
//	quantified: atom ('*'|'+'|'?')? '?'?   // 尾随 '?' 把量词标记为非贪婪（*? +? ??）
//	atom      : literal | '.' | '\' escape | '[' charclass ']' | '(' alternate ')'
//
// 这是标准正则解析器结构（参考 Russ Cox "Regular Expression Matching: the Visual Guide"）。
package parser

import (
	"fmt"

	"github.com/QiuShichang/regex-engine/internal/ast"
)

// Parser 是递归下降解析器。
type Parser struct {
	src             []rune // 源字符串（rune 便于处理中文等）
	pos             int    // 当前位置
	caseInsensitive bool   // (?i) 标志：整个正则不区分大小写
}

// New 创建 Parser。
func New(src string) *Parser {
	return &Parser{src: []rune(src)}
}

// Parse 解析完整正则，返回 AST 根。
func (p *Parser) Parse() (*ast.Node, error) {
	if len(p.src) == 0 {
		return nil, fmt.Errorf("空正则")
	}
	// 仅支持正则开头的 (?i) 标志（简化版，不支持中间插入或 (?-i)）。
	if hasCaseInsensitiveFlag(p.src) {
		p.caseInsensitive = true
		p.pos = len(caseInsensitivePrefix) // 跳过 "(?i)" 4 字符
	}
	node, err := p.parseAlternate()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.src) {
		return nil, fmt.Errorf("位置 %d: 未预期的字符 %q（解析未完成）", p.pos, string(p.src[p.pos]))
	}
	return node, nil
}

// Parse 是包级便捷函数。
func Parse(src string) (*ast.Node, error) {
	return New(src).Parse()
}

// caseInsensitivePrefix 是不区分大小写标志的字面形式。
const caseInsensitivePrefix = "(?i)"

// hasCaseInsensitiveFlag 报告 src 是否以 "(?i)" 开头。
func hasCaseInsensitiveFlag(src []rune) bool {
	if len(src) < len(caseInsensitivePrefix) {
		return false
	}
	for i, r := range caseInsensitivePrefix {
		if src[i] != r {
			return false
		}
	}
	return true
}

// newLiteral 按 caseInsensitive 标志构造字面量节点。
// 不区分大小写时，把 ASCII 字母字面量 a 扩展成字符类 [aA]
// （非字母保持普通 KindLiteral），从而整段匹配大小写两版本。
func (p *Parser) newLiteral(c rune) *ast.Node {
	if p.caseInsensitive {
		if folded := caseFoldRunes(c); len(folded) > 1 {
			return ast.NewCharClass(folded, false)
		}
	}
	return ast.NewLiteral(c)
}

// expandCharClassForCI 把字符类字符集按 caseInsensitive 标志扩展：
// 对每个 ASCII 字母补充其大小写对侧（'a' 补 'A'，反之亦然），
// 去重后返回。非字母不受影响。取反（[^...]）语义不依赖字符集内容
// （matcher 用 set 判断后取反），故同样有效——扩展集意味着
// "[^abc]" 在 (?i) 下同时排除 ABC。
func (p *Parser) expandCharClassForCI(chars []rune) []rune {
	if !p.caseInsensitive {
		return chars
	}
	seen := map[rune]bool{}
	out := make([]rune, 0, len(chars)*2)
	for _, c := range chars {
		for _, f := range caseFoldRunes(c) {
			if !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	return out
}

// caseFoldRunes 返回字符 c 在不区分大小写语义下的所有等价字符。
// 仅处理 ASCII 字母：'a'→{'a','A'}，'A'→{'A','a'}，其余→{c}。
func caseFoldRunes(c rune) []rune {
	switch {
	case c >= 'a' && c <= 'z':
		return []rune{c, c - ('a' - 'A')}
	case c >= 'A' && c <= 'Z':
		return []rune{c, c + ('a' - 'A')}
	default:
		return []rune{c}
	}
}

// peek 看当前字符（不消费）。
func (p *Parser) peek() rune {
	if p.pos >= len(p.src) {
		return -1 // EOF
	}
	return p.src[p.pos]
}

// advance 消费当前字符并返回它。
func (p *Parser) advance() rune {
	r := p.src[p.pos]
	p.pos++
	return r
}

// match 检查当前字符是否是 r，是则消费返回 true。
func (p *Parser) match(r rune) bool {
	if p.peek() == r {
		p.advance()
		return true
	}
	return false
}

// parseAlternate: concat ('|' concat)*
func (p *Parser) parseAlternate() (*ast.Node, error) {
	left, err := p.parseConcat()
	if err != nil {
		return nil, err
	}
	for p.match('|') {
		right, err := p.parseConcat()
		if err != nil {
			return nil, err
		}
		left = ast.NewAlternate(left, right)
	}
	return left, nil
}

// parseConcat: quantified+（一个或多个 quantified 连接）
func (p *Parser) parseConcat() (*ast.Node, error) {
	var result *ast.Node
	for {
		// 遇到这些字符时结束 concat
		c := p.peek()
		if c == -1 || c == '|' || c == ')' {
			break
		}
		node, err := p.parseQuantified()
		if err != nil {
			return nil, err
		}
		if result == nil {
			result = node
		} else {
			result = ast.NewConcat(result, node)
		}
	}
	if result == nil {
		return nil, fmt.Errorf("位置 %d: 期望子表达式", p.pos)
	}
	return result, nil
}

// parseQuantified: atom ('*' | '+' | '?')?，量词后可跟 '?' 表示非贪婪。
func (p *Parser) parseQuantified() (*ast.Node, error) {
	atom, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	switch p.peek() {
	case '*':
		p.advance()
		lazy := p.match('?') // 非贪婪 *?
		if lazy {
			return ast.NewStarLazy(atom), nil
		}
		return ast.NewStar(atom), nil
	case '+':
		p.advance()
		lazy := p.match('?') // 非贪婪 +?
		if lazy {
			return ast.NewPlusLazy(atom), nil
		}
		return ast.NewPlus(atom), nil
	case '?':
		p.advance()
		lazy := p.match('?') // 非贪婪 ??
		if lazy {
			return ast.NewQuestionLazy(atom), nil
		}
		return ast.NewQuestion(atom), nil
	}
	return atom, nil
}

// parseAtom: literal | '.' | '\' escape | '[' charclass ']' | '(' alternate ')' | '^' | '$'
func (p *Parser) parseAtom() (*ast.Node, error) {
	c := p.peek()
	switch c {
	case -1:
		return nil, fmt.Errorf("位置 %d: 期望原子但到结尾", p.pos)
	case '(':
		p.advance()
		node, err := p.parseAlternate()
		if err != nil {
			return nil, err
		}
		if !p.match(')') {
			return nil, fmt.Errorf("位置 %d: 缺少右括号 ')'", p.pos)
		}
		return ast.NewGroup(node), nil
	case '[':
		return p.parseCharClass()
	case '.':
		p.advance()
		return ast.NewWildcard(), nil
	case '\\':
		return p.parseEscape()
	case '*', '+', '?':
		return nil, fmt.Errorf("位置 %d: 量词 %q 无前置表达式", p.pos, string(c))
	case '|', ')':
		return nil, fmt.Errorf("位置 %d: 未预期的 %q", p.pos, string(c))
	case '^', '$':
		// 锚点 ^（行首）/ $（行尾）。零宽断言，不消费字符：
		//   ^ 在 pos==0 或前一字符是 \n 时成立
		//   $ 在 pos==len 或当前字符是 \n 时成立
		// 位置检查由 matcher 的 ε 闭包在遍历锚点边时执行（见 nfa.addAnchor）。
		p.advance()
		return ast.NewAnchor(byte(c)), nil
	default:
		p.advance()
		return p.newLiteral(c), nil
	}
}

// parseEscape: \. \* \+ \? \| \( \) \\ \d \w \s 等
func (p *Parser) parseEscape() (*ast.Node, error) {
	p.advance() // 消费 \
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("位置 %d: 转义符 \\ 后到结尾", p.pos)
	}
	esc := p.advance()
	switch esc {
	case 'p': // \p{name} POSIX 风格字符类简写
		return p.parsePosixClass()
	case 'd': // [0-9]
		return ast.NewCharClass([]rune("0123456789"), false), nil
	case 'w': // [a-zA-Z0-9_]
		return ast.NewCharClass([]rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"), false), nil
	case 's': // [ \t\n\r]
		return ast.NewCharClass([]rune(" \t\n\r"), false), nil
	case 'D', 'W', 'S': // 取反
		var chars []rune
		switch esc {
		case 'D':
			chars = []rune("0123456789")
		case 'W':
			chars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_")
		case 'S':
			chars = []rune(" \t\n\r")
		}
		return ast.NewCharClass(chars, true), nil
	default:
		// 转义普通字符：\. \* \+ 等，按字面量处理
		return p.newLiteral(esc), nil
	}
}

// parsePosixClass 解析 \p{name} 形式的预定义字符类简写。
// 进入时位置已消费了 "\p"，期望当前字符是 '{'。
// 支持的 name（仿 POSIX/常见 regex 引擎风格，子集）：
//
//	\p{lower} → [a-z]
//	\p{upper} → [A-Z]
//	\p{digit} → [0-9]（等价 \d）
//	\p{alpha} → [a-zA-Z]
//	\p{alnum} → [a-zA-Z0-9]
//
// 不识别的 name 报错（避免静默产生空字符类造成困惑）。
func (p *Parser) parsePosixClass() (*ast.Node, error) {
	if !p.match('{') {
		return nil, fmt.Errorf("位置 %d: 期望 \\p{ 后跟 '{'", p.pos)
	}
	// 收集 name 到 '}'
	var name []rune
	for {
		c := p.peek()
		if c == -1 {
			return nil, fmt.Errorf("位置 %d: \\p{...} 未闭合（缺少 '}'）", p.pos)
		}
		if c == '}' {
			p.advance()
			break
		}
		name = append(name, p.advance())
	}
	nameStr := string(name)
	// (?i) 下扩展字符集（补大小写对侧），与普通字符类处理一致。
	var chars []rune
	switch nameStr {
	case "lower": // [a-z]
		chars = []rune("abcdefghijklmnopqrstuvwxyz")
	case "upper": // [A-Z]
		chars = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	case "digit": // [0-9]，等价 \d
		chars = []rune("0123456789")
	case "alpha": // [a-zA-Z]
		chars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	case "alnum": // [a-zA-Z0-9]
		chars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	default:
		return nil, fmt.Errorf("位置 %d: 未知的字符类简写 \\p{%s}", p.pos, nameStr)
	}
	chars = p.expandCharClassForCI(chars)
	return ast.NewCharClass(chars, false), nil
}

// parseCharClass: [...] 字符类，支持 a-z 范围、^ 取反、\d \w \s 转义
func (p *Parser) parseCharClass() (*ast.Node, error) {
	p.advance() // 消费 [
	negated := false
	if p.match('^') {
		negated = true
	}
	var chars []rune
	for {
		c := p.peek()
		if c == -1 {
			return nil, fmt.Errorf("位置 %d: 字符类未闭合（缺少 ']'）", p.pos)
		}
		if c == ']' {
			p.advance()
			break
		}
		var ch rune
		if c == '\\' {
			p.advance()
			esc := p.advance()
			switch esc {
			case 'd':
				chars = append(chars, []rune("0123456789")...)
				continue
			case 'w':
				chars = append(chars, []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_")...)
				continue
			case 's':
				chars = append(chars, []rune(" \t\n\r")...)
				continue
			case 'n':
				ch = '\n'
			case 't':
				ch = '\t'
			case 'r':
				ch = '\r'
			default:
				ch = esc // 转义字面量
			}
		} else {
			ch = p.advance()
		}
		// 范围：a-z
		if p.peek() == '-' {
			p.advance() // 消费 -
			next := p.peek()
			if next != ']' && next != -1 {
				p.advance()
				for r := ch; r <= next; r++ {
					chars = append(chars, r)
				}
				continue
			}
			// - 在末尾，当字面量
			chars = append(chars, ch, '-')
			continue
		}
		chars = append(chars, ch)
	}
	chars = p.expandCharClassForCI(chars)
	return ast.NewCharClass(chars, negated), nil
}
