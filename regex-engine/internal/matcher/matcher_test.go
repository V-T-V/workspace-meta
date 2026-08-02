package matcher

import (
	"testing"

	"github.com/QiuShichang/regex-engine/internal/nfa"
	"github.com/QiuShichang/regex-engine/internal/parser"
)

// compile 是测试辅助：正则字符串 → Matcher。
func compile(t *testing.T, pattern string) *Matcher {
	t.Helper()
	ast, err := parser.Parse(pattern)
	if err != nil {
		t.Fatalf("解析 %q 失败: %v", pattern, err)
	}
	return New(nfa.Build(ast))
}

func TestLiteral(t *testing.T) {
	m := compile(t, "abc")
	if !m.Match("abc") {
		t.Error("abc 应匹配 abc")
	}
	if !m.Match("xabcx") {
		t.Error("abc 应匹配 xabcx 的子串")
	}
	if m.Match("ab") {
		t.Error("ab 不应匹配 abc")
	}
}

func TestWildcard(t *testing.T) {
	m := compile(t, "a.c")
	if !m.Match("abc") {
		t.Error("a.c 应匹配 abc")
	}
	if !m.Match("axc") {
		t.Error("a.c 应匹配 axc")
	}
	if m.Match("ac") {
		t.Error("a.c 不应匹配 ac（. 需要一个字符）")
	}
}

func TestStar(t *testing.T) {
	m := compile(t, "ab*c")
	if !m.Match("ac") {
		t.Error("ab*c 应匹配 ac（b* = 0 次）")
	}
	if !m.Match("abc") {
		t.Error("ab*c 应匹配 abc")
	}
	if !m.Match("abbbbc") {
		t.Error("ab*c 应匹配 abbbbc")
	}
}

func TestPlus(t *testing.T) {
	m := compile(t, "ab+c")
	if m.Match("ac") {
		t.Error("ab+c 不应匹配 ac（+ 至少一次）")
	}
	if !m.Match("abc") {
		t.Error("ab+c 应匹配 abc")
	}
	if !m.Match("abbbbc") {
		t.Error("ab+c 应匹配 abbbbc")
	}
}

func TestQuestion(t *testing.T) {
	m := compile(t, "ab?c")
	if !m.Match("ac") {
		t.Error("ab?c 应匹配 ac（b? = 0 次）")
	}
	if !m.Match("abc") {
		t.Error("ab?c 应匹配 abc（b? = 1 次）")
	}
	if m.Match("abbc") {
		t.Error("ab?c 不应匹配 abbc（b? 至多 1 次）")
	}
}

func TestAlternate(t *testing.T) {
	m := compile(t, "cat|dog")
	if !m.Match("cat") {
		t.Error("cat|dog 应匹配 cat")
	}
	if !m.Match("dog") {
		t.Error("cat|dog 应匹配 dog")
	}
	if m.Match("bird") {
		t.Error("cat|dog 不应匹配 bird")
	}
}

func TestGroup(t *testing.T) {
	m := compile(t, "(ab)+")
	if !m.Match("ab") {
		t.Error("(ab)+ 应匹配 ab")
	}
	if !m.Match("ababab") {
		t.Error("(ab)+ 应匹配 ababab")
	}
	// Match 是子串匹配：aba 含子串 ab，所以 Match 返回 true（正确）
	if !m.Match("aba") {
		t.Error("(ab)+ 应匹配 aba 的子串 ab")
	}
	// 用 IsFullMatch 验证完整匹配：aba 不是 (ab)+ 的完整匹配
	if m.IsFullMatch("aba") {
		t.Error("(ab)+ IsFullMatch 不应匹配 aba（不完整）")
	}
}

func TestCharClass(t *testing.T) {
	m := compile(t, "[abc]")
	for _, s := range []string{"a", "b", "c"} {
		if !m.Match(s) {
			t.Errorf("[abc] 应匹配 %s", s)
		}
	}
	if m.Match("d") {
		t.Error("[abc] 不应匹配 d")
	}
}

