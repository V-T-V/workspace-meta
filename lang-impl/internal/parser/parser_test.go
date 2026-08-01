package parser

import (
	"strings"
	"testing"

	"github.com/QiuShichang/lang-impl/internal/core"
	"github.com/QiuShichang/lang-impl/internal/lexer"
)

// tokenize 是测试辅助：把源码切成 token，失败即 t.Fatal。
func tokenize(t *testing.T, src string) []core.Token {
	t.Helper()
	toks, err := lexer.Tokenize(src)
	if err != nil {
		t.Fatalf("lexer 失败: %v", err)
	}
	return toks
}

// mustParse 把源码解析成程序，失败即 t.Fatal。
func mustParse(t *testing.T, src string) *core.Program {
	t.Helper()
	prog, err := Parse(tokenize(t, src))
	if err != nil {
		t.Fatalf("parse 失败: %v", err)
	}
	return prog
}

// ===== 字面量与主表达式 =====

func TestParseNumber(t *testing.T) {
	prog := mustParse(t, `42;`)
	es, ok := prog.Stmts[0].(*core.ExprStmt)
	if !ok {
		t.Fatalf("应为 ExprStmt，实际 %T", prog.Stmts[0])
	}
	n, ok := es.Expr.(*core.NumberExpr)
	if !ok {
		t.Fatalf("应为 NumberExpr，实际 %T", es.Expr)
	}
	if n.Value != 42 {
		t.Errorf("值应为 42，实际 %d", n.Value)
	}
}

func TestParseString(t *testing.T) {
	prog := mustParse(t, `"hello";`)
	es := prog.Stmts[0].(*core.ExprStmt)
	s := es.Expr.(*core.StringExpr)
	if s.Value != "hello" {
		t.Errorf("值应为 hello，实际 %q", s.Value)
	}
}

func TestParseBool(t *testing.T) {
	for _, c := range []struct {
		src string
		v   bool
	}{
		{"true;", true},
		{"false;", false},
	} {
		prog := mustParse(t, c.src)
		b := prog.Stmts[0].(*core.ExprStmt).Expr.(*core.BoolExpr)
		if b.Value != c.v {
			t.Errorf("%s: 值应为 %v，实际 %v", c.src, c.v, b.Value)
		}
	}
}

func TestParseIdent(t *testing.T) {
	prog := mustParse(t, `x;`)
	id := prog.Stmts[0].(*core.ExprStmt).Expr.(*core.IdentExpr)
	if id.Name != "x" {
		t.Errorf("名字应为 x，实际 %q", id.Name)
	}
}

// ===== 语句 =====

func TestParseLet(t *testing.T) {
	prog := mustParse(t, `let x = 10;`)
	let := prog.Stmts[0].(*core.LetStmt)
	if let.Name != "x" {
		t.Errorf("名字应为 x，实际 %q", let.Name)
	}
	if _, ok := let.Init.(*core.NumberExpr); !ok {
		t.Errorf("Init 应为 NumberExpr，实际 %T", let.Init)
	}
}

func TestParseFn(t *testing.T) {
	prog := mustParse(t, `fn add(a, b) { return a + b; }`)
	fn := prog.Stmts[0].(*core.FnDecl)
	if fn.Name != "add" {
		t.Errorf("名字应为 add，实际 %q", fn.Name)
	}
	if len(fn.Params) != 2 || fn.Params[0] != "a" || fn.Params[1] != "b" {
		t.Errorf("参数应为 [a,b]，实际 %v", fn.Params)
	}
	if fn.Body == nil || len(fn.Body.Stmts) != 1 {
		t.Errorf("函数体应有 1 条语句")
	}
}

func TestParseFnNoParams(t *testing.T) {
	prog := mustParse(t, `fn zero() { return 0; }`)
	fn := prog.Stmts[0].(*core.FnDecl)
	if len(fn.Params) != 0 {
		t.Errorf("无参函数参数应为空，实际 %v", fn.Params)
	}
}

