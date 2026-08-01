package interpreter

import (
	"testing"

	"github.com/QiuShichang/lang-impl/internal/core"
	"github.com/QiuShichang/lang-impl/internal/lexer"
	"github.com/QiuShichang/lang-impl/internal/parser"
)

// run 把源码 lex→parse→interpret 一步跑完，返回结果。
func run(t *testing.T, src string) (any, error) {
	t.Helper()
	toks, err := lexer.Tokenize(src)
	if err != nil {
		t.Fatalf("lexer 失败: %v", err)
	}
	prog, err := parser.Parse(toks)
	if err != nil {
		t.Fatalf("parse 失败: %v", err)
	}
	return Run(prog)
}

// runInt 跑出 int64 结果（失败即 t.Fatal）。
func runInt(t *testing.T, src string) int64 {
	t.Helper()
	v, err := run(t, src)
	if err != nil {
		t.Fatalf("运行错误: %v", err)
	}
	n, ok := v.(int64)
	if !ok {
		t.Fatalf("期望 int64，实际 %T (%v)", v, v)
	}
	return n
}

// runBool / runString 同理。
func runBool(t *testing.T, src string) bool {
	t.Helper()
	v, err := run(t, src)
	if err != nil {
		t.Fatalf("运行错误: %v", err)
	}
	b, ok := v.(bool)
	if !ok {
		t.Fatalf("期望 bool，实际 %T (%v)", v, v)
	}
	return b
}

func runString(t *testing.T, src string) string {
	t.Helper()
	v, err := run(t, src)
	if err != nil {
		t.Fatalf("运行错误: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("期望 string，实际 %T (%v)", v, v)
	}
	return s
}

// ===== 算术 =====

func TestArithmeticBasic(t *testing.T) {
	if runInt(t, `1 + 2;`) != 3 {
		t.Error("1+2 应为 3")
	}
	if runInt(t, `10 - 4;`) != 6 {
		t.Error("10-4 应为 6")
	}
	if runInt(t, `3 * 4;`) != 12 {
		t.Error("3*4 应为 12")
	}
	if runInt(t, `20 / 5;`) != 4 {
		t.Error("20/5 应为 4")
	}
	if runInt(t, `17 % 5;`) != 2 {
		t.Error("17%%5 应为 2")
	}
}

func TestArithmeticPrecedence(t *testing.T) {
	if runInt(t, `1 + 2 * 3;`) != 7 {
		t.Error("1+2*3 应为 7")
	}
	if runInt(t, `(1 + 2) * 3;`) != 9 {
		t.Error("(1+2)*3 应为 9")
	}
	if runInt(t, `2 * 3 + 4 * 5;`) != 26 {
		t.Error("2*3+4*5 应为 26")
	}
}

func TestUnaryMinus(t *testing.T) {
	if runInt(t, `-5;`) != -5 {
		t.Error("-5 应为 -5")
	}
	if runInt(t, `3 - -2;`) != 5 {
		t.Error("3-(-2) 应为 5")
	}
}

// ===== 变量 =====

func TestVariableLet(t *testing.T) {
	if runInt(t, `let x = 42; x;`) != 42 {
		t.Error("let 后引用应为 42")
	}
}

func TestVariableScope(t *testing.T) {
	// 块内 let 不污染外层（块级作用域）
	src := `let x = 1;
{ let x = 99; }
x;`
	if runInt(t, src) != 1 {
		t.Error("外层 x 应仍为 1（块内 let 是局部）")
	}
}

// ===== 函数调用与递归 =====

func TestFunctionCall(t *testing.T) {
	src := `fn add(a, b) { return a + b; }
add(3, 4);`
	if runInt(t, src) != 7 {
		t.Error("add(3,4) 应为 7")
	}
}

func TestFunctionNoReturn(t *testing.T) {
	// 没有 return 的函数返回 nil
	src := `fn noop() { 1; }
noop();`
	v, err := run(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Errorf("无 return 应返回 nil，实际 %v", v)
	}
}

func TestFunctionBareReturn(t *testing.T) {
	src := `fn f() { return; }
f();`
	v, err := run(t, src)
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Errorf("裸 return 应返回 nil，实际 %v", v)
	}
}

func TestFibonacciRecursive(t *testing.T) {
	src := `fn fib(n) {
  if (n < 2) { return n; }
  return fib(n - 1) + fib(n - 2);
}
fib(10);`
	if got := runInt(t, src); got != 55 {
		t.Errorf("fib(10) 应为 55，实际 %d", got)
	}
}

