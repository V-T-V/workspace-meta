// Package nfa 用 Thompson 算法把正则 AST 转换成 NFA（非确定性有限自动机）。
//
// Thompson 算法（Ken Thompson 1968）的核心：每种 AST 节点对应一个 NFA 片段，
// 片段有"起始状态"和"接受状态"，通过 ε 转移（空转移）拼接。
//
// NFA 状态用整数 ID 表示，转移表用 map[state][]edge。
// 匹配时用"状态集合"模拟（同时跟踪所有可能状态），避免回溯。
//
// 这是 Russ Cox "Regular Expression Matching: the Visual Guide" 的经典实现。
package nfa

import (
	"github.com/QiuShichang/regex-engine/internal/ast"
)

// State 是 NFA 状态 ID。
type State int

// Edge 是 NFA 的一条转移。
// epsilon=true 时是 ε 转移（from→to 不消费字符）；
// 否则是字符转移：from --charMatcher(c)--> to（c 由 matcher 函数判断）。
//
// CaptureStart / CaptureEnd 用于分组捕获（>=0 时表示经过此边会
// 触发对应第 N 个捕获组的"进入"/"离开"事件）。仅 ε 边会带捕获标记：
// 进入分组用一条 CaptureStart=N 的 ε 边，离开用一条 CaptureEnd=N 的 ε 边。
// -1（默认）表示不参与捕获。
type Edge struct {
	To           State
	Epsilon      bool
	CaptureStart int // >=0 表示进入第 N 个捕获组；-1 表示无
	CaptureEnd   int // >=0 表示离开第 N 个捕获组；-1 表示无
	// matcher 判断输入字符是否匹配此边（epsilon=false 时用）。
	// 对于字面量是 c == char；对于 . 是 true；对于 [...] 是集合包含。
	matcher func(rune) bool
}

// noCapture 是"无捕获"的哨兵值。
const noCapture = -1

// Match 报告字符 r 是否匹配此边（epsilon=false 时调用）。
// epsilon 边返回 false（不消费字符）。
func (e Edge) Match(r rune) bool {
	if e.Epsilon || e.matcher == nil {
		return false
	}
	return e.matcher(r)
}

// NFA 是一个完整的非确定性有限自动机。
type NFA struct {
	Start  State
	Accept State
	// transitions[from] = 从 from 出发的所有边
	transitions map[State][]Edge
	nextID      State
	groupCount  int // 捕获分组总数（不含整体匹配的组 0）
}

// New 创建空 NFA。
func New() *NFA {
	return &NFA{transitions: map[State][]Edge{}}
}

// GroupCount 返回正则里捕获分组的总数（不含"整体匹配"的组 0）。
// 例如 "(ab)" 返回 1，"(\d+)-(\d+)" 返回 2，"abc"（无分组）返回 0。
func (n *NFA) GroupCount() int {
	return n.groupCount
}

// Transitions 返回从 s 出发的所有边。
func (n *NFA) Transitions(s State) []Edge {
	return n.transitions[s]
}

// newState 分配一个新状态 ID。
func (n *NFA) newState() State {
	s := n.nextID
	n.nextID++
	return s
}

// addEpsilon 加一条 ε 转移。
func (n *NFA) addEpsilon(from, to State) {
	n.transitions[from] = append(n.transitions[from], Edge{
		To:           to,
		Epsilon:      true,
		CaptureStart: noCapture,
		CaptureEnd:   noCapture,
	})
}

// addCaptureEpsilon 加一条带捕获标记的 ε 转移。
// capStart/capEnd 为 >=0 时分别表示触发"进入/离开第 N 组"，否则传 noCapture。
func (n *NFA) addCaptureEpsilon(from, to State, capStart, capEnd int) {
	n.transitions[from] = append(n.transitions[from], Edge{
		To:           to,
		Epsilon:      true,
		CaptureStart: capStart,
		CaptureEnd:   capEnd,
	})
}

// addChar 加一条字符转移（matcher 判断是否匹配）。
func (n *NFA) addChar(from State, to State, matcher func(rune) bool) {
	n.transitions[from] = append(n.transitions[from], Edge{
		To:           to,
		Epsilon:      false,
		CaptureStart: noCapture,
		CaptureEnd:   noCapture,
		matcher:      matcher,
	})
}

