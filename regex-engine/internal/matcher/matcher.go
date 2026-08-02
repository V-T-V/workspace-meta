// Package matcher 用状态集合模拟 NFA 执行，实现正则匹配。
//
// 算法：维护"当前可能的状态集合"，每消费一个输入字符，把所有状态沿匹配的边推进，
// 并对结果集求 ε 闭包。若任意时刻接受状态在集合中，则匹配成功。
//
// 这种"状态集合模拟"比回溯法（backtracking）更优：
//   - 无回溯，避免指数级最坏情况（如 (a|a)*b 对 aaaaaaaaaa 失配时）
//   - 时间复杂度 O(n*m)（n 输入长度，m NFA 状态数）
//
// 参考：Russ Cox "Regular Expression Matching in the Wild"。
package matcher

import (
	"strings"

	"github.com/QiuShichang/regex-engine/internal/nfa"
	"github.com/QiuShichang/regex-engine/internal/parser"
)

// Matcher 持有编译好的 NFA。
type Matcher struct {
	nfa *nfa.NFA
}

// New 从 NFA 构造 Matcher。
func New(n *nfa.NFA) *Matcher {
	return &Matcher{nfa: n}
}

// Match 报告输入 text 是否包含匹配正则的子串（unanchored）。
// 若想全匹配（anchored），用 IsFullMatch。
func (m *Matcher) Match(text string) bool {
	// 尝试从每个位置开始匹配
	runes := []rune(text)
	for start := 0; start <= len(runes); start++ {
		if m.matchFrom(runes, start) {
			return true
		}
	}
	return false
}

// IsFullMatch 报告整个 text 是否完整匹配正则（从开头到结尾，等价 ^regex$）。
// 实现：从 start 开始跑 NFA 到结尾，验证最终状态集合含接受状态。
func (m *Matcher) IsFullMatch(text string) bool {
	runes := []rune(text)
	current := m.epsilonClosure(map[nfa.State]bool{m.nfa.Start: true}, runes, 0)
	for i := 0; i < len(runes); i++ {
		next := map[nfa.State]bool{}
		for s := range current {
			for _, edge := range m.nfa.Transitions(s) {
				if edge.Match(runes[i]) {
					next[edge.To] = true
				}
			}
		}
		if len(next) == 0 {
			return false
		}
		current = m.epsilonClosure(next, runes, i+1)
	}
	return current[m.nfa.Accept]
}

// matchFrom 从 start 位置开始，跑 NFA，报告是否能到达接受状态。
// 用于 Match（子串匹配）：尝试从每个 start 位置找最早匹配。
func (m *Matcher) matchFrom(runes []rune, start int) bool {
	current := m.epsilonClosure(map[nfa.State]bool{m.nfa.Start: true}, runes, start)
	// start 位置就可能是接受（如 a* 对空串匹配）
	if current[m.nfa.Accept] {
		return true
	}
	for i := start; i < len(runes); i++ {
		next := map[nfa.State]bool{}
		for s := range current {
			for _, edge := range m.nfa.Transitions(s) {
				if edge.Match(runes[i]) {
					next[edge.To] = true
				}
			}
		}
		if len(next) == 0 {
			return false
		}
		current = m.epsilonClosure(next, runes, i+1)
		if current[m.nfa.Accept] {
			return true
		}
	}
	return current[m.nfa.Accept]
}

// Match 代表一次匹配的结果（位置 + 文本）。
type Match struct {
	Start int    // 匹配起始 rune 位置
	End   int    // 匹配结束 rune 位置（不含）
	Text  string // 匹配到的文本
}

// FindAll 返回输入中所有不重叠的匹配。
// 从左到右扫描，找到一个匹配后从其后继续。
// 量词语义：
//   - 全贪婪（无 *? +? ??）→ "最左最长"：对同一起始位置取能到达的最远接受点；
//   - 含非贪婪 → "最左最短"：对同一起始位置取能到达的最近接受点（惰性优先）。
func (m *Matcher) FindAll(text string) []Match {
	runes := []rune(text)
	var results []Match
	start := 0
	for start <= len(runes) {
		var end int
		if m.nfa.HasLazy() {
			end = m.matchShortestFrom(runes, start)
		} else {
			end = m.matchLongestFrom(runes, start)
		}
		if end >= 0 {
			// 找到匹配：记录并从 end 继续（不重叠）
			results = append(results, Match{
				Start: start,
				End:   end,
				Text:  string(runes[start:end]),
			})
			if end == start {
				// 空匹配（如 a* 在非 a 处匹配空串）：强制前进 1 避免死循环
				start++
			} else {
				start = end
			}
		} else {
			// 此 start 无匹配，从下一位置再试
			start++
		}
	}
	return results
}