func TestCharClassRange(t *testing.T) {
	m := compile(t, "[0-9]+")
	if !m.Match("12345") {
		t.Error("[0-9]+ 应匹配 12345")
	}
	if m.Match("12a45") {
		// 实际上 12 是子串匹配会成功，这里测试语义
		// 12a45 含 12，所以 Match 返回 true（子串匹配）
		// 这是正确的——unanchored
	}
}

func TestNegatedClass(t *testing.T) {
	m := compile(t, "[^0-9]")
	if m.Match("5") {
		t.Error("[^0-9] 不应匹配 5")
	}
	if !m.Match("a") {
		t.Error("[^0-9] 应匹配 a")
	}
}

func TestEscape(t *testing.T) {
	m := compile(t, `a\.b`)
	if !m.Match("a.b") {
		t.Error(`a\.b 应匹配 a.b`)
	}
	if m.Match("axb") {
		t.Error(`a\.b 不应匹配 axb（\. 是字面量 .）`)
	}
}

func TestDigitClass(t *testing.T) {
	m := compile(t, `\d+`)
	if !m.Match("hello123") {
		t.Error(`\d+ 应匹配 hello123 的 123`)
	}
	if m.Match("hello") {
		t.Error(`\d+ 不应匹配 hello`)
	}
}

func TestEmailLike(t *testing.T) {
	// 简化的 email 模式：\w+@\w+\.\w+
	m := compile(t, `\w+@\w+\.\w+`)
	if !m.Match("user@example.com") {
		t.Error("应匹配 email")
	}
	if m.Match("not-an-email") {
		t.Error("不应匹配非 email")
	}
}

func TestNoCatastrophicBacktracking(t *testing.T) {
	// 这个模式在回溯引擎上会指数级慢（ReDoS），但 NFA 状态集合模拟是线性的
	m := compile(t, "(a|a)*b")
	// 一长串 a 不含 b → 不匹配，但应快速返回（不卡死）
	longA := make([]rune, 100)
	for i := range longA {
		longA[i] = 'a'
	}
	if m.Match(string(longA)) {
		t.Error("(a|a)*b 不应匹配纯 a 串")
	}
	// 含 b 应匹配
	if !m.Match(string(longA) + "b") {
		t.Error("(a|a)*b 应匹配 a...ab")
	}
}

func TestIsFullMatch(t *testing.T) {
	m := compile(t, `[0-9]+`)
	if !m.IsFullMatch("12345") {
		t.Error("IsFullMatch 应对纯数字返回 true")
	}
	if m.IsFullMatch("12345a") {
		t.Error("IsFullMatch 应对 12345a 返回 false")
	}
}

// matchesEqual 比较两个 Match 切片（仅 Start/End/Text）。
func matchesEqual(got, want []Match) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Start != want[i].Start || got[i].End != want[i].End || got[i].Text != want[i].Text {
			return false
		}
	}
	return true
}

func TestFindAllLiteral(t *testing.T) {
	m := compile(t, "a")
	got := m.FindAll("banana")
	want := []Match{{1, 2, "a"}, {3, 4, "a"}, {5, 6, "a"}}
	if !matchesEqual(got, want) {
		t.Errorf("FindAll(banana, a) = %+v, want %+v", got, want)
	}
}

func TestFindAllDigits(t *testing.T) {
	m := compile(t, `\d+`)
	got := m.FindAll("a1b22c333")
	want := []Match{{1, 2, "1"}, {3, 5, "22"}, {6, 9, "333"}}
	if !matchesEqual(got, want) {
		t.Errorf("FindAll(a1b22c333, \\d+) = %+v, want %+v", got, want)
	}
}

func TestFindAllStarEmptyNoInfiniteLoop(t *testing.T) {
	// a* 在 baaab 上：空匹配不死循环，且能抓到 "aaa"
	m := compile(t, "a*")
	got := m.FindAll("baaab")
	// 关键断言：必须包含非空匹配 "aaa"（位置 1..4），且能正常返回（不死循环）
	foundAAA := false
	for _, mt := range got {
		if mt.Start == 1 && mt.End == 4 && mt.Text == "aaa" {
			foundAAA = true
		}
		// 所有匹配都不应是负值或越界
		if mt.Start < 0 || mt.End > 5 || mt.Start > mt.End {
			t.Errorf("非法匹配位置: %+v", mt)
		}
	}
	if !foundAAA {
		t.Errorf("FindAll(baaab, a*) 未抓到 aaa: %+v", got)
	}
}