func TestFibonacciZeroOne(t *testing.T) {
	src := `fn fib(n) {
  if (n < 2) { return n; }
  return fib(n - 1) + fib(n - 2);
}`
	for _, c := range []struct {
		n   int64
		out int64
	}{
		{0, 0}, {1, 1}, {2, 1}, {5, 5}, {7, 13},
	} {
		// 构造 "fib(N);" 作为最后一条表达式语句
		prog := mustProg(t, src+"\nfib("+FormatValue(c.n)+");")
		v, err := Run(prog)
		if err != nil {
			t.Fatalf("fib(%d) 运行错误: %v", c.n, err)
		}
		got, ok := v.(int64)
		if !ok {
			t.Fatalf("fib(%d) 应返回 int64，实际 %T (%v)", c.n, v, v)
		}
		if got != c.out {
			t.Errorf("fib(%d) 应为 %d，实际 %d", c.n, c.out, got)
		}
	}
}

// ===== if / while =====

func TestIfTrue(t *testing.T) {
	src := `if (1 < 2) { return 10; } else { return 20; }`
	if runInt(t, src) != 10 {
		t.Error("条件真应走 then")
	}
}

func TestIfFalse(t *testing.T) {
	src := `if (1 > 2) { return 10; } else { return 20; }`
	if runInt(t, src) != 20 {
		t.Error("条件假应走 else")
	}
}

func TestIfElseIf(t *testing.T) {
	src := `fn grade(s) {
  if (s >= 90) { return 1; }
  else if (s >= 60) { return 2; }
  else { return 3; }
}
grade(75);`
	if runInt(t, src) != 2 {
		t.Error("75 应得 2（else-if 链）")
	}
}

func TestWhileFalseSkipsBody(t *testing.T) {
	// while 条件恒假：循环体一次都不执行 → 函数走到后面的 return。
	src := `fn f() {
  while (false) { return 1; }
  return 2;
}
f();`
	if runInt(t, src) != 2 {
		t.Error("while(false) 不应执行循环体，函数应返回 2")
	}
}

func TestWhileTrueReturnsInBody(t *testing.T) {
	// while(true)：循环体立即 return（首次进入即返回）。
	// 这验证了 while 循环体能被执行、return 能跳出循环+函数。
	src := `fn f() {
  while (true) {
    return 99;
  }
  return 0; // 永远不会执行到
}
f();`
	if runInt(t, src) != 99 {
		t.Error("while(true) 体内 return 应返回 99")
	}
}

func TestWhileNestedIfReturn(t *testing.T) {
	// while(true) + 嵌套 if：第一次进入循环即触发 return。
	src := `fn pick(x) {
  while (true) {
    if (x == 1) { return 10; }
    return 20;
  }
  return 0;
}
pick(1);`
	if runInt(t, src) != 10 {
		t.Error("pick(1) 应在 while 内 return 10")
	}
}

// ===== return =====

func TestReturnInsideIf(t *testing.T) {
	src := `fn f(n) {
  if (n > 0) {
    return 100;
  }
  return -100;
}
f(5);`
	if runInt(t, src) != 100 {
		t.Error("n>0 时应 return 100")
	}
}

func TestReturnDeepNested(t *testing.T) {
	// return 应能穿透多层嵌套直达函数边界
	src := `fn f() {
  {
    {
      return 42;
    }
  }
}
f();`
	if runInt(t, src) != 42 {
		t.Error("深嵌套 return 应返回 42")
	}
}

// ===== 布尔与逻辑 =====

func TestBoolLiteral(t *testing.T) {
	if !runBool(t, `true;`) {
		t.Error("true 应为 true")
	}
	if runBool(t, `false;`) {
		t.Error("false 应为 false")
	}
}

func TestComparison(t *testing.T) {
	cases := []struct {
		src string
		v   bool
	}{
		{"3 > 2;", true},
		{"2 > 3;", false},
		{"3 >= 3;", true},
		{"2 <= 1;", false},
		{"3 == 3;", true},
		{"3 != 4;", true},
	}
	for _, c := range cases {
		if got := runBool(t, c.src); got != c.v {
			t.Errorf("%s 应为 %v，实际 %v", c.src, c.v, got)
		}
	}
}

