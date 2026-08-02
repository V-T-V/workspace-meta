// Package ast 定义正则表达式的抽象语法树节点。
//
// 正则语法（支持的子集，覆盖 80% 常用场景）：
//
//	字符字面量：  a b c 1 2 3（普通字符）
//	任意字符：    .（匹配除换行外任意字符）
//	转义：        \. \* \+ \? \| \( \) \[ \] \\ \d \w \s
//	字符类：      [abc] [a-z] [^abc] [\d\w]
//	量词：        a*（0或多次）a+（1或多次）a?（0或1次）
//	              非贪婪变体 a*? a+? a??（优先匹配最少）
//	选择：        a|b
//	分组：        (ab)*（捕获分组）
//
// 不支持的（M2 候选）：{n,m} 花括号量词、反向引用、零宽断言、命名分组，
// 以及锚点 ^（行首）/ $（行尾）——锚点目前由 parser 在解析阶段直接报错，
// 故 KindAnchor 分支不会被构造（保留节点定义便于 M2 扩展）。
package ast

// Kind 标识 AST 节点类型。
type Kind int

const (
	KindLiteral   Kind = iota // 单个字面量字符
	KindWildcard              // .（任意字符）
	KindCharClass             // [...] 字符类
	KindConcat                // ab（顺序连接）
	KindAlternate             // a|b（选择）
	KindStar                  // a*（Kleene 闭包）
	KindPlus                  // a+（等价 aa*）
	KindQuestion              // a?（等价 a|ε）
	KindGroup                 // (...) 分组
	KindAnchor                // ^ 或 $
)

// Node 是正则 AST 的节点。
type Node struct {
	Kind     Kind
	Char     rune    // KindLiteral / KindWildcard 时用
	Chars    []rune  // KindCharClass 时的字符集（已展开，如 a-z 变成 abc...z）
	Negated  bool    // KindCharClass 是否取反（[^abc]）
	Children []*Node // KindConcat/Alternate 时是左右操作数；Group 是内部；Star/Plus/Question 是单子
	Anchor   byte    // KindAnchor：'^' 或 '$'
	// Lazy 标记 Star/Plus/Question 为非贪婪量词（*? / +? / ??）。
	// NFA 结构不区分贪婪/非贪婪（状态集合模拟并行推进所有状态），
	// 由 matcher 在多个接受位置中"选最短"还是"选最长"来体现（见 nfa.hasLazy）。
	Lazy bool
}

// NewNode 构造辅助函数（简化创建）。
func NewLiteral(c rune) *Node { return &Node{Kind: KindLiteral, Char: c} }
func NewWildcard() *Node      { return &Node{Kind: KindWildcard} }
func NewCharClass(chars []rune, negated bool) *Node {
	return &Node{Kind: KindCharClass, Chars: chars, Negated: negated}
}
func NewConcat(left, right *Node) *Node {
	return &Node{Kind: KindConcat, Children: []*Node{left, right}}
}
func NewAlternate(left, right *Node) *Node {
	return &Node{Kind: KindAlternate, Children: []*Node{left, right}}
}
func NewStar(child *Node) *Node { return &Node{Kind: KindStar, Children: []*Node{child}} }
func NewPlus(child *Node) *Node { return &Node{Kind: KindPlus, Children: []*Node{child}} }
func NewQuestion(child *Node) *Node {
	return &Node{Kind: KindQuestion, Children: []*Node{child}}
}
func NewGroup(child *Node) *Node { return &Node{Kind: KindGroup, Children: []*Node{child}} }
func NewAnchor(a byte) *Node     { return &Node{Kind: KindAnchor, Anchor: a} }

// NewStarLazy 构造非贪婪 a*? 节点。
func NewStarLazy(child *Node) *Node {
	return &Node{Kind: KindStar, Children: []*Node{child}, Lazy: true}
}

// NewPlusLazy 构造非贪婪 a+? 节点。
func NewPlusLazy(child *Node) *Node {
	return &Node{Kind: KindPlus, Children: []*Node{child}, Lazy: true}
}

// NewQuestionLazy 构造非贪婪 a?? 节点。
func NewQuestionLazy(child *Node) *Node {
	return &Node{Kind: KindQuestion, Children: []*Node{child}, Lazy: true}
}