func TestFindAllNoMatch(t *testing.T) {
	m := compile(t, `\d`)
	got := m.FindAll("abc")
	if len(got) != 0 {
		t.Errorf("FindAll(abc, \\d) = %+v, want 空切片", got)
	}
}

func TestFindAllGroup(t *testing.T) {
	// (ab)+ 在 ababab 上：单个完整匹配（最左最长，贪心吞掉全部）
	m := compile(t, "(ab)+")
	got := m.FindAll("ababab")
	want := []Match{{0, 6, "ababab"}}
	if !matchesEqual(got, want) {
		t.Errorf("FindAll(ababab, (ab)+) = %+v, want %+v", got, want)
	}
}

func TestFindAllGreedyLongest(t *testing.T) {
	// 同一起点的贪心取最长：a+ 在 aaa 上应得单个 [0,3)
	m := compile(t, "a+")
	got := m.FindAll("aaa")
	want := []Match{{0, 3, "aaa"}}
	if !matchesEqual(got, want) {
		t.Errorf("FindAll(aaa, a+) = %+v, want %+v (最长匹配)", got, want)
	}
}

func TestFindAllMultipleWords(t *testing.T) {
	// 多次出现的单词："x a b _ a b _ x a b"
	//   索引:       0 1 2 3 4 5 6 7 8 9
	m := compile(t, "ab")
	got := m.FindAll("xab ab xab")
	want := []Match{{1, 3, "ab"}, {4, 6, "ab"}, {8, 10, "ab"}}
	if !matchesEqual(got, want) {
		t.Errorf("FindAll(xab ab xab, ab) = %+v, want %+v", got, want)
	}
}

func TestReplaceAllSimple(t *testing.T) {
	m := compile(t, "cat")
	got := m.ReplaceAll("the cat sat", "dog")
	if got != "the dog sat" {
		t.Errorf("ReplaceAll(the cat sat, cat→dog) = %q, want %q", got, "the dog sat")
	}
}

func TestReplaceAllDigits(t *testing.T) {
	m := compile(t, `\d`)
	got := m.ReplaceAll("a1b2c3", "#")
	if got != "a#b#c#" {
		t.Errorf("ReplaceAll(a1b2c3, \\d→#) = %q, want %q", got, "a#b#c#")
	}
}

func TestReplaceAllNoMatch(t *testing.T) {
	m := compile(t, `\d`)
	got := m.ReplaceAll("abc", "#")
	if got != "abc" {
		t.Errorf("ReplaceAll(abc, \\d→#) = %q, want 原文 %q", got, "abc")
	}
}

func TestReplaceAllEmptyMatches(t *testing.T) {
	// a* 含空匹配：每个空匹配也插入替换串，但不死循环、不漏 "aaa"。
	// b a a a b 在 a* 下产生：[0,0)"" + "b" + [1,4)"aaa" + "b" + [4,4)"" + [5,5)""
	// → "X" "b" "X" "b" "X" "X" = "XbXXbX"（与 Go 标准库 regexp 行为一致）
	m := compile(t, "a*")
	got := m.ReplaceAll("baaab", "X")
	if got != "XbXXbX" {
		t.Errorf("ReplaceAll(baaab, a*→X) = %q, want %q", got, "XbXXbX")
	}
}

func TestReplaceAllEveryChar(t *testing.T) {
	// 把每个字符替换为 "."（. 匹配任意非换行）
	m := compile(t, ".")
	got := m.ReplaceAll("abc", ".")
	if got != "..." {
		t.Errorf("ReplaceAll(abc, .→.) = %q, want %q", got, "...")
	}
}

// groupsEqual 比较两个 GroupMatch 切片。
func groupsEqual(got, want []GroupMatch) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Start != want[i].Start || got[i].End != want[i].End || got[i].Text != want[i].Text {
			return false
		}
	}
	return true
}