// ReplaceAll 把所有匹配替换为 replacement，返回替换后的字符串。
// replacement 是普通字符串（不支持 $1 反向引用，M2 候选）。
func (m *Matcher) ReplaceAll(text, replacement string) string {
	matches := m.FindAll(text)
	if len(matches) == 0 {
		return text
	}
	runes := []rune(text)
	var out []rune
	cursor := 0 // 下一个未拷贝的 rune 位置
	for _, mt := range matches {
		// 拷贝匹配前的未匹配部分
		out = append(out, runes[cursor:mt.Start]...)
		// 拼入替换串
		out = append(out, []rune(replacement)...)
		cursor = mt.End
	}
	// 拷贝尾部未匹配部分
	out = append(out, runes[cursor:]...)
	return string(out)
}

// Split 按正则分割字符串，返回分割后的子串切片。
// 连续匹配产生空串（与 Go 标准库 regexp.Split 行为一致）。
//
// 参数 n 控制返回的子串数：
//   - n < 0：返回所有子串（末尾不截断）。
//   - n == 0：返回 nil（与 regexp.Split 行为一致——0 被视作无意义请求）。
//   - n > 0：最多返回 n 个子串；最后一个子串是"第 n-1 次分割点之后未再分割的剩余文本"。
//
// 与标准库语义一致：分隔符不包含在结果里；首尾的匹配分别产生一个空串（前缀/后缀）。
// 例如用 \s+ 分割 "a  b" 得到 ["a","b"]；分割 " a " 得到 ["","a",""]。
func (m *Matcher) Split(text string, n int) []string {
	if n == 0 {
		return nil
	}
	matches := m.FindAll(text)
	// n > 0 时：至多再切 (n-1) 刀，超出范围的匹配不再切分。
	cut := len(matches)
	if n > 0 && cut > n-1 {
		cut = n - 1
	}

	runes := []rune(text)
	out := make([]string, 0, cut+1)
	cursor := 0 // 下一个未拷贝的 rune 位置
	for i := 0; i < cut; i++ {
		mt := matches[i]
		out = append(out, string(runes[cursor:mt.Start]))
		cursor = mt.End
	}
	// 拼接末尾剩余（含未参与切分的匹配及其后的文本）
	out = append(out, string(runes[cursor:]))
	return out
}

// matchLongestFrom 从 start 位置开始跑 NFA，返回能到达接受状态的最远位置（不含）。
// 若从 start 起任何时刻（含 start 本身，如 a* 对空串）都不能接受，返回 -1。
// 这是 POSIX "最左最长" 语义：记录最后一次进入接受态的位置。
func (m *Matcher) matchLongestFrom(runes []rune, start int) int {
	current := m.epsilonClosure(map[nfa.State]bool{m.nfa.Start: true}, runes, start)
	lastAccept := -1
	if current[m.nfa.Accept] {
		// start 位置即接受（空匹配可能）
		lastAccept = start
	}
	for i := start; i < len(runes); i++ {
		next := map[nfa.State]bool{}
		for s := range current {
			for _, edge := range m.nfa.Transitions(s) {
				if edge.Match(runes[i]) {
					next[edge.To] = true
				}
			}
		}
		if len(next) == 0 {
			break
		}
		current = m.epsilonClosure(next, runes, i+1)
		if current[m.nfa.Accept] {
			lastAccept = i + 1
		}
	}
	return lastAccept
}

// matchShortestFrom 从 start 位置开始跑 NFA，返回能到达接受状态的最近位置（不含）。
// 即第一个（最早的）接受点——对应非贪婪语义：优先匹配最少字符。
// 若从 start 起任何时刻（含 start 本身，如 a*? 对空串）都不能接受，返回 -1。
//
// 注意：纯 NFA 状态集合模拟只能保证"是否存在接受路径"，无法表达"先尝试不消费再
// 尝试消费"的回溯式惰性。因此这里采用"第一次进入接受态即返回"的近似：对 a*?、a+?、
// a?? 这类惰性量词，它给出与标准回溯引擎一致的"最短左匹配"结果。
func (m *Matcher) matchShortestFrom(runes []rune, start int) int {
	current := m.epsilonClosure(map[nfa.State]bool{m.nfa.Start: true}, runes, start)
	if current[m.nfa.Accept] {
		return start // start 位置即接受（空匹配可能）
	}
	for i := start; i < len(runes); i++ {
		next := map[nfa.State]bool{}
		for s := range current {
			for _, edge := range m.nfa.Transitions(s) {
				if edge.Match(runes[i]) {
					next[edge.To] = true
				}
			}
		}
		if len(next) == 0 {
			return -1
		}
		current = m.epsilonClosure(next, runes, i+1)
		if current[m.nfa.Accept] {
			return i + 1
		}
	}
	return -1
}

