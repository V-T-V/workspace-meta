package interpreter

import (
	"context"
	"fmt"

	"github.com/QiuShichang/lang-impl/internal/lexer"
	"github.com/QiuShichang/lang-impl/internal/parser"
)

// DemoResult 是 interpreter demo 的输出摘要。
type DemoResult struct {
	Fib10       int64  // fib(10) 的结果（应为 55）
	Fib10String string // 格式化的 fib(10) 结果
	ExprValues  []string
}

// Demo 演示解释执行：跑 fib(10)=55 + 几个表达式，打印结果。
// 确定性：固定源码，无 goroutine/time/rand。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	src := `// M 语言示例：斐波那契
fn fib(n) {
  if (n < 2) { return n; }
  return fib(n - 1) + fib(n - 2);
}
let r = fib(10);
r;`

	fmt.Println("=== 解释执行 demo ===")
	fmt.Println("源码:")
	fmt.Println(src)
	fmt.Println()

	tokens, err := lexer.Tokenize(src)
	if err != nil {
		return nil, err
	}
	prog, err := parser.Parse(tokens)
	if err != nil {
		return nil, err
	}
	result, err := Run(prog)
	if err != nil {
		return nil, err
	}

	fib10, _ := result.(int64)
	fmt.Printf("fib(10) = %s\n", FormatValue(result))

	// 再跑几个独立表达式
	fmt.Println("\n几个独立表达式:")
	exprs := []string{
		"1 + 2 * 3;",
		"(1 + 2) * 3;",
		"10 / 3;",
		"10 % 3;",
		`"a" + "b";`, // 字符串拼接（动态类型双语义：+ 支持 int 相加和 string 拼接）
		"3 > 2 && 1 < 5;",
		"!false;",
	}
	var vals []string
	for _, e := range exprs {
		toks, _ := lexer.Tokenize(e)
		p, _ := parser.Parse(toks)
		v, runErr := Run(p)
		var s string
		if runErr != nil {
			s = "错误: " + runErr.Error()
		} else {
			s = FormatValue(v)
		}
		vals = append(vals, s)
		fmt.Printf("  %s => %s\n", trimSemicolon(e), s)
	}

	return &DemoResult{
		Fib10:       fib10,
		Fib10String: FormatValue(result),
		ExprValues:  vals,
	}, nil
}

// trimSemicolon 去掉末尾分号（打印用）。
func trimSemicolon(s string) string {
	for len(s) > 0 && (s[len(s)-1] == ';') {
		s = s[:len(s)-1]
	}
	return s
}