func TestFindAllWithGroupsSingle(t *testing.T) {
	// (ab) 在 "xabx" → Groups[1] = {1,3,"ab"}；Groups[0] 是整体匹配
	m := compile(t, "(ab)")
	got := m.FindAllWithGroups("xabx")
	if len(got) != 1 {
		t.Fatalf("(ab) 在 xabx 应得 1 个匹配，实际 %d", len(got))
	}
	want := []GroupMatch{{1, 3, "ab"}, {1, 3, "ab"}}
	if !groupsEqual(got[0].Groups, want) {
		t.Errorf("(ab) 在 xabx 的 Groups = %+v, want %+v", got[0].Groups, want)
	}
}

func TestFindAllWithGroupsTwoGroups(t *testing.T) {
	// (\d+)-(\d+) 在 "12-34" → Groups[1]={0,2,"12"}, Groups[2]={3,5,"34"}
	m := compile(t, `(\d+)-(\d+)`)
	got := m.FindAllWithGroups("12-34")
	if len(got) != 1 {
		t.Fatalf("应得 1 个匹配，实际 %d", len(got))
	}
	want := []GroupMatch{
		{0, 5, "12-34"}, // 组 0 整体
		{0, 2, "12"},    // 组 1
		{3, 5, "34"},    // 组 2
	}
	if !groupsEqual(got[0].Groups, want) {
		t.Errorf("(\\d+)-(\\d+) 的 Groups = %+v, want %+v", got[0].Groups, want)
	}
}

func TestFindAllWithGroupsAdjacentGroups(t *testing.T) {
	// (a)(b) 在 "ab" → Groups[1]={0,1,"a"}, Groups[2]={1,2,"b"}
	m := compile(t, "(a)(b)")
	got := m.FindAllWithGroups("ab")
	if len(got) != 1 {
		t.Fatalf("应得 1 个匹配，实际 %d", len(got))
	}
	want := []GroupMatch{
		{0, 2, "ab"},
		{0, 1, "a"},
		{1, 2, "b"},
	}
	if !groupsEqual(got[0].Groups, want) {
		t.Errorf("(a)(b) 的 Groups = %+v, want %+v", got[0].Groups, want)
	}
}

func TestFindAllWithGroupsNoCaptureGroup(t *testing.T) {
	// 无捕获分组 (abc) 在 abc 上 → Groups 长度 1（只有整体匹配）
	m := compile(t, "(abc)")
	got := m.FindAllWithGroups("abc")
	if len(got) != 1 {
		t.Fatalf("应得 1 个匹配，实际 %d", len(got))
	}
	// (abc) 仍是 1 个捕获组：Groups 长度应为 2（整体 + 组 1）
	// 注：(abc) 是捕获分组（语法里 (...) 默认捕获），所以 Groups = [整体, abc]
	if len(got[0].Groups) != 2 {
		t.Errorf("(abc) 的 Groups 长度应为 2（整体+组1），实际 %d", len(got[0].Groups))
	}
}

func TestFindAllWithGroupsTrulyNoGroups(t *testing.T) {
	// 真正无分组的 abc → Groups 长度 1
	m := compile(t, "abc")
	got := m.FindAllWithGroups("abc")
	if len(got) != 1 {
		t.Fatalf("应得 1 个匹配，实际 %d", len(got))
	}
	if len(got[0].Groups) != 1 {
		t.Errorf("无分组 abc 的 Groups 长度应为 1，实际 %d", len(got[0].Groups))
	}
	want := []GroupMatch{{0, 3, "abc"}}
	if !groupsEqual(got[0].Groups, want) {
		t.Errorf("abc 的 Groups = %+v, want %+v", got[0].Groups, want)
	}
}

func TestFindAllWithGroupsNested(t *testing.T) {
	// 嵌套 ((ab)) → 两组（外层组 1、内层组 2）
	m := compile(t, "((ab))")
	got := m.FindAllWithGroups("ab")
	if len(got) != 1 {
		t.Fatalf("应得 1 个匹配，实际 %d", len(got))
	}
	if len(got[0].Groups) != 3 {
		t.Fatalf("((ab)) 的 Groups 长度应为 3（整体+2 组），实际 %d", len(got[0].Groups))
	}
	want := []GroupMatch{
		{0, 2, "ab"}, // 组 0 整体
		{0, 2, "ab"}, // 组 1 外层 = ab
		{0, 2, "ab"}, // 组 2 内层 = ab
	}
	if !groupsEqual(got[0].Groups, want) {
		t.Errorf("((ab)) 的 Groups = %+v, want %+v", got[0].Groups, want)
	}
}