// reachesAccept 已删除（合并到 IsFullMatch）。

// epsilonClosure 求状态集合的 ε 闭包：从这些状态出发，仅通过 ε 边能到达的所有状态。
// pos 是当前光标位置（下一个待消费字符的下标，可为 len(runes)）；
// 锚点边 ^ / $ 用它做位置判定（^ 行首 / $ 行尾）。
// 用 BFS 实现。
func (m *Matcher) epsilonClosure(states map[nfa.State]bool, runes []rune, pos int) map[nfa.State]bool {
	closure := map[nfa.State]bool{}
	var queue []nfa.State
	for s := range states {
		closure[s] = true
		queue = append(queue, s)
	}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		for _, edge := range m.nfa.Transitions(s) {
			if !edge.Epsilon {
				continue
			}
			if edge.IsAnchor && !anchorHolds(edge.Anchor, runes, pos) {
				// 锚点位置条件不满足：此 ε 边不放行。
				continue
			}
			if !closure[edge.To] {
				closure[edge.To] = true
				queue = append(queue, edge.To)
			}
		}
	}
	return closure
}

// anchorHolds 判断锚点（'^' 或 '$'）在光标位置 pos 是否成立。
//
//	^ 行首：pos==0 或前一字符是 '\n'
//	$ 行尾：pos==len(runes) 或当前字符是 '\n'
//
// 这是多行模式（multiline）语义——单行模式下 $ 通常也允许结尾 \n，
// 此处统一采用 ^/$ 都认 \n 边界的宽松定义，与多数正则引擎的默认多行行为一致。
func anchorHolds(anchor byte, runes []rune, pos int) bool {
	switch anchor {
	case '^':
		if pos == 0 {
			return true
		}
		return runes[pos-1] == '\n'
	case '$':
		if pos == len(runes) {
			return true
		}
		return runes[pos] == '\n'
	}
	return false
}

// GroupMatch 是一个捕获组的匹配结果。
type GroupMatch struct {
	Start int    // 组起始 rune 位置（含）；未匹配时为 -1
	End   int    // 组结束 rune 位置（不含）；未匹配时为 -1
	Text  string // 组匹配到的文本（未匹配时为空串）
}

// MatchWithGroups 是含分组捕获的匹配结果。
// Groups[0] 是整体匹配（等价于 Start/End/Text）；
// Groups[1..] 是各捕获子组，按左括号出现顺序排列。
type MatchWithGroups struct {
	Start  int
	End    int
	Text   string
	Groups []GroupMatch
}

// FindAllWithGroups 返回所有不重叠的匹配，并提取每个捕获组的内容。
// 量词语义与 FindAll 一致：全贪婪取最长、含非贪婪取最短，且不重叠。
// 无捕获分组的正则：Groups 仅含整体匹配（长度 1）。
func (m *Matcher) FindAllWithGroups(text string) []MatchWithGroups {
	runes := []rune(text)
	gc := m.nfa.GroupCount() // 子组数量（不含组 0）
	var results []MatchWithGroups
	start := 0
	for start <= len(runes) {
		var end int
		var caps []int
		if m.nfa.HasLazy() {
			end, caps = m.matchShortestWithGroupsFrom(runes, start)
		} else {
			end, caps = m.matchLongestWithGroupsFrom(runes, start)
		}
		if end >= 0 {
			groups := buildGroups(runes, caps, gc)
			groups[0] = GroupMatch{Start: start, End: end, Text: string(runes[start:end])}
			results = append(results, MatchWithGroups{
				Start:  start,
				End:    end,
				Text:   string(runes[start:end]),
				Groups: groups,
			})
			if end == start {
				start++ // 空匹配：强制前进避免死循环
			} else {
				start = end
			}
		} else {
			start++
		}
	}
	return results
}

