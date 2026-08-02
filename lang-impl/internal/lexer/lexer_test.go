package lexer

import (
	"testing"

	"github.com/QiuShichang/lang-impl/internal/core"
)

func TestSimpleArithmetic(t *testing.T) {
	tokens, err := Tokenize("1 + 2 * 3")
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := []core.TokenType{
		core.TokNumber, core.TokPlus, core.TokNumber, core.TokStar, core.TokNumber, core.TokEOF,
	}
	for i, wt := range wantTypes {
		if i >= len(tokens) {
			t.Fatalf("token %d 缺失", i)
		}
		if tokens[i].Type != wt {
			t.Errorf("token %d: 想要 %s，实际 %s", i, core.TokenName(wt), core.TokenName(tokens[i].Type))
		}
	}
}

func TestNumbers(t *testing.T) {
	tokens, _ := Tokenize("42 0 12345")
	if tokens[0].Value != "42" || tokens[1].Value != "0" || tokens[2].Value != "12345" {
		t.Errorf("数字值错: %v", tokens[:3])
	}
}

func TestKeywords(t *testing.T) {
	tokens, _ := Tokenize("let fn if else while return true false")
	want := []core.TokenType{core.TokLet, core.TokFn, core.TokIf, core.TokElse, core.TokWhile, core.TokReturn, core.TokTrue, core.TokFalse}
	for i, wt := range want {
		if tokens[i].Type != wt {
			t.Errorf("关键字 %d: 想要 %s，实际 %s", i, core.TokenName(wt), core.TokenName(tokens[i].Type))
		}
	}
}

func TestIdentifiers(t *testing.T) {
	tokens, _ := Tokenize("x foo_bar _priv")
	for _, tok := range tokens[:3] {
		if tok.Type != core.TokIdent {
			t.Errorf("应为 ident，实际 %s", core.TokenName(tok.Type))
		}
	}
	if tokens[0].Value != "x" || tokens[1].Value != "foo_bar" || tokens[2].Value != "_priv" {
		t.Errorf("ident 值错: %v", tokens[:3])
	}
}

func TestStringLiteral(t *testing.T) {
	tokens, _ := Tokenize(`"hello world"`)
	if tokens[0].Type != core.TokString || tokens[0].Value != "hello world" {
		t.Errorf("字符串错: %v", tokens[0])
	}
}

func TestStringEscapes(t *testing.T) {
	tokens, _ := Tokenize(`"a\nb\tc\"d"`)
	want := "a\nb\tc\"d"
	if tokens[0].Value != want {
		t.Errorf("转义错: 想要 %q，实际 %q", want, tokens[0].Value)
	}
}

func TestMultiCharOps(t *testing.T) {
	tokens, _ := Tokenize(">= <= == != && ||")
	want := []core.TokenType{core.TokGE, core.TokLE, core.TokEQ, core.TokNE, core.TokAnd, core.TokOr}
	for i, wt := range want {
		if tokens[i].Type != wt {
			t.Errorf("运算符 %d: 想要 %s，实际 %s", i, core.TokenName(wt), core.TokenName(tokens[i].Type))
		}
	}
}

func TestSingleCharOps(t *testing.T) {
	// 用空格分隔每个运算符，避免 ;, 和 ><= 的多字符歧义
	tokens, _ := Tokenize("( ) { } ; , + - * / % > < = !")
	want := []core.TokenType{
		core.TokLParen, core.TokRParen, core.TokLBrace, core.TokRBrace,
		core.TokSemicolon, core.TokComma,
		core.TokPlus, core.TokMinus, core.TokStar, core.TokSlash, core.TokPercent,
		core.TokGT, core.TokLT, core.TokAssign, core.TokNot,
	}
	for i, wt := range want {
		if tokens[i].Type != wt {
			t.Errorf("分隔符 %d: 想要 %s，实际 %s", i, core.TokenName(wt), core.TokenName(tokens[i].Type))
		}
	}
}

func TestLineComment(t *testing.T) {
	tokens, _ := Tokenize("1 + 2 // 这是注释\n+ 3")
	// 应得到: 1 + 2 + 3 EOF（注释和换行跳过）
	numCount := 0
	plusCount := 0
	for _, tok := range tokens {
		if tok.Type == core.TokNumber {
			numCount++
		}
		if tok.Type == core.TokPlus {
			plusCount++
		}
	}
	if numCount != 3 || plusCount != 2 {
		t.Errorf("注释跳过错: num=%d plus=%d（应 3,2）", numCount, plusCount)
	}
}

func TestSourceLoc(t *testing.T) {
	tokens, _ := Tokenize("1\n+ 2")
	if tokens[0].Loc.Line != 1 {
		t.Errorf("第一个 token 行号应为 1，实际 %d", tokens[0].Loc.Line)
	}
	// 第二个 token（+）在第 2 行
	if tokens[1].Loc.Line != 2 || tokens[1].Loc.Column != 1 {
		t.Errorf("+ 的位置应为 2:1，实际 %s", tokens[1].Loc)
	}
}

func TestUnclosedString(t *testing.T) {
	_, err := Tokenize(`"未闭合`)
	if err == nil {
		t.Error("未闭合字符串应报错")
	}
}

func TestUnknownChar(t *testing.T) {
	_, err := Tokenize("1 @ 2")
	if err == nil {
		t.Error("@ 应报错为未知字符")
	}
}

func TestEmpty(t *testing.T) {
	tokens, err := Tokenize("")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 1 || tokens[0].Type != core.TokEOF {
		t.Errorf("空源码应只有 EOF，实际 %v", tokens)
	}
}

func TestAmbersandAlone(t *testing.T) {
	_, err := Tokenize("&x")
	if err == nil {
		t.Error("单独 & 应报错")
	}
}

func TestBlockComment(t *testing.T) {
	src := `/* 块注释 */
1 + /* 行内 */ 2`
	tokens, err := Tokenize(src)
	if err != nil {
		t.Fatal(err)
	}
	// 应得到 1 + 2 EOF（注释被跳过）
	wantTypes := []core.TokenType{core.TokNumber, core.TokPlus, core.TokNumber, core.TokEOF}
	if len(tokens) < len(wantTypes) {
		t.Fatalf("token 数不足")
	}
	for i, wt := range wantTypes {
		if tokens[i].Type != wt {
			t.Errorf("token %d: 想要 %s，实际 %s", i, core.TokenName(wt), core.TokenName(tokens[i].Type))
		}
	}
}

func TestMultilineBlockComment(t *testing.T) {
	src := `/* 多行
注释 */
42`
	tokens, err := Tokenize(src)
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Type != core.TokNumber || tokens[0].Value != "42" {
		t.Error("多行块注释后应得到 42")
	}
}