func TestLogicalAnd(t *testing.T) {
	if !runBool(t, `true && true;`) {
		t.Error("true&&true 应为 true")
	}
	if runBool(t, `true && false;`) {
		t.Error("true&&false 应为 false")
	}
}

func TestLogicalOr(t *testing.T) {
	if !runBool(t, `false || true;`) {
		t.Error("false||true 应为 true")
	}
	if runBool(t, `false || false;`) {
		t.Error("false||false 应为 false")
	}
}

func TestLogicalShortCircuit(t *testing.T) {
	// && 短路：左假时不求值右（右是未定义变量，若求值会报错）
	src := `false && undefinedVar;`
	if runBool(t, src) {
		t.Error("短路应返回 false")
	}
	// || 短路：左真时不求值右
	src2 := `true || undefinedVar;`
	if !runBool(t, src2) {
		t.Error("短路应返回 true")
	}
}

func TestNotOperator(t *testing.T) {
	if runBool(t, `!true;`) {
		t.Error("!true 应为 false")
	}
	if !runBool(t, `!false;`) {
		t.Error("!false 应为 true")
	}
}

// ===== 字符串 =====

func TestStringLiteral(t *testing.T) {
	if runString(t, `"hello";`) != "hello" {
		t.Error("字符串应返回 hello")
	}
}

func TestStringEquality(t *testing.T) {
	if !runBool(t, `"abc" == "abc";`) {
		t.Error("相同字符串应相等")
	}
	if runBool(t, `"abc" == "abd";`) {
		t.Error("不同字符串不应相等")
	}
}

// ===== 类型检查错误 =====

func TestErrTypeMismatchPlus(t *testing.T) {
	// bool + int 是真类型错误（+ 支持 int+int 或 string+any，但不支持 bool）
	// 注意："a" + 1 不再报错（动态类型双语义，类 JS/Python，1 被转字符串拼接）
	_, err := run(t, `true + 1;`)
	if err == nil {
		t.Error(`true + 1 应类型错误（bool 不能参与 +）`)
	}
}

func TestStringConcat(t *testing.T) {
	// + 支持 string 拼接（动态类型双语义）
	if got := runString(t, `"a" + "b";`); got != "ab" {
		t.Errorf(`"a"+"b" 应 "ab"，实际 %q`, got)
	}
	// string + int → 字符串拼接（int 转字符串）
	if got := runString(t, `"n=" + 42;`); got != "n=42" {
		t.Errorf(`"n="+42 应 "n=42"，实际 %q`, got)
	}
}

func TestErrTypeMismatchComparison(t *testing.T) {
	_, err := run(t, `"a" > 1;`)
	if err == nil {
		t.Error(`"a" > 1 应类型错误`)
	}
}

func TestErrDivideByZero(t *testing.T) {
	_, err := run(t, `1 / 0;`)
	if err == nil {
		t.Error("除零应报错")
	}
}

func TestErrModuloByZero(t *testing.T) {
	_, err := run(t, `5 % 0;`)
	if err == nil {
		t.Error("模零应报错")
	}
}

func TestErrUndefinedVariable(t *testing.T) {
	_, err := run(t, `x;`)
	if err == nil {
		t.Error("未定义变量应报错")
	}
}

func TestErrUndefinedFunction(t *testing.T) {
	_, err := run(t, `ghost(1);`)
	if err == nil {
		t.Error("未定义函数应报错")
	}
}

func TestErrArgCount(t *testing.T) {
	_, err := run(t, `fn f(a) { return a; } f(1, 2);`)
	if err == nil {
		t.Error("参数数量不匹配应报错")
	}
}

func TestErrUnaryMinusOnString(t *testing.T) {
	_, err := run(t, `-"abc";`)
	if err == nil {
		t.Error(`-"abc" 应类型错误`)
	}
}

func TestErrNotOnInt(t *testing.T) {
	_, err := run(t, `!5;`)
	if err == nil {
		t.Error("!5 应类型错误")
	}
}

func TestErrCondNotBool(t *testing.T) {
	_, err := run(t, `if (5) { 1; }`)
	if err == nil {
		t.Error("if 条件非 bool 应报错")
	}
}

// ===== 错误带 SourceLoc =====