// matchLongestWithGroupsFrom 从 start 位置跑 NFA，返回最远接受位置及该次接受时的捕获快照。
// 无人接受时 end=-1，caps 为 nil。
//
// 实现：状态集合模拟，但每个活跃状态携带自己的捕获快照（[]int，长度 2*groupCount，
// caps[2i]=第 i 组起始、caps[2i+1]=第 i 组结束，-1 表示未设置）。经过 CaptureStart=N
// 的 ε 边时把 caps[2N]=当前光标位置；经过 CaptureEnd=N 时把 caps[2N+1]=当前光标位置。
// 对同一状态，第一条到达它的路径胜出（leftmost 语义），与 Thompson 模拟一致。
func (m *Matcher) matchLongestWithGroupsFrom(runes []rune, start int) (int, []int) {
	gc := m.nfa.GroupCount()
	newCaps := func() []int {
		if gc == 0 {
			return nil
		}
		c := make([]int, 2*gc)
		for i := range c {
			c[i] = -1
		}
		return c
	}

	// frontier: state -> 捕获快照
	frontier := map[nfa.State][]int{m.nfa.Start: newCaps()}
	frontier = m.closureWithCaps(frontier, start, runes)

	lastAccept := -1
	var lastCaps []int
	if caps, ok := frontier[m.nfa.Accept]; ok {
		lastAccept = start
		lastCaps = cloneCaps(caps)
	}

	for i := start; i < len(runes); i++ {
		next := map[nfa.State][]int{}
		for s, caps := range frontier {
			for _, edge := range m.nfa.Transitions(s) {
				if edge.Epsilon {
					continue
				}
				if edge.Match(runes[i]) {
					if _, seen := next[edge.To]; !seen {
						next[edge.To] = cloneCaps(caps) // 消费字符不改变捕获，但要独立副本
					}
				}
			}
		}
		if len(next) == 0 {
			break
		}
		frontier = m.closureWithCaps(next, i+1, runes)
		if caps, ok := frontier[m.nfa.Accept]; ok {
			lastAccept = i + 1
			lastCaps = cloneCaps(caps)
		}
	}
	if lastAccept < 0 {
		return -1, nil
	}
	return lastAccept, lastCaps
}

// matchShortestWithGroupsFrom 从 start 位置跑 NFA，返回第一个（最近的）接受位置及此刻捕获快照。
// 无人接受时 end=-1，caps 为 nil。对应非贪婪语义：优先匹配最少字符。
func (m *Matcher) matchShortestWithGroupsFrom(runes []rune, start int) (int, []int) {
	gc := m.nfa.GroupCount()
	newCaps := func() []int {
		if gc == 0 {
			return nil
		}
		c := make([]int, 2*gc)
		for i := range c {
			c[i] = -1
		}
		return c
	}

	// frontier: state -> 捕获快照
	frontier := map[nfa.State][]int{m.nfa.Start: newCaps()}
	frontier = m.closureWithCaps(frontier, start, runes)
	if caps, ok := frontier[m.nfa.Accept]; ok {
		return start, cloneCaps(caps) // start 位置即接受
	}

	for i := start; i < len(runes); i++ {
		next := map[nfa.State][]int{}
		for s, caps := range frontier {
			for _, edge := range m.nfa.Transitions(s) {
				if edge.Epsilon {
					continue
				}
				if edge.Match(runes[i]) {
					if _, seen := next[edge.To]; !seen {
						next[edge.To] = cloneCaps(caps)
					}
				}
			}
		}
		if len(next) == 0 {
			return -1, nil
		}
		frontier = m.closureWithCaps(next, i+1, runes)
		if caps, ok := frontier[m.nfa.Accept]; ok {
			return i + 1, cloneCaps(caps) // 第一次接受即返回
		}
	}
	return -1, nil
}

// closureWithCaps 求带捕获快照的 ε 闭包。pos 是当前光标位置（捕获事件记录此值）。
// runes 用于锚点边的位置判定（^ / $）。
// BFS：弹出 (s, caps)，对其每条 ε 边：
//   - 锚点边（IsAnchor）：仅当 anchorHolds 时通过，caps 不变
//   - CaptureStart=N：newcaps = caps 副本，置 newcaps[2N]=pos
//   - CaptureEnd=N：newcaps = caps 副本，置 newcaps[2N+1]=pos
//   - 无标记：直接复用 caps
//
// 若目标状态已存在，保留先到的那条（first-wins）。
func (m *Matcher) closureWithCaps(seed map[nfa.State][]int, pos int, runes []rune) map[nfa.State][]int {
	closure := map[nfa.State][]int{}
	var queue []nfa.State
	for s, caps := range seed {
		closure[s] = caps
		queue = append(queue, s)
	}
	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]
		caps := closure[s]
		for _, edge := range m.nfa.Transitions(s) {
			if !edge.Epsilon {
				continue
			}
			// 锚点边：位置不满足则不放行。
			if edge.IsAnchor && !anchorHolds(edge.Anchor, runes, pos) {
				continue
			}
			var nextCaps []int
			if edge.CaptureStart >= 0 || edge.CaptureEnd >= 0 {
				nextCaps = cloneCaps(caps)
				if edge.CaptureStart >= 0 {
					nextCaps[2*edge.CaptureStart] = pos
				}
				if edge.CaptureEnd >= 0 {
					nextCaps[2*edge.CaptureEnd+1] = pos
				}
			} else {
				nextCaps = caps // 无变化，直接共享
			}
			if _, seen := closure[edge.To]; !seen {
				closure[edge.To] = nextCaps
				queue = append(queue, edge.To)
			}
		}
	}
	return closure
}