func TestFindAllWithGroupsRepeated(t *testing.T) {
	// (ab)+ 在 ababab 上：整体匹配 {0,6,"ababab"}，
	// 组 1 捕获最后一次循环的 "ab"（位置 {4,6}，按 ε 边进入/离开时的光标记录）
	m := compile(t, "(ab)+")
	got := m.FindAllWithGroups("ababab")
	if len(got) != 1 {
		t.Fatalf("应得 1 个匹配，实际 %d", len(got))
	}
	want := []GroupMatch{
		{0, 6, "ababab"}, // 组 0 整体
		{4, 6, "ab"},     // 组 1：最后一次进入/离开捕获
	}
	if !groupsEqual(got[0].Groups, want) {
		t.Errorf("(ab)+ 的 Groups = %+v, want %+v", got[0].Groups, want)
	}
}

func TestFindAllWithGroupsMultipleMatches(t *testing.T) {
	// (\d) 在 a1b2 上 → 两个匹配，各自组 1
	m := compile(t, `(\d)`)
	got := m.FindAllWithGroups("a1b2")
	if len(got) != 2 {
		t.Fatalf("应得 2 个匹配，实际 %d", len(got))
	}
	want0 := []GroupMatch{{1, 2, "1"}, {1, 2, "1"}}
	want1 := []GroupMatch{{3, 4, "2"}, {3, 4, "2"}}
	if !groupsEqual(got[0].Groups, want0) {
		t.Errorf("第 1 个匹配 Groups = %+v, want %+v", got[0].Groups, want0)
	}
	if !groupsEqual(got[1].Groups, want1) {
		t.Errorf("第 2 个匹配 Groups = %+v, want %+v", got[1].Groups, want1)
	}
}

func TestFindAllWithGroupsDidNotParticipate(t *testing.T) {
	// (a)(b)? 在 a 上：组 2 未参与匹配 → Start=End=-1, Text=""
	m := compile(t, "(a)(b)?")
	got := m.FindAllWithGroups("a")
	if len(got) != 1 {
		t.Fatalf("应得 1 个匹配，实际 %d", len(got))
	}
	if len(got[0].Groups) != 3 {
		t.Fatalf("Groups 长度应为 3，实际 %d", len(got[0].Groups))
	}
	// 组 2 未参与
	g2 := got[0].Groups[2]
	if g2.Start != -1 || g2.End != -1 || g2.Text != "" {
		t.Errorf("未参与组应为 {-1,-1,\"\"}，实际 %+v", g2)
	}
}

func TestCaseInsensitiveLiteral(t *testing.T) {
	m := compile(t, "(?i)abc")
	for _, s := range []string{"abc", "ABC", "AbC", "aBc", "abC"} {
		if !m.Match(s) {
			t.Errorf("(?i)abc 应匹配 %q", s)
		}
	}
	if !m.Match("xABCy") {
		t.Error("(?i)abc 应匹配子串 xABCy")
	}
	if m.Match("abd") {
		t.Error("(?i)abc 不应匹配 abd")
	}
	if m.Match("AB") {
		t.Error("(?i)abc 不应匹配 AB（长度不足）")
	}
}

func TestCaseInsensitiveWord(t *testing.T) {
	m := compile(t, "(?i)hello")
	if !m.Match("HELLO") {
		t.Error("(?i)hello 应匹配 HELLO")
	}
	if !m.Match("HeLLo") {
		t.Error("(?i)hello 应匹配 HeLLo")
	}
	if !m.Match("hello") {
		t.Error("(?i)hello 应匹配 hello")
	}
	if m.Match("hell") {
		t.Error("(?i)hello 不应匹配 hell")
	}
}

func TestCaseInsensitiveCharClass(t *testing.T) {
	// (?i)[a-c]x 应同时匹配大小写范围内的字符 + x。
	m := compile(t, "(?i)[a-c]x")
	for _, s := range []string{"ax", "Bx", "cx", "AX", "Cx"} {
		if !m.Match(s) {
			t.Errorf("(?i)[a-c]x 应匹配 %q", s)
		}
	}
	if m.Match("dx") {
		t.Error("(?i)[a-c]x 不应匹配 dx（d 不在 [a-c]）")
	}
}