func TestParseIf(t *testing.T) {
	prog := mustParse(t, `if (x < 2) { return x; }`)
	ifs := prog.Stmts[0].(*core.IfStmt)
	if ifs.Else != nil {
		t.Error("无 else 时 Else 应为 nil")
	}
	if _, ok := ifs.Cond.(*core.BinaryExpr); !ok {
		t.Errorf("Cond 应为 BinaryExpr，实际 %T", ifs.Cond)
	}
}

func TestParseIfElse(t *testing.T) {
	prog := mustParse(t, `if (x) { 1; } else { 2; }`)
	ifs := prog.Stmts[0].(*core.IfStmt)
	if ifs.Else == nil {
		t.Fatal("应有 else 块")
	}
}

func TestParseIfElseIf(t *testing.T) {
	prog := mustParse(t, `if (x) { 1; } else if (y) { 2; } else { 3; }`)
	ifs := prog.Stmts[0].(*core.IfStmt)
	if ifs.Else == nil {
		t.Fatal("else-if 应产生 else 块")
	}
	if len(ifs.Else.Stmts) != 1 {
		t.Fatalf("else 块应含 1 条嵌套 if，实际 %d", len(ifs.Else.Stmts))
	}
	if _, ok := ifs.Else.Stmts[0].(*core.IfStmt); !ok {
		t.Errorf("else 块内应为嵌套 IfStmt，实际 %T", ifs.Else.Stmts[0])
	}
}

func TestParseWhile(t *testing.T) {
	// M 的 = 是 let 专用，不是赋值表达式运算符；body 用调用语句代替。
	prog := mustParse(t, `while (i < 10) { step(); }`)
	w := prog.Stmts[0].(*core.WhileStmt)
	if _, ok := w.Cond.(*core.BinaryExpr); !ok {
		t.Errorf("Cond 应为 BinaryExpr，实际 %T", w.Cond)
	}
	if w.Body == nil || len(w.Body.Stmts) != 1 {
		t.Errorf("while body 应有 1 条语句")
	}
}

func TestParseReturn(t *testing.T) {
	prog := mustParse(t, `return 5;`)
	ret := prog.Stmts[0].(*core.ReturnStmt)
	if ret.Value == nil {
		t.Error("return 5; 的 Value 不应为 nil")
	}
}

func TestParseReturnBare(t *testing.T) {
	prog := mustParse(t, `return;`)
	ret := prog.Stmts[0].(*core.ReturnStmt)
	if ret.Value != nil {
		t.Error("裸 return; 的 Value 应为 nil")
	}
}

func TestParseBlock(t *testing.T) {
	prog := mustParse(t, `{ 1; 2; 3; }`)
	blk := prog.Stmts[0].(*core.BlockStmt)
	if len(blk.Stmts) != 3 {
		t.Errorf("块内应有 3 条语句，实际 %d", len(blk.Stmts))
	}
}

func TestParseExprStmt(t *testing.T) {
	prog := mustParse(t, `1 + 2;`)
	if _, ok := prog.Stmts[0].(*core.ExprStmt); !ok {
		t.Errorf("应为 ExprStmt，实际 %T", prog.Stmts[0])
	}
}

// ===== 运算符优先级 =====

// binary 构造一条二元表达式链（左结合），用于断言优先级树形。
func TestPriorityMulOverAdd(t *testing.T) {
	// 1 + 2 * 3  →  (1 + (2 * 3))
	prog := mustParse(t, `1 + 2 * 3;`)
	top := prog.Stmts[0].(*core.ExprStmt).Expr.(*core.BinaryExpr)
	if top.Op != core.TokPlus {
		t.Errorf("顶层应为 +，实际 %s", core.TokenName(top.Op))
	}
	if _, ok := top.Right.(*core.BinaryExpr); !ok {
		t.Errorf("右侧应为内层 *，实际 %T", top.Right)
	} else if top.Right.(*core.BinaryExpr).Op != core.TokStar {
		t.Errorf("内层应为 *，实际 %s", core.TokenName(top.Right.(*core.BinaryExpr).Op))
	}
}

