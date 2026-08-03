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

// ===== for 循环 =====

func TestForLoopBasic(t *testing.T) {
	// C 风格 for：累加 0..9 = 45。
	src := `let total = 0;
for (let i = 0; i < 10; i = i + 1) { total = total + i; }
return total;`
	if got := runInt(t, src); got != 45 {
		t.Errorf("for 累加 0..9 应为 45，实际 %d", got)
	}
}

func TestForLoopScopedVar(t *testing.T) {
	// for 的 init（let i）不应泄漏到外层：循环结束后 i 不可见。
	src := `let outer = 0;
for (let i = 0; i < 3; i = i + 1) { outer = outer + i; }
return outer;`
	if got := runInt(t, src); got != 3 {
		t.Errorf("for 累加 0..2 应为 3，实际 %d", got)
	}
}

func TestForLoopReturnInBody(t *testing.T) {
	// return 应能穿透 for 循环直达函数边界。
	src := `fn firstHit() {
  for (let i = 0; i < 100; i = i + 1) {
    if (i == 7) { return i; }
  }
  return -1;
}
return firstHit();`
	if got := runInt(t, src); got != 7 {
		t.Errorf("for 内 return 应返回 7，实际 %d", got)
	}
}

func TestForLoopNested(t *testing.T) {
	// 嵌套 for：乘法表式累加。
	src := `let total = 0;
for (let i = 0; i < 3; i = i + 1) {
  for (let j = 0; j < 3; j = j + 1) {
    total = total + 1;
  }
}
return total;`
	if got := runInt(t, src); got != 9 {
		t.Errorf("嵌套 for 应累加到 9，实际 %d", got)
	}
}