func TestCaseInsensitiveNegatedClass(t *testing.T) {
	// (?i)[^abc] 取反也应不区分大小写：A/B/C 都应被排除。
	m := compile(t, "(?i)[^abc]")
	if m.Match("A") {
		t.Error("(?i)[^abc] 不应匹配 A（A 等价 a）")
	}
	if !m.Match("d") {
		t.Error("(?i)[^abc] 应匹配 d")
	}
}

func TestCaseInsensitiveIsFullMatch(t *testing.T) {
	m := compile(t, "(?i)abc")
	if !m.IsFullMatch("AbC") {
		t.Error("(?i)abc 应全匹配 AbC")
	}
	if m.IsFullMatch("AbCx") {
		t.Error("(?i)abc 不应全匹配 AbCx")
	}
}

func TestCaseInsensitiveWithoutFlag(t *testing.T) {
	// 没有 (?i) 时仍区分大小写。
	m := compile(t, "abc")
	if m.Match("ABC") {
		t.Error("abc（无 (?i)）不应匹配 ABC")
	}
	if !m.Match("abc") {
		t.Error("abc 应匹配 abc")
	}
}

// -----------------------------------------------------------------------------
// 非贪婪量词 *? +? ??
// -----------------------------------------------------------------------------

func TestLazyStarShortest(t *testing.T) {
	// a.*?b 在 "axbxxb" → 非贪婪取最短 "axb"（不是 "axbxxb"）。
	m := compile(t, "a.*?b")
	got := m.FindAll("axbxxb")
	want := []Match{{0, 3, "axb"}}
	if !matchesEqual(got, want) {
		t.Errorf("FindAll(axbxxb, a.*?b) = %+v, want %+v", got, want)
	}
}

func TestLazyStarVsGreedy(t *testing.T) {
	// 对比：贪婪 a.*b 取最长 "axbxxb"，非贪婪 a.*?b 取最短 "axb"。
	mlazy := compile(t, "a.*?b")
	mgreedy := compile(t, "a.*b")
	lazy := mlazy.FindAll("axbxxb")
	greedy := mgreedy.FindAll("axbxxb")
	if len(lazy) != 1 || lazy[0].Text != "axb" {
		t.Errorf("非贪婪 a.*?b 应得 [axb]，实际 %+v", lazy)
	}
	if len(greedy) != 1 || greedy[0].Text != "axbxxb" {
		t.Errorf("贪婪 a.*b 应得 [axbxxb]，实际 %+v", greedy)
	}
}

func TestLazyPlusShortest(t *testing.T) {
	// a.+?b 在 "axbxxb" → 非贪婪至少 1 字符 → "axb"。
	m := compile(t, "a.+?b")
	got := m.FindAll("axbxxb")
	want := []Match{{0, 3, "axb"}}
	if !matchesEqual(got, want) {
		t.Errorf("FindAll(axbxxb, a.+?b) = %+v, want %+v", got, want)
	}
}

func TestLazyPlusVsGreedy(t *testing.T) {
	mlazy := compile(t, "a.+?b")
	mgreedy := compile(t, "a.+b")
	lazy := mlazy.FindAll("axbxxb")
	greedy := mgreedy.FindAll("axbxxb")
	if len(lazy) != 1 || lazy[0].Text != "axb" {
		t.Errorf("非贪婪 a.+?b 应得 [axb]，实际 %+v", lazy)
	}
	if len(greedy) != 1 || greedy[0].Text != "axbxxb" {
		t.Errorf("贪婪 a.+b 应得 [axbxxb]，实际 %+v", greedy)
	}
}

func TestLazyQuestionShortest(t *testing.T) {
	// ba?? 在 "ba" 上：非贪婪优先匹配 0 次 a → "b"（不是 "ba"）。
	// 选这个例子是因为从同一起点 0，0 次与 1 次两种路径都能到达接受态，
	// 非贪婪取最近（最短）。
	m := compile(t, "ba??")
	got := m.FindAll("ba")
	want := []Match{{0, 1, "b"}}
	if !matchesEqual(got, want) {
		t.Errorf("FindAll(ba, ba??) = %+v, want %+v（优先 0 次）", got, want)
	}
}