func TestPriorityLeftAssoc(t *testing.T) {
	// 1 - 2 - 3  →  ((1 - 2) - 3)（左结合）
	prog := mustParse(t, `1 - 2 - 3;`)
	top := prog.Stmts[0].(*core.ExprStmt).Expr.(*core.BinaryExpr)
	if top.Op != core.TokMinus {
		t.Fatalf("顶层应为 -，实际 %s", core.TokenName(top.Op))
	}
	if _, ok := top.Left.(*core.BinaryExpr); !ok {
		t.Errorf("左侧应为内层 -（左结合），实际 %T", top.Left)
	}
	if n, ok := top.Right.(*core.NumberExpr); !ok || n.Value != 3 {
		t.Errorf("右侧应为 Number 3")
	}
}

func TestPriorityParens(t *testing.T) {
	// (1 + 2) * 3  →  括号分组改变优先级
	prog := mustParse(t, `(1 + 2) * 3;`)
	top := prog.Stmts[0].(*core.ExprStmt).Expr.(*core.BinaryExpr)
	if top.Op != core.TokStar {
		t.Errorf("顶层应为 *，实际 %s", core.TokenName(top.Op))
	}
	if _, ok := top.Left.(*core.BinaryExpr); !ok {
		t.Errorf("左侧应为括号里的 +，实际 %T", top.Left)
	}
}

func TestPriorityUnary(t *testing.T) {
	prog := mustParse(t, `-x;`)
	u := prog.Stmts[0].(*core.ExprStmt).Expr.(*core.UnaryExpr)
	if u.Op != core.TokMinus {
		t.Errorf("应为 -，实际 %s", core.TokenName(u.Op))
	}
}

func TestPriorityNotOverAnd(t *testing.T) {
	// !a && b → (!a) && b
	prog := mustParse(t, `!a && b;`)
	top := prog.Stmts[0].(*core.ExprStmt).Expr.(*core.BinaryExpr)
	if top.Op != core.TokAnd {
		t.Errorf("顶层应为 &&，实际 %s", core.TokenName(top.Op))
	}
	if _, ok := top.Left.(*core.UnaryExpr); !ok {
		t.Errorf("左侧应为 !a，实际 %T", top.Left)
	}
}

func TestPriorityFullChain(t *testing.T) {
	// a || b && c == d > e + f * g
	// 结合：a || (b && (c == (d > (e + (f * g)))))
	prog := mustParse(t, `a || b && c == d > e + f * g;`)
	top := prog.Stmts[0].(*core.ExprStmt).Expr.(*core.BinaryExpr)
	if top.Op != core.TokOr {
		t.Fatalf("顶层应为 ||，实际 %s", core.TokenName(top.Op))
	}
	if top.Right.(*core.BinaryExpr).Op != core.TokAnd {
		t.Errorf("第二层应为 &&")
	}
}

func TestCallExpr(t *testing.T) {
	prog := mustParse(t, `fib(10);`)
	call := prog.Stmts[0].(*core.ExprStmt).Expr.(*core.CallExpr)
	if call.Callee != "fib" {
		t.Errorf("Callee 应为 fib，实际 %q", call.Callee)
	}
	if len(call.Args) != 1 {
		t.Fatalf("应有 1 个参数，实际 %d", len(call.Args))
	}
}

func TestCallExprMultiArgs(t *testing.T) {
	prog := mustParse(t, `add(1, 2, 3);`)
	call := prog.Stmts[0].(*core.ExprStmt).Expr.(*core.CallExpr)
	if len(call.Args) != 3 {
		t.Errorf("应有 3 个参数，实际 %d", len(call.Args))
	}
}

func TestCallExprNoArgs(t *testing.T) {
	prog := mustParse(t, `zero();`)
	call := prog.Stmts[0].(*core.ExprStmt).Expr.(*core.CallExpr)
	if len(call.Args) != 0 {
		t.Errorf("无参调用应 0 参数，实际 %d", len(call.Args))
	}
}

// ===== 完整程序 =====

func TestParseFibProgram(t *testing.T) {
	src := `fn fib(n) {
  if (n < 2) { return n; }
  return fib(n - 1) + fib(n - 2);
}
let r = fib(10);`
	prog := mustParse(t, src)
	if len(prog.Stmts) != 2 {
		t.Fatalf("顶层应有 2 条语句，实际 %d", len(prog.Stmts))
	}
	if _, ok := prog.Stmts[0].(*core.FnDecl); !ok {
		t.Errorf("第一条应为 FnDecl，实际 %T", prog.Stmts[0])
	}
	if _, ok := prog.Stmts[1].(*core.LetStmt); !ok {
		t.Errorf("第二条应为 LetStmt，实际 %T", prog.Stmts[1])
	}
}