// cloneCaps 复制捕获快照（nil 安全）。
func cloneCaps(caps []int) []int {
	if caps == nil {
		return nil
	}
	out := make([]int, len(caps))
	copy(out, caps)
	return out
}

// buildGroups 由捕获快照、子组数组装出 GroupMatch 切片。
// Groups[0] 占位为空 GroupMatch（由调用方填整体匹配）；Groups[1+i] 对应第 i 个子组。
func buildGroups(runes []rune, caps []int, gc int) []GroupMatch {
	groups := make([]GroupMatch, 1, gc+1) // Groups[0] 留给整体匹配，调用方覆盖
	for i := 0; i < gc; i++ {
		g := GroupMatch{Start: -1, End: -1}
		if caps != nil {
			s := caps[2*i]
			e := caps[2*i+1]
			if s >= 0 && e >= 0 && e >= s {
				g.Start = s
				g.End = e
				g.Text = string(runes[s:e])
			}
		}
		groups = append(groups, g)
	}
	return groups
}

// Compile 一步编译正则字符串为 Matcher（parser.Parse + nfa.Build + matcher.New）。
func Compile(pattern string) (*Matcher, error) {
	ast, err := parser.Parse(pattern)
	if err != nil {
		return nil, err
	}
	return New(nfa.Build(ast)), nil
}

// MatchString 报告 text 是否匹配 pattern（一步编译+匹配，unanchored 子串匹配）。
//
// 适用于"只匹配一次就丢弃"的简单场景；若需要对同一模式多次匹配，应先用
// Compile 编译出 Matcher 复用，避免重复解析/构建 NFA。
//
// 等价于：
//
//	m, err := Compile(pattern)
//	if err != nil { return false, err }
//	return m.Match(text), nil
func MatchString(pattern, text string) (bool, error) {
	m, err := Compile(pattern)
	if err != nil {
		return false, err
	}
	return m.Match(text), nil
}

// MustMatchString 同 MatchString，但编译失败时 panic 而非返回 error。
//
// 适用于 pattern 是编译期常量、不可能非法的场景（如测试、表驱动用例），
// 可省去错误处理。运行期外部输入的 pattern 不要用本函数。
func MustMatchString(pattern, text string) bool {
	m, err := Compile(pattern)
	if err != nil {
		panic(err)
	}
	return m.Match(text)
}

// metaChars 是本引擎识别的正则元字符。Quote 会在它们前面加反斜杠转义。
// 与 Go 标准库 regexp.QuoteMeta 的字符集保持一致。
const metaChars = `\.+*?()|[]{}^$`

// isMetaByte 报告 b 是否为正则元字符。
// 用查表替代线性扫描，让 Quote 在长字符串上保持 O(n)。
var isMetaByte [128]bool

func init() {
	for _, c := range metaChars {
		isMetaByte[c] = true
	}
}

// Quote 返回一个匹配字面量字符串 s 的正则表达式。
// 把所有正则元字符转义：. * + ? ( ) [ ] { } | \ ^ $
//
// 典型用途：把用户输入（如搜索关键字、文件名片段）安全地嵌入正则，
// 避免其中的元字符改变匹配语义。
//
//	Quote("1+1=2")  → `1\+1=2`
//	Quote("(a)b")   → `\(a\)b`
//	Quote(`a\b`)    → `a\\b`
//
// 非 ASCII（rune >= 128）字符不可能是 ASCII 元字符，原样透传。
func Quote(s string) string {
	// 预扫描：若没有任何元字符，直接返回原串，避免无谓分配。
	var needed int
	for i := 0; i < len(s); i++ {
		if s[i] < 128 && isMetaByte[s[i]] {
			needed++
		}
	}
	if needed == 0 {
		return s
	}
	// 每个元字符前插一个 '\'，结果长度 = len(s) + needed
	var b strings.Builder
	b.Grow(len(s) + needed)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 128 && isMetaByte[c] {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return b.String()
}
