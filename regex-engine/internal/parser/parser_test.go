package parser

import (
	"testing"

	"github.com/QiuShichang/regex-engine/internal/ast"
)

func TestParseLiteral(t *testing.T) {
	n, err := Parse("a")
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != ast.KindLiteral || n.Char != 'a' {
		t.Errorf("应为 Literal a，实际 %v", n)
	}
}

func TestParseWildcard(t *testing.T) {
	n, err := Parse(".")
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != ast.KindWildcard {
		t.Errorf("应为 Wildcard，实际 %v", n)
	}
}

func TestParseStar(t *testing.T) {
	n, err := Parse("a*")
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != ast.KindStar {
		t.Errorf("应为 Star，实际 %v", n)
	}
	if n.Children[0].Kind != ast.KindLiteral {
		t.Error("Star 的子节点应为 Literal")
	}
}

func TestParsePlus(t *testing.T) {
	n, err := Parse("a+")
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != ast.KindPlus {
		t.Errorf("应为 Plus，实际 %v", n)
	}
}

func TestParseQuestion(t *testing.T) {
	n, err := Parse("a?")
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != ast.KindQuestion {
		t.Errorf("应为 Question，实际 %v", n)
	}
}

func TestParseConcat(t *testing.T) {
	n, err := Parse("ab")
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != ast.KindConcat {
		t.Errorf("应为 Concat，实际 %v", n)
	}
}

func TestParseAlternate(t *testing.T) {
	n, err := Parse("a|b")
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != ast.KindAlternate {
		t.Errorf("应为 Alternate，实际 %v", n)
	}
}

func TestParseGroup(t *testing.T) {
	n, err := Parse("(ab)")
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != ast.KindGroup {
		t.Errorf("应为 Group，实际 %v", n)
	}
}

func TestParseCharClass(t *testing.T) {
	n, err := Parse("[abc]")
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != ast.KindCharClass {
		t.Errorf("应为 CharClass，实际 %v", n)
	}
}

func TestParseCharClassRange(t *testing.T) {
	n, err := Parse("[a-z]")
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != ast.KindCharClass {
		t.Errorf("应为 CharClass，实际 %v", n)
	}
	if len(n.Chars) != 26 {
		t.Errorf("a-z 应展开为 26 字符，实际 %d", len(n.Chars))
	}
}

func TestParseNegatedClass(t *testing.T) {
	n, err := Parse("[^abc]")
	if err != nil {
		t.Fatal(err)
	}
	if !n.Negated {
		t.Error("[^abc] 应 Negated=true")
	}
}

func TestParseEscape(t *testing.T) {
	n, err := Parse(`\.`)
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != ast.KindLiteral || n.Char != '.' {
		t.Errorf(`\. 应为 Literal .，实际 %v`, n)
	}
}

func TestParseDigitClass(t *testing.T) {
	n, err := Parse(`\d`)
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != ast.KindCharClass || len(n.Chars) != 10 {
		t.Errorf(`\d 应为 CharClass 10 字符，实际 %v`, n)
	}
}

func TestParseEmptyError(t *testing.T) {
	_, err := Parse("")
	if err == nil {
		t.Error("空正则应报错")
	}
}

func TestParseQuantifierAtStartError(t *testing.T) {
	_, err := Parse("*")
	if err == nil {
		t.Error("* 开头应报错")
	}
	_, err = Parse("+")
	if err == nil {
		t.Error("+ 开头应报错")
	}
}

func TestParseUnclosedParenError(t *testing.T) {
	_, err := Parse("(abc")
	if err == nil {
		t.Error("未闭合括号应报错")
	}
}

func TestParseUnclosedCharClassError(t *testing.T) {
	_, err := Parse("[abc")
	if err == nil {
		t.Error("未闭合字符类应报错")
	}
}

func TestParseAnchorNotSupported(t *testing.T) {
	// 锚点 M2 才支持，当前应报错
	_, err := Parse("^abc")
	if err == nil {
		t.Error("^ 应报错（M2 不支持）")
	}
	_, err = Parse("abc$")
	if err == nil {
		t.Error("$ 应报错（M2 不支持）")
	}
}

func TestParseComplex(t *testing.T) {
	// 复杂正则：\w+@\w+\.\w+ 应能解析不报错
	_, err := Parse(`\w+@\w+\.\w+`)
	if err != nil {
		t.Errorf("email 正则应能解析: %v", err)
	}
}

func TestParseLeftoverError(t *testing.T) {
	// 解析完后有多余字符应报错
	_, err := Parse("a)b")
	if err == nil {
		t.Error("多余 ) 应报错")
	}
}

