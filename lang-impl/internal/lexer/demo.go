package lexer

import (
	"context"
	"fmt"

	"github.com/QiuShichang/lang-impl/internal/core"
)

// DemoResult 是 lexer demo 的输出摘要。
type DemoResult struct {
	TokenCount int
	Tokens     []core.Token // 前 N 个 token（展示用）
}

// Demo 演示词法分析：把一段 M 语言源码切成 token 并打印。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	src := `// M 语言示例
fn fib(n) {
  if (n < 2) { return n; }
  return fib(n - 1) + fib(n - 2);
}
let r = fib(10);`
	tokens, err := Tokenize(src)
	if err != nil {
		return nil, err
	}
	fmt.Println("=== 词法分析 demo ===")
	fmt.Println("源码:")
	fmt.Println(src)
	fmt.Println()
	fmt.Println("Tokens:")
	for _, t := range tokens {
		fmt.Printf("  %-10s %q\n", core.TokenName(t.Type), t.Value)
	}
	fmt.Printf("\n共 %d 个 token\n", len(tokens))
	return &DemoResult{TokenCount: len(tokens), Tokens: tokens}, nil
}