func TestErrorHasSourceLoc(t *testing.T) {
	_, err := run(t, "1;\nundefVar;")
	if err == nil {
		t.Fatal("应报错")
	}
	cerr, ok := err.(*core.Error)
	if !ok {
		t.Fatalf("应为 *core.Error，实际 %T", err)
	}
	if cerr.Loc.Line != 2 {
		t.Errorf("错误应在第 2 行，实际 %d", cerr.Loc.Line)
	}
}

// ===== 辅助 =====

// mustProg 复用：解析源码为程序（不跑解释器）。
func mustProg(t *testing.T, src string) *core.Program {
	t.Helper()
	toks, _ := lexer.Tokenize(src)
	prog, err := parser.Parse(toks)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return prog
}

func TestArrayLiteral(t *testing.T) {
	got := runInt(t, `[1, 2, 3][0];`)
	if got != 1 {
		t.Errorf("数组[0] 应为 1，实际 %d", got)
	}
}

func TestArrayIndex(t *testing.T) {
	src := `let a = [10, 20, 30]; return a[2];`
	if runInt(t, src) != 30 {
		t.Error("a[2] 应为 30")
	}
}

func TestArrayLen(t *testing.T) {
	src := `let a = [1, 2, 3, 4, 5]; return len(a);`
	if runInt(t, src) != 5 {
		t.Error("len([1,2,3,4,5]) 应为 5")
	}
}

func TestStringLen(t *testing.T) {
	if runInt(t, `len("hello");`) != 5 {
		t.Error(`len("hello") 应为 5`)
	}
}

func TestStringIndex(t *testing.T) {
	got := runString(t, `"hello"[1];`)
	if got != "e" {
		t.Errorf(`"hello"[1] 应为 "e"，实际 %q`, got)
	}
}

func TestArraySum(t *testing.T) {
	src := `fn sum(arr) {
  let total = 0;
  let i = 0;
  while (i < len(arr)) { total = total + arr[i]; i = i + 1; }
  return total;
}
return sum([1, 2, 3, 4, 5]);`
	if runInt(t, src) != 15 {
		t.Error("sum([1,2,3,4,5]) 应为 15")
	}
}

func TestArrayOutOfBounds(t *testing.T) {
	_, err := run(t, `let a = [1, 2]; return a[5];`)
	if err == nil {
		t.Error("数组越界应报错")
	}
}

func TestEmptyArray(t *testing.T) {
	if runInt(t, `len([]);`) != 0 {
		t.Error("len([]) 应为 0")
	}
}

func TestNestedArray(t *testing.T) {
	src := `let m = [[1, 2], [3, 4]]; return m[1][0];`
	if runInt(t, src) != 3 {
		t.Error("m[1][0] 应为 3")
	}
}

func TestSubstr(t *testing.T) {
	if got := runString(t, `substr("hello world", 0, 5);`); got != "hello" {
		t.Errorf(`substr("hello world",0,5) 应 "hello"，实际 %q`, got)
	}
	if got := runString(t, `substr("hello world", 6, 11);`); got != "world" {
		t.Errorf(`substr("hello world",6,11) 应 "world"，实际 %q`, got)
	}
	// 越界自动钳制
	if got := runString(t, `substr("abc", 0, 100);`); got != "abc" {
		t.Errorf(`substr("abc",0,100) 应 "abc"，实际 %q`, got)
	}
	// 中文
	if got := runString(t, `substr("你好世界", 0, 2);`); got != "你好" {
		t.Errorf(`substr("你好世界",0,2) 应 "你好"，实际 %q`, got)
	}
}

func TestCharAt(t *testing.T) {
	if got := runString(t, `charAt("hello", 1);`); got != "e" {
		t.Errorf(`charAt("hello",1) 应 "e"，实际 %q`, got)
	}
}

func TestPush(t *testing.T) {
	src := `let a = [1, 2]; let b = push(a, 3); return len(b);`
	if runInt(t, src) != 3 {
		t.Error("push 后数组长度应为 3")
	}
	// 原数组不被修改
	src2 := `let a = [1, 2]; let b = push(a, 3); return len(a);`
	if runInt(t, src2) != 2 {
		t.Error("push 不应修改原数组")
	}
}

func TestCharArrayMixed(t *testing.T) {
	// 数组 + 字符串操作组合
	src := `let parts = ["he", "llo"];
let s = parts[0] + parts[1];
return substr(s, 1, 4);`
	if got := runString(t, src); got != "ell" {
		t.Errorf("组合操作应 'ell'，实际 %q", got)
	}
}