func TestParseLazyStar(t *testing.T) {
	n, err := Parse("a*?")
	if err != nil {
		t.Fatalf("a*? 应能解析: %v", err)
	}
	if n.Kind != ast.KindStar {
		t.Fatalf("a*? 根应为 Star，实际 %v", n.Kind)
	}
	if !n.Lazy {
		t.Error("a*? 应标记 Lazy=true")
	}
}

func TestParseLazyPlus(t *testing.T) {
	n, err := Parse("a+?")
	if err != nil {
		t.Fatalf("a+? 应能解析: %v", err)
	}
	if n.Kind != ast.KindPlus {
		t.Fatalf("a+? 根应为 Plus，实际 %v", n.Kind)
	}
	if !n.Lazy {
		t.Error("a+? 应标记 Lazy=true")
	}
}

func TestParseLazyQuestion(t *testing.T) {
	n, err := Parse("a??")
	if err != nil {
		t.Fatalf("a?? 应能解析: %v", err)
	}
	if n.Kind != ast.KindQuestion {
		t.Fatalf("a?? 根应为 Question，实际 %v", n.Kind)
	}
	if !n.Lazy {
		t.Error("a?? 应标记 Lazy=true")
	}
}

func TestParseGreedyNotLazy(t *testing.T) {
	// 普通 a* 不应被标记 Lazy。
	n, err := Parse("a*")
	if err != nil {
		t.Fatal(err)
	}
	if n.Lazy {
		t.Error("a* 不应标记 Lazy")
	}
}

func TestParseLazyInsideConcat(t *testing.T) {
	// a.*?b：.*? 应在 Concat 内被正确标记为 Lazy Star。
	n, err := Parse("a.*?b")
	if err != nil {
		t.Fatalf("a.*?b 应能解析: %v", err)
	}
	if n.Kind != ast.KindConcat {
		t.Fatalf("a.*?b 根应为 Concat，实际 %v", n.Kind)
	}
	// 找到 Star 节点验证 Lazy。
	var star *ast.Node
	var walk func(*ast.Node)
	walk = func(node *ast.Node) {
		if star != nil || node == nil {
			return
		}
		if node.Kind == ast.KindStar {
			star = node
			return
		}
		for _, c := range node.Children {
			walk(c)
		}
	}
	walk(n)
	if star == nil {
		t.Fatal("a.*?b 内应含 Star 节点")
	}
	if !star.Lazy {
		t.Error("a.*?b 中的 .* 应标记 Lazy=true")
	}
}

func TestParseCaseInsensitivePrefix(t *testing.T) {
	// (?i) 应被识别为标志并跳过，不解析成普通字符。
	// "abc" 部分：每个 ASCII 字母字面量在 (?i) 下应展开为含大小写两版本的字符类。
	n, err := Parse("(?i)abc")
	if err != nil {
		t.Fatalf("(?i)abc 应能解析: %v", err)
	}
	if n.Kind != ast.KindConcat {
		t.Fatalf("应为 Concat，实际 %v", n.Kind)
	}
	// Concat 左结合：(ab)c → 先 a·b 再 ·c，叶子都在左侧 Children[0] 上。
	// 找到第一个叶子字符验证它被展开成大小写字符类。
	leaf := n
	for leaf.Kind == ast.KindConcat {
		leaf = leaf.Children[0]
	}
	if leaf.Kind != ast.KindCharClass {
		t.Errorf("(?i) 下 'a' 应展开为 CharClass，实际 %v", leaf.Kind)
	}
	if !containsRune(leaf.Chars, 'a') || !containsRune(leaf.Chars, 'A') {
		t.Errorf("(?i) 下 'a' 应展开为含 a 与 A，实际 %v", leaf.Chars)
	}
}

func TestParseCaseInsensitiveCharClass(t *testing.T) {
	// (?i)[abc] 应把每个字母补上大小写对侧：得到 a A b B c C（共 6 个）。
	n, err := Parse("(?i)[abc]")
	if err != nil {
		t.Fatalf("(?i)[abc] 应能解析: %v", err)
	}
	if n.Kind != ast.KindCharClass {
		t.Fatalf("应为 CharClass，实际 %v", n.Kind)
	}
	for _, want := range []rune{'a', 'A', 'b', 'B', 'c', 'C'} {
		if !containsRune(n.Chars, want) {
			t.Errorf("(?i)[abc] 应包含 %q，实际 %v", string(want), n.Chars)
		}
	}
}

func TestParseCaseInsensitiveOnlyFlag(t *testing.T) {
	// 只有 (?i) 没有表达式：应报错（期望子表达式）。
	_, err := Parse("(?i)")
	if err == nil {
		t.Error("(?i) 后无表达式应报错")
	}
}

// containsRune 报告 chars 是否含字符 r（测试辅助）。
func containsRune(chars []rune, r rune) bool {
	for _, c := range chars {
		if c == r {
			return true
		}
	}
	return false
}