func TestParseEmptyProgram(t *testing.T) {
	prog := mustParse(t, ``)
	if len(prog.Stmts) != 0 {
		t.Errorf("空程序应 0 条语句，实际 %d", len(prog.Stmts))
	}
}

func TestParseComments(t *testing.T) {
	prog := mustParse(t, `// 注释
1; // 行尾注释`)
	if len(prog.Stmts) != 1 {
		t.Errorf("注释后应只 1 条语句，实际 %d", len(prog.Stmts))
	}
}

// ===== 错误场景 =====

func TestErrMissingSemicolon(t *testing.T) {
	_, err := Parse(tokenize(t, `let x = 10`))
	if err == nil {
		t.Error("缺分号应报错")
	}
}

func TestErrMissingRParen(t *testing.T) {
	_, err := Parse(tokenize(t, `(1 + 2;`))
	if err == nil {
		t.Error("缺右括号应报错")
	}
}

func TestErrMissingRBrace(t *testing.T) {
	_, err := Parse(tokenize(t, `fn f() { 1; `))
	if err == nil {
		t.Error("缺右大括号应报错")
	}
}

func TestErrLetNoIdent(t *testing.T) {
	_, err := Parse(tokenize(t, `let = 10;`))
	if err == nil {
		t.Error("let 后缺 ident 应报错")
	}
}

func TestErrFnNoBody(t *testing.T) {
	_, err := Parse(tokenize(t, `fn f() `))
	if err == nil {
		t.Error("函数缺函数体应报错")
	}
}

func TestErrMissingRParenInCall(t *testing.T) {
	_, err := Parse(tokenize(t, `f(1, 2;`))
	if err == nil {
		t.Error("调用缺右括号应报错")
	}
}

func TestErrUnexpectedToken(t *testing.T) {
	_, err := Parse(tokenize(t, `);`))
	if err == nil {
		t.Error("意外 token 应报错")
	}
}

func TestErrMissingCommaInArgs(t *testing.T) {
	// f(1 2) —— 第二个参数应是表达式起点，1 后期望 ) 或 ,
	_, err := Parse(tokenize(t, `f(1 2);`))
	if err == nil {
		t.Error("参数间缺逗号应报错")
	}
}

// ===== 错误带 SourceLoc =====

func TestErrorHasSourceLoc(t *testing.T) {
	// 第二行缺分号
	_, err := Parse(tokenize(t, "1;\nlet x = 5"))
	if err == nil {
		t.Fatal("应报错")
	}
	cerr, ok := err.(*core.Error)
	if !ok {
		t.Fatalf("错误应为 *core.Error，实际 %T", err)
	}
	if cerr.Loc.Line != 2 {
		t.Errorf("错误应在第 2 行，实际 %d", cerr.Loc.Line)
	}
}

// ===== AST Printer =====

func TestPrintProgram(t *testing.T) {
	prog := mustParse(t, `let x = 1 + 2;`)
	out := Print(prog)
	if !strings.Contains(out, "Let") {
		t.Errorf("Print 输出应含 Let，实际:\n%s", out)
	}
	if !strings.Contains(out, "Binary +") {
		t.Errorf("Print 输出应含 Binary +，实际:\n%s", out)
	}
}

func TestPrintNumber(t *testing.T) {
	prog := mustParse(t, `42;`)
	out := Print(prog.Stmts[0])
	if !strings.Contains(out, "Number 42") {
		t.Errorf("应含 Number 42，实际:\n%s", out)
	}
}

func TestPrintCallArgs(t *testing.T) {
	prog := mustParse(t, `fib(10);`)
	out := Print(prog.Stmts[0])
	if !strings.Contains(out, `Call "fib"`) {
		t.Errorf("应含 Call fib，实际:\n%s", out)
	}
	if !strings.Contains(out, "Number 10") {
		t.Errorf("参数 Number 10 应出现，实际:\n%s", out)
	}
}