// Build 从 AST 构造 NFA（Thompson 算法）。
func Build(node *ast.Node) *NFA {
	n := New()
	start, accept := n.build(node)
	n.Start = start
	n.Accept = accept
	return n
}

// build 递归构造子 NFA，返回其 start/accept 状态。
func (n *NFA) build(node *ast.Node) (State, State) {
	switch node.Kind {
	case ast.KindLiteral:
		// s --c--> e
		s := n.newState()
		e := n.newState()
		c := node.Char
		n.addChar(s, e, func(r rune) bool { return r == c })
		return s, e

	case ast.KindWildcard:
		// s --任意(非换行)--> e
		s := n.newState()
		e := n.newState()
		n.addChar(s, e, func(r rune) bool { return r != '\n' })
		return s, e

	case ast.KindCharClass:
		// s --[chars]--> e（negated 时反向）
		s := n.newState()
		e := n.newState()
		chars := node.Chars
		negated := node.Negated
		// 用 map 加速查
		set := map[rune]bool{}
		for _, c := range chars {
			set[c] = true
		}
		n.addChar(s, e, func(r rune) bool {
			inSet := set[r]
			if negated {
				return !inSet
			}
			return inSet
		})
		return s, e

	case ast.KindConcat:
		// a.start ... a.accept --ε--> b.start ... b.accept
		aStart, aEnd := n.build(node.Children[0])
		bStart, bEnd := n.build(node.Children[1])
		n.addEpsilon(aEnd, bStart)
		return aStart, bEnd

	case ast.KindAlternate:
		// new start --ε--> a.start / b.start；a.accept/b.accept --ε--> new accept
		s := n.newState()
		e := n.newState()
		aStart, aEnd := n.build(node.Children[0])
		bStart, bEnd := n.build(node.Children[1])
		n.addEpsilon(s, aStart)
		n.addEpsilon(s, bStart)
		n.addEpsilon(aEnd, e)
		n.addEpsilon(bEnd, e)
		return s, e

	case ast.KindStar:
		// s --ε--> a.start；a.accept --ε--> s / e；s --ε--> e
		s := n.newState()
		e := n.newState()
		aStart, aEnd := n.build(node.Children[0])
		n.addEpsilon(s, aStart)
		n.addEpsilon(s, e)    // 0 次
		n.addEpsilon(aEnd, s) // 循环
		n.addEpsilon(aEnd, e)
		return s, e

	case ast.KindPlus:
		// a+ 等价 aa*：必须至少一次
		s := n.newState()
		e := n.newState()
		aStart, aEnd := n.build(node.Children[0])
		n.addEpsilon(s, aStart)
		n.addEpsilon(aEnd, e)
		n.addEpsilon(aEnd, s) // 循环（不再有 s→e 的 0 次跳转）
		return s, e

	case ast.KindQuestion:
		// a? 等价 a|ε
		s := n.newState()
		e := n.newState()
		aStart, aEnd := n.build(node.Children[0])
		n.addEpsilon(s, aStart)
		n.addEpsilon(s, e) // 0 次
		n.addEpsilon(aEnd, e)
		return s, e

	case ast.KindGroup:
		// (a) 的捕获：在子 NFA 外包一层 ε 边，
		// 进入边打 CaptureStart=N，离开边打 CaptureEnd=N。
		// 组号按"左括号出现顺序"（先序）分配：先取号再递归子节点，
		// 这样嵌套 ((ab)) 时外层=1、内层=2（与标准正则一致）。
		groupIndex := n.groupCount
		n.groupCount++
		childStart, childEnd := n.build(node.Children[0])
		s := n.newState()
		e := n.newState()
		n.addCaptureEpsilon(s, childStart, groupIndex, noCapture)
		n.addCaptureEpsilon(childEnd, e, noCapture, groupIndex)
		return s, e

	case ast.KindAnchor:
		// ^ $ 锚点简化处理：作为 ε 转移，匹配时由 matcher 特判
		s := n.newState()
		e := n.newState()
		a := node.Anchor
		n.addChar(s, e, func(r rune) bool {
			// 锚点简化：在 matcher 里通过特殊字符触发位置检查
			// 这里用一个标记字符（匹配器会特殊处理 ^ $）
			_ = a
			return false // 锚点逻辑在 matcher.Match 里处理，这里返回 false 避免 NFA 误消费
		})
		return s, e
	}
	// 不应到达
	s := n.newState()
	return s, s
}
