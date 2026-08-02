package parser

import (
	"context"
	"fmt"

	"github.com/QiuShichang/lang-impl/internal/core"
	"github.com/QiuShichang/lang-impl/internal/lexer"
)

// DemoResult 是 parser demo 的输出摘要。
type DemoResult struct {
	StmtCount int
	Program   *core.Program
	Tree      string // AST 树形字符串（Print 输出）
}

// Demo 演示语法分析：把一段 fib 源码解析成 AST 并打印其结构。
// 确定性：固定源码，无 goroutine/time/rand。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	src := `// M 语言示例：斐波那契
fn fib(n) {
  if (n < 2) { return n; }
  return fib(n - 1) + fib(n - 2);
}
let r = fib(10);`

	tokens, err := lexer.Tokenize(src)
	if err != nil {
		return nil, err
	}
	prog, err := Parse(tokens)
	if err != nil {
		return nil, err
	}

	tree := Print(prog)
	fmt.Println("=== 语法分析 demo ===")
	fmt.Println("源码:")
	fmt.Println(src)
	fmt.Println()
	fmt.Println("AST:")
	fmt.Print(tree)
	fmt.Printf("\n共 %d 条顶层语句\n", len(prog.Stmts))
	return &DemoResult{StmtCount: len(prog.Stmts), Program: prog, Tree: tree}, nil
}