func TestLazyQuestionVsGreedy(t *testing.T) {
	// 贪婪 ba? 优先匹配 1 次 a → "ba"；非贪婪 ba?? 优先 0 次 → "b"。
	mgreedy := compile(t, "ba?")
	mlazy := compile(t, "ba??")
	greedy := mgreedy.FindAll("ba")
	lazy := mlazy.FindAll("ba")
	if len(greedy) != 1 || greedy[0].Text != "ba" {
		t.Errorf("贪婪 ba? 应得 [ba]，实际 %+v", greedy)
	}
	if len(lazy) != 1 || lazy[0].Text != "b" {
		t.Errorf("非贪婪 ba?? 应得 [b]，实际 %+v", lazy)
	}
}

func TestLazyMultipleMatches(t *testing.T) {
	// a.*?b 在 "a1b a2b" 上：两次非贪婪匹配 "a1b"、"a2b"。
	m := compile(t, "a.*?b")
	got := m.FindAll("a1b a2b")
	want := []Match{{0, 3, "a1b"}, {4, 7, "a2b"}}
	if !matchesEqual(got, want) {
		t.Errorf("FindAll(a1b a2b, a.*?b) = %+v, want %+v", got, want)
	}
}

func TestLazyGreedyMixed(t *testing.T) {
	// <.*?> 匹配 HTML 标签：非贪婪，每个标签单独。
	m := compile(t, "<.*?>")
	got := m.FindAll("<a><b>")
	want := []Match{{0, 3, "<a>"}, {3, 6, "<b>"}}
	if !matchesEqual(got, want) {
		t.Errorf("FindAll(<a><b>, <.*?>) = %+v, want %+v", got, want)
	}
}

func TestLazyStarEmptyMatch(t *testing.T) {
	// a*? 可匹配空串且不死循环：在 "b" 上应得到一个空匹配后前进。
	m := compile(t, "a*?")
	got := m.FindAll("b")
	for _, mt := range got {
		if mt.Start < 0 || mt.End > 1 || mt.Start > mt.End {
			t.Errorf("非法匹配位置: %+v", mt)
		}
	}
	if len(got) == 0 {
		t.Error("a*? 在 'b' 上至少应有一个空匹配")
	}
}

func TestLazyWithGroups(t *testing.T) {
	// (a)(.*?)(b) 在 "axxxb" → 非贪婪 .*? 取最少，组 2 = "xxx"。
	m := compile(t, "(a)(.*?)(b)")
	got := m.FindAllWithGroups("axxxb")
	if len(got) != 1 {
		t.Fatalf("应得 1 个匹配，实际 %d", len(got))
	}
	want := []GroupMatch{
		{0, 5, "axxxb"}, // 组 0 整体
		{0, 1, "a"},     // 组 1
		{1, 4, "xxx"},   // 组 2（非贪婪仍需吞到 b 前）
		{4, 5, "b"},     // 组 3
	}
	if !groupsEqual(got[0].Groups, want) {
		t.Errorf("(a)(.*?)(b) 的 Groups = %+v, want %+v", got[0].Groups, want)
	}
}

func TestLazyMatchStillWorks(t *testing.T) {
	// 非贪婪量词不应破坏存在性匹配。
	m := compile(t, "a.*?b")
	if !m.Match("axxxb") {
		t.Error("a.*?b 应匹配 axxxb")
	}
	if m.Match("axxx") {
		t.Error("a.*?b 不应匹配 axxx（无结尾 b）")
	}
}

func TestCompile(t *testing.T) {
	m, err := Compile("abc")
	if err != nil {
		t.Fatalf("Compile 失败: %v", err)
	}
	if !m.Match("xabcx") {
		t.Error("Compile 后的 Matcher 应能匹配")
	}
}

func TestCompileError(t *testing.T) {
	_, err := Compile("[abc")
	if err == nil {
		t.Error("非法正则应返回 error")
	}
}

// ===== 锚点 ^ $ =====