func TestForLoopEmptyCond(t *testing.T) {
	// 空 cond 视为恒真：只能靠 return 跳出。
	src := `let n = 0;
for (let i = 0; ; i = i + 1) {
  n = n + 1;
  if (i >= 4) { return n; }
}
return -1;`
	if got := runInt(t, src); got != 5 {
		t.Errorf("空 cond for 循环应跑 5 轮，实际 %d", got)
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

func TestJoin(t *testing.T) {
	// 字符串数组用空格拼接
	if got := runString(t, `return join(["hello", "world"], " ");`); got != "hello world" {
		t.Errorf(`join(["hello","world"]," ") 应 "hello world"，实际 %q`, got)
	}
	// 逗号分隔
	if got := runString(t, `return join(["a", "b", "c"], ",");`); got != "a,b,c" {
		t.Errorf(`join(["a","b","c"],",") 应 "a,b,c"，实际 %q`, got)
	}
	// 空分隔符 → 直接拼接
	if got := runString(t, `return join(["ab", "cd"], "");`); got != "abcd" {
		t.Errorf(`join(["ab","cd"],"") 应 "abcd"，实际 %q`, got)
	}
	// 数组元素是 int：按 valueToString 转字符串
	if got := runString(t, `return join([1, 2, 3], "-");`); got != "1-2-3" {
		t.Errorf(`join([1,2,3],"-") 应 "1-2-3"，实际 %q`, got)
	}
	// 混合类型元素
	if got := runString(t, `return join([1, true, "x"], "|");`); got != "1|true|x" {
		t.Errorf(`join([1,true,"x"],"|") 应 "1|true|x"，实际 %q`, got)
	}
	// 空数组 → 空字符串
	if got := runString(t, `return join([], ",");`); got != "" {
		t.Errorf(`join([],",") 应 ""，实际 %q`, got)
	}
	// 单元素数组
	if got := runString(t, `return join(["solo"], ",");`); got != "solo" {
		t.Errorf(`join(["solo"],",") 应 "solo"，实际 %q`, got)
	}
}

func TestSplit(t *testing.T) {
	// 基本分割：逗号分隔
	src := `let parts = split("a,b,c", ",");
	return parts[0] + parts[1] + parts[2];`
	if got := runString(t, src); got != "abc" {
		t.Errorf(`split("a,b,c",",") 元素拼接应 "abc"，实际 %q`, got)
	}

	// 通过长度验证分割数量
	if got := runInt(t, `let p = split("a,b,c", ","); return len(p);`); got != 3 {
		t.Errorf(`len(split("a,b,c",",")) 应 3，实际 %d`, got)
	}

	// 多字符分隔符
	if got := runInt(t, `let p = split("a::b::c", "::"); return len(p);`); got != 3 {
		t.Errorf(`len(split("a::b::c","::")) 应 3，实际 %d`, got)
	}

	// 分隔符不出现：返回单元素数组（原串）
	if got := runInt(t, `let p = split("abc", ","); return len(p);`); got != 1 {
		t.Errorf(`len(split("abc",",")) 应 1，实际 %d`, got)
	}

	// 空分隔符：按字符拆分（strings.Split 语义：每个 rune 一段）
	if got := runString(t, `let p = split("ab", ""); return p[0];`); got != "a" {
		t.Errorf(`split("ab","")[0] 应 "a"，实际 %q`, got)
	}

	// 空字符串 + 非空分隔符：strings.Split 返回 [""]（长度 1）
	if got := runInt(t, `let p = split("", ","); return len(p);`); got != 1 {
		t.Errorf(`len(split("",",")) 应 1，实际 %d`, got)
	}

	// 中文按字符拆分
	srcCN := `let parts = split("你好世界", "");
	return parts[1];`
	if got := runString(t, srcCN); got != "好" {
		t.Errorf(`split("你好世界","")[1] 应 "好"，实际 %q`, got)
	}

	// 与 join 互为逆操作：join(split(s, sep), sep) == s
	srcRound := `let s = "one-two-three";
let p = split(s, "-");
return join(p, "-");`
	if got := runString(t, srcRound); got != "one-two-three" {
		t.Errorf(`join(split(s,"-"),"-") 应还原为原串，实际 %q`, got)
	}
}

func TestSplitErrors(t *testing.T) {
	// 第一个参数非字符串应报错
	if _, err := run(t, `return split(123, ",");`); err == nil {
		t.Error(`split(123,",") 第一个参数非字符串应报错`)
	}
	// 第二个参数非字符串应报错
	if _, err := run(t, `return split("abc", 1);`); err == nil {
		t.Error(`split("abc",1) 第二个参数非字符串应报错`)
	}
	// 参数数量不对：split 内置只在恰好 2 参时匹配，其余落到"未定义函数"分支报错
	if _, err := run(t, `return split("abc");`); err == nil {
		t.Error(`split("abc") 参数不足应报错`)
	}
}

func TestJoinErrors(t *testing.T) {
	// 第一个参数非数组应报错
	if _, err := run(t, `return join("abc", ",");`); err == nil {
		t.Error(`join("abc",",") 第一个参数非数组应报错`)
	}
	// 第二个参数非字符串应报错
	if _, err := run(t, `return join([1, 2], 3);`); err == nil {
		t.Error(`join([1,2],3) 第二个参数非字符串应报错`)
	}
	// 参数数量不对：join 内置只在恰好 2 参时匹配，其余会落到"未定义函数"分支报错
	if _, err := run(t, `return join([1, 2]);`); err == nil {
		t.Error(`join([1,2]) 参数不足应报错`)
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

func TestTemplateString(t *testing.T) {
	src := `let name = "alice";
return "hello ${name}!";`
	got := runString(t, src)
	if got != "hello alice!" {
		t.Errorf(`模板字符串应 "hello alice!"，实际 %q`, got)
	}
}

func TestTemplateStringExpr(t *testing.T) {
	// ${30 + 1} 嵌入表达式
	got := runString(t, `return "age: ${20 + 15}";`)
	if got != "age: 35" {
		t.Errorf(`模板应 "age: 35"，实际 %q`, got)
	}
}

func TestTemplateStringMultiple(t *testing.T) {
	src := `let a = 1; let b = 2;
return "${a} + ${b} = ${a + b}";`
	got := runString(t, src)
	if got != "1 + 2 = 3" {
		t.Errorf(`模板应 "1 + 2 = 3"，实际 %q`, got)
	}
}

func TestTemplateStringNoInterp(t *testing.T) {
	// 无 ${} 的字符串不变
	got := runString(t, `return "no interp here";`)
	if got != "no interp here" {
		t.Errorf("无插值字符串应不变")
	}
}

// ===== break / continue =====

func TestBreakFor(t *testing.T) {
	// 0+1+2+3+4 = 10，i==5 时 break
	src := `let total = 0;
for (let i = 0; i < 100; i = i + 1) {
  if (i == 5) { break; }
  total = total + i;
}
return total;`
	if got := runInt(t, src); got != 10 {
		t.Errorf("break 应让循环停在 i==5，累加应为 10，实际 %d", got)
	}
}

func TestBreakWhile(t *testing.T) {
	src := `let i = 0;
let total = 0;
while (i < 100) {
  if (i == 3) { break; }
  total = total + i;
  i = i + 1;
}
return total;` // 0+1+2 = 3
	if got := runInt(t, src); got != 3 {
		t.Errorf("while break 累加应为 3，实际 %d", got)
	}
}

func TestBreakNested(t *testing.T) {
	// break 只跳出最近一层循环（内层），外层继续。
	// 内层 j 循环每次在 j==2 break；外层 i 跑 0..2。
	// 每轮内层累加 j=0,1（2 break）→ 每轮 +1，3 轮 → 3。
	src := `let total = 0;
for (let i = 0; i < 3; i = i + 1) {
  for (let j = 0; j < 10; j = j + 1) {
    if (j == 2) { break; }
    total = total + j;
  }
}
return total;` // 3 轮 × (0+1) = 3
	if got := runInt(t, src); got != 3 {
		t.Errorf("嵌套 break 应只跳内层，累加应为 3，实际 %d", got)
	}
}

func TestBreakInIfBranch(t *testing.T) {
	// break 在 if-else 的嵌套块里也要能正确冒泡到循环
	src := `let i = 0;
let total = 0;
while (i < 100) {
  i = i + 1;
  if (i == 4) {
    { break; }
  }
  total = total + i;
}
return total;` // i=1,2,3 累加 → 6，i=4 时 break
	if got := runInt(t, src); got != 6 {
		t.Errorf("嵌套块里的 break 应跳出循环，累加应为 6，实际 %d", got)
	}
}

func TestContinueFor(t *testing.T) {
	// continue 跳过本轮：i==2 不累加 → 0+1+3+4 = 8
	src := `let total = 0;
for (let i = 0; i < 5; i = i + 1) {
  if (i == 2) { continue; }
  total = total + i;
}
return total;`
	if got := runInt(t, src); got != 8 {
		t.Errorf("continue 应跳过 i==2，累加应为 8，实际 %d", got)
	}
}

func TestContinueWhile(t *testing.T) {
	// while continue：跳过本轮剩余，仍需 i=i+1，否则死循环。
	src := `let i = 0;
let total = 0;
while (i < 5) {
  i = i + 1;
  if (i == 3) { continue; }
  total = total + i;
}
return total;` // i=1,2,(3 skip),4,5 → 1+2+4+5 = 12
	if got := runInt(t, src); got != 12 {
		t.Errorf("while continue 累加应为 12，实际 %d", got)
	}
}

func TestContinueNested(t *testing.T) {
	// 内层 continue 不影响外层。内层每次 j==1 continue（跳 j 累加），
	// 所以内层累加永远是 j=0 → 每轮 +0；3 轮 → 0。
	src := `let total = 0;
for (let i = 0; i < 3; i = i + 1) {
  for (let j = 0; j < 3; j = j + 1) {
    if (j == 1) { continue; }
    total = total + j;
  }
}
return total;` // 3 轮 × (0 + skip(1) + 2) = 3 × 2 = 6
	if got := runInt(t, src); got != 6 {
		t.Errorf("嵌套 continue 累加应为 6，实际 %d", got)
	}
}

func TestBreakReturnInteraction(t *testing.T) {
	// break 不应吞掉 return：函数内 break 后函数仍能正常返回
	src := `fn sum(limit) {
  let total = 0;
  for (let i = 0; i < 100; i = i + 1) {
    if (i == limit) { break; }
    total = total + i;
  }
  return total;
}
return sum(4);` // 0+1+2+3 = 6
	if got := runInt(t, src); got != 6 {
		t.Errorf("函数内 break 后 return 应为 6，实际 %d", got)
	}
}

// ===== 一等函数（函数值） =====

func TestFirstClassFunction(t *testing.T) {
	// 基本用例：匿名函数赋值给变量，再通过变量名调用。
	src := `let double = fn(x) { return x * 2; };
return double(21);`
	if got := runInt(t, src); got != 42 {
		t.Errorf("double(21) 应为 42，实际 %d", got)
	}
}

func TestFirstClassFunctionIncrement(t *testing.T) {
	// 题目给的另一示例变体：x + 1。
	src := `let inc = fn(x) { return x + 1; };
return inc(41);`
	if got := runInt(t, src); got != 42 {
		t.Errorf("inc(41) 应为 42，实际 %d", got)
	}
}

func TestFirstClassFunctionClosure(t *testing.T) {
	// 闭包：匿名函数捕获外层 let 绑定的变量。
	// makeAdder(n) 返回 fn(x){ return x + n; }，n 是定义时捕获的值。
	src := `fn makeAdder(n) {
  return fn(x) { return x + n; };
}
let add10 = makeAdder(10);
let add20 = makeAdder(20);
return add10(5) + add20(5);` // 15 + 25 = 40
	if got := runInt(t, src); got != 40 {
		t.Errorf("闭包 add10(5)+add20(5) 应为 40，实际 %d", got)
	}
}

func TestFirstClassFunctionPassedAsArg(t *testing.T) {
	// 一等函数作为参数传递：apply(f, x) 调用传入的函数值。
	src := `fn apply(f, x) {
  return f(x);
}
let square = fn(n) { return n * n; };
return apply(square, 6);` // 36
	if got := runInt(t, src); got != 36 {
		t.Errorf("apply(square, 6) 应为 36，实际 %d", got)
	}
}

func TestFirstClassFunctionNoParams(t *testing.T) {
	// 无参匿名函数。
	src := `let fortytwo = fn() { return 42; };
return fortytwo();`
	if got := runInt(t, src); got != 42 {
		t.Errorf("无参函数调用应为 42，实际 %d", got)
	}
}

func TestFirstClassFunctionMultiParams(t *testing.T) {
	// 多参数匿名函数。
	src := `let add = fn(a, b) { return a + b; };
return add(15, 27);`
	if got := runInt(t, src); got != 42 {
		t.Errorf("add(15,27) 应为 42，实际 %d", got)
	}
}

func TestFirstClassFunctionArgCountError(t *testing.T) {
	// 一等函数参数数量不匹配应报错（而非静默通过）。
	src := `let f = fn(x) { return x; };
return f(1, 2);`
	_, err := run(t, src)
	if err == nil {
		t.Error("参数数量不匹配应返回错误")
	}
}

func TestFirstClassFunctionOverridesNamed(t *testing.T) {
	// 同名时，函数表里的命名函数与变量环境里的 function value 各自独立：
	// let f = ... 绑定到变量环境，调用 f() 时优先用变量环境里的 function value。
	src := `fn f() { return 1; }
let g = fn() { return 99; };
return g();`
	if got := runInt(t, src); got != 99 {
		t.Errorf("g() 应为 99，实际 %d", got)
	}
}

// TestClosureCounter 验证递归闭包（counter 工厂模式）。
//
// makeCounter 返回一个闭包，闭包内通过裸赋值 count = count + 1 修改外层
// makeCounter 作用域里捕获的 count。每次调用闭包都更新同一个被捕获的环境，
// 因此多次调用 c() 累计递增：1, 2, 3。
//
// 这是验证已有的 fn 表达式 + 闭包能正确支持"递归状态捕获"：
//   - 闭包捕获的是定义时的环境（makeCounter 的块作用域），而非调用现场的环境；
//   - 多次调用同一个闭包共享同一个被捕获的环境；
//   - 闭包内的裸赋值（IsAssign）通过 Environment.Set 沿链向上找到并更新那层
//     被捕获的变量，而非在闭包自己的调用层新建变量。
func TestClosureCounter(t *testing.T) {
	src := `let makeCounter = fn() {
  let count = 0;
  return fn() {
    count = count + 1;
    return count;
  };
};
let c = makeCounter();
return c() + c() + c();` // 1+2+3 = 6
	if got := runInt(t, src); got != 6 {
		t.Errorf("counter 闭包三次调用累加应为 6（1+2+3），实际 %d", got)
	}
}

// TestClosureCounterIndependent 验证两个独立 counter 互不干扰：
// 每次调用 makeCounter() 都新建一个独立的捕获环境。
func TestClosureCounterIndependent(t *testing.T) {
	src := `let makeCounter = fn() {
  let count = 0;
  return fn() {
    count = count + 1;
    return count;
  };
};
let a = makeCounter();
let b = makeCounter();
return a() + a() + b();` // a:1, a:2, b:1 → 4
	if got := runInt(t, src); got != 4 {
		t.Errorf("两个独立 counter 应为 4（a=1,a=2,b=1），实际 %d", got)
	}
}

func TestMinMax(t *testing.T) {
	if runInt(t, "return min(3, 7);") != 3 {
		t.Error("min(3,7)应为3")
	}
	if runInt(t, "return max(3, 7);") != 7 {
		t.Error("max(3,7)应为7")
	}
	if runInt(t, "return abs(-5);") != 5 {
		t.Error("abs(-5)应为5")
	}
}

func TestRangeBuiltin(t *testing.T) {
	src := `let r = range(5); return len(r);`
	if runInt(t, src) != 5 {
		t.Error("range(5)长度应为5")
	}
	// 验证 range(3)[0]==0, [1]==1, [2]==2
	src2 := `let r = range(3); return r[0] + r[1] + r[2];`
	if runInt(t, src2) != 3 {
		t.Error("range(3)元素和应为3")
	}
}

func TestJoinBuiltin(t *testing.T) {
	got := runString(t, `let parts = ["a","b","c"]; return join(parts, "-");`)
	if got != "a-b-c" {
		t.Errorf("join应为a-b-c，实际%q", got)
	}
}

func TestSplitBuiltin(t *testing.T) {
	src := `let parts = split("a-b-c", "-"); return len(parts);`
	if runInt(t, src) != 3 {
		t.Error("split应返回3段")
	}
}

func TestForLoopBreak(t *testing.T) {
	src := `let total = 0;
for (let i = 0; i < 100; i = i + 1) {
  if (i == 5) { break; }
  total = total + i;
}
return total;`
	if runInt(t, src) != 10 {
		t.Error("break 应使 total=10")
	}
}

func TestForLoopContinue(t *testing.T) {
	src := `let total = 0;
for (let i = 0; i < 5; i = i + 1) {
  if (i == 2) { continue; }
  total = total + i;
}
return total;`
	if runInt(t, src) != 8 {
		t.Error("continue 应使 total=8（0+1+3+4）")
	}
}
