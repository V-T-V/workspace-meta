// Command langimpl 是 lang-impl 的统一入口。
//
// 用法：
//
//	langimpl -d lex          # 词法分析 demo
//	langimpl -d parse        # 语法分析 demo（生成 AST）
//	langimpl -d interpret    # 解释执行 demo（跑 fib）
//	langimpl -d all          # 全流程：lex → parse → interpret
//	langimpl -d repl         # 交互式 REPL
//	langimpl -d run prog.m   # 运行一个 .m 源文件
//	langimpl -version
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/QiuShichang/lang-impl/internal/interpreter"
	"github.com/QiuShichang/lang-impl/internal/lexer"
	"github.com/QiuShichang/lang-impl/internal/parser"
	"github.com/QiuShichang/lang-impl/internal/wasm"
)

var version = "dev"

func main() {
	var (
		demo    string
		showVer bool
	)
	flag.StringVar(&demo, "d", "lex", "demo: lex|parse|interpret|all|repl|run <file>")
	wasmOut := flag.String("wasm", "", "编译到 WASM：-wasm <file.m> 输出 <file.wasm>")
	flag.BoolVar(&showVer, "version", false, "打印版本号")
	flag.Parse()

	if showVer {
		fmt.Println("lang-impl", version)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// -wasm 模式：编译到 WebAssembly
	if *wasmOut != "" {
		exit(compileToWasm(*wasmOut))
		return
	}

	// 特殊：run 模式跑文件
	if demo == "run" {
		args := flag.Args()
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "用法: langimpl -d run <file.m>")
			os.Exit(1)
		}
		exit(runFile(ctx, args[0]))
		return
	}

	if demo == "repl" {
		runREPL(ctx)
		return
	}

	if err := run(ctx, demo); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, demo string) error {
	if demo == "all" {
		for _, d := range []string{"lex", "parse", "interpret"} {
			fmt.Printf("\n========== ▶ %s ==========\n", d)
			if err := run(ctx, d); err != nil {
				return err
			}
		}
		return nil
	}
	switch demo {
	case "lex":
		_, err := lexer.Demo(ctx)
		return err
	case "parse":
		_, err := parser.Demo(ctx)
		return err
	case "interpret":
		_, err := interpreter.Demo(ctx)
		return err
	default:
		return fmt.Errorf("未知 demo: %s（可选: lex|parse|interpret|all|repl|run）", demo)
	}
}

// compileToWasm 把 .m 源文件编译成 .wasm 二进制文件。
func compileToWasm(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}
	tokens, err := lexer.Tokenize(string(src))
	if err != nil {
		return err
	}
	prog, err := parser.Parse(tokens)
	if err != nil {
		return err
	}
	mod, err := wasm.Compile(prog)
	if err != nil {
		return err
	}
	wasmBytes := mod.Bytes()
	outPath := path + ".wasm"
	if err := os.WriteFile(outPath, wasmBytes, 0o644); err != nil {
		return err
	}
	fmt.Printf("✅ 编译完成: %s → %s（%d 字节，%d 函数）\n", path, outPath, len(wasmBytes), mod.FunctionCount())
	fmt.Printf("   导出函数: %v\n", mod.ExportedFunctions())
	fmt.Printf("   hex: %s...（前 32 字节）\n", mod.HexString()[:min(64, len(mod.HexString()))])
	fmt.Printf("\n用 node 执行:\n  node -e \"const w=new WebAssembly.Module(require('fs').readFileSync('%s'));const i=new WebAssembly.Instance(w);console.log(i.exports)\"\n", outPath)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// runFile 运行一个 M 源文件：lex → parse → interpret。
func runFile(ctx context.Context, path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}
	tokens, err := lexer.Tokenize(string(src))
	if err != nil {
		return err
	}
	prog, err := parser.Parse(tokens)
	if err != nil {
		return err
	}
	result, err := interpreter.Run(prog)
	if err != nil {
		return err
	}
	if result != nil {
		fmt.Println("结果:", formatValue(result))
	}
	return nil
}

// runREPL 启动交互式 read-eval-print loop。
// 状态保持：REPL 维护一个累积历史（已执行的语句），每次新输入追加到历史末尾，
// 重新 lex→parse→interpret 全部历史。这样 let/fn 定义的状态能保持到后续输入。
// 对小规模交互输入够用（教学 REPL 不追求性能）。
func runREPL(ctx context.Context) {
	fmt.Printf("M 语言 REPL %s（输入 :q 退出）\n", version)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	history := "" // 累积已执行语句
	for {
		fmt.Print("m> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == ":q" || line == ":quit" {
			break
		}
		if line == "" {
			continue
		}
		// 追加到历史，重新求值全部
		prevHistory := history
		history = history + "\n" + line
		result, err := evalHistory(history)
		if err != nil {
			fmt.Println("  error:", err)
			history = prevHistory // 出错则回滚历史，不让坏语句污染
			continue
		}
		if result != nil {
			fmt.Println("  =", formatValue(result))
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "REPL 读取错误:", err)
	}
}

// evalHistory 解析并执行累积历史，返回最后一条表达式语句的值。
func evalHistory(src string) (any, error) {
	tokens, err := lexer.Tokenize(src)
	if err != nil {
		return nil, err
	}
	prog, err := parser.Parse(tokens)
	if err != nil {
		return nil, err
	}
	return interpreter.Run(prog)
}

// formatValue 把解释器的值格式化成字符串。
func formatValue(v any) string {
	switch x := v.(type) {
	case string:
		return fmt.Sprintf("%q", x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func exit(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}