func TestAnchorStartMatch(t *testing.T) {
	// ^abc：只在行首匹配
	m := compile(t, "^abc")
	if !m.Match("abc") {
		t.Error("^abc 应匹配 abc")
	}
	if !m.Match("abcx") {
		t.Error("^abc 应匹配 abcx 的前缀")
	}
	if m.Match("xabc") {
		t.Error("^abc 不应匹配 xabc（abc 不在行首）")
	}
}

func TestAnchorEndMatch(t *testing.T) {
	// abc$：只在行尾匹配
	m := compile(t, "abc$")
	if !m.Match("abc") {
		t.Error("abc$ 应匹配 abc")
	}
	if !m.Match("xabc") {
		t.Error("abc$ 应匹配 xabc 的后缀")
	}
	if m.Match("abcx") {
		t.Error("abc$ 不应匹配 abcx（abc 不在行尾）")
	}
}

func TestAnchorBothMatch(t *testing.T) {
	// ^abc$：整串恰好 abc
	m := compile(t, "^abc$")
	if !m.Match("abc") {
		t.Error("^abc$ 应匹配 abc")
	}
	if m.Match("xabc") {
		t.Error("^abc$ 不应匹配 xabc")
	}
	if m.Match("abcx") {
		t.Error("^abc$ 不应匹配 abcx")
	}
}

func TestAnchorFindAll(t *testing.T) {
	// ^\d+：每行的行首数字串。FindAll 从 start=0 扫描，^ 只在 start==0
	// 或前一字符为 \n 时成立，故只在文本开头和 \n 后匹配。
	m := compile(t, `^\d+`)
	got := m.FindAll("12abc\n34def")
	// 期望两处匹配："12"（位置 0）和 "34"（位置 6，紧跟 \n 后）
	want := []Match{{0, 2, "12"}, {6, 8, "34"}}
	if !matchesEqual(got, want) {
		t.Errorf("FindAll(^\\d+) = %+v, want %+v", got, want)
	}
}

func TestAnchorEndFindAll(t *testing.T) {
	// \d+$：每行行尾数字。$ 在 pos==len 或当前字符 \n 时成立。
	m := compile(t, `\d+$`)
	got := m.FindAll("ab12\ncd34")
	// "12"（位置 2..4，其后是 \n）和 "34"（位置 7..9，到结尾）
	want := []Match{{2, 4, "12"}, {7, 9, "34"}}
	if !matchesEqual(got, want) {
		t.Errorf("FindAll(\\d+$) = %+v, want %+v", got, want)
	}
}

func TestAnchorIsFullMatch(t *testing.T) {
	// ^abc$ 应等价于 IsFullMatch 语义
	m := compile(t, "^abc$")
	if !m.Match("abc") {
		t.Error("^abc$ 应匹配 abc")
	}
	// 注意：因为 ^ 和 $ 都锚定，对 xabc 应整体不匹配
	if m.Match("xabc") {
		t.Error("^abc$ 不应匹配 xabc")
	}
}

func TestAnchorInGroup(t *testing.T) {
	// 锚点在分组内也要工作
	m := compile(t, "(^abc)")
	if !m.Match("abc") {
		t.Error("(^abc) 应匹配 abc")
	}
	if m.Match("xabc") {
		t.Error("(^abc) 不应匹配 xabc")
	}
}

func TestAnchorMultilineDollar(t *testing.T) {
	// $ 在多行下认 \n 边界：a$ 应在 "xa\nyb" 里匹配 a（其后是 \n）
	m := compile(t, "a$")
	if !m.Match("xa\nyb") {
		t.Error("a$ 应匹配 'xa\nyb' 里的 a（a 后跟 \n）")
	}
	// 但不应匹配没有行尾边界的中间 a
	if m.Match("xayb") {
		t.Error("a$ 不应匹配 xayb（a 不在行尾）")
	}
}

func TestAnchorStartMultiline(t *testing.T) {
	// ^ 在多行下认 \n 边界：^b 应在 "a\nbc" 里匹配 b（前是 \n）
	m := compile(t, "^b")
	if !m.Match("a\nbc") {
		t.Error("^b 应匹配 'a\nbc' 里的 b（前是 \n）")
	}
	if m.Match("abc") {
		t.Error("^b 不应匹配 abc（b 不在行首）")
	}
}
