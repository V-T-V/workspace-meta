// Command regex 是 regex-engine 的入口：编译正则、匹配文本、展示 NFA。
//
// 用法：
//
//	regex -d                                        # demo：几个正则匹配示例
//	regex -pattern "a(b|c)*d" -text "abcd"          # 单次匹配
//	regex -pattern "\d+" -text "abc123def"          # 子串匹配
//	regex -pattern "cat" -text "the cat sat" -replace "dog"  # 替换所有匹配
//	regex -pattern "\d+" -text "a1b2c3" -replace "#"          # → a#b#c#
//	regex -version
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/QiuShichang/regex-engine/internal/matcher"
	"github.com/QiuShichang/regex-engine/internal/nfa"
	"github.com/QiuShichang/regex-engine/internal/parser"
)

var version = "dev"

func main() {
	pattern := flag.String("pattern", "", "正则模式")
	text := flag.String("text", "", "要匹配的文本")
	replace := flag.String("replace", "", "替换串：把所有匹配替换为此串并输出（需配合 -pattern/-text）")
	demo := flag.Bool("d", false, "跑 demo（几个匹配示例）")
	showVer := flag.Bool("version", false, "打印版本号")
	flag.Parse()
	replaceSet := isFlagSet(flag.CommandLine, "replace")

	if *showVer {
		fmt.Println("regex-engine", version)
		return
	}

	if *demo {
		runDemo()
		return
	}

	if *pattern == "" {
		fmt.Fprintln(os.Stderr, "用法: regex -pattern <regex> -text <text> [-replace <replacement>]  或  -d 跑 demo")
		os.Exit(1)
	}

	// 编译正则
	ast, err := parser.Parse(*pattern)
	if err != nil {
		fmt.Fprintln(os.Stderr, "解析错误:", err)
		os.Exit(1)
	}
	m := matcher.New(nfa.Build(ast))

	// 匹配
	if *text == "" {
		fmt.Fprintln(os.Stderr, "缺少 -text")
		os.Exit(1)
	}

	// 替换模式：把所有匹配替换为 -replace 后输出
	if replaceSet {
		fmt.Println(m.ReplaceAll(*text, *replace))
		return
	}

	if m.Match(*text) {
		fmt.Printf("✅ %q 匹配 %q\n", *pattern, *text)
	} else {
		fmt.Printf("❌ %q 不匹配 %q\n", *pattern, *text)
	}
	_ = context.Background()
}

// isFlagSet 报告 name flag 是否在命令行被显式设置。
// 用 flag.Visit（仅遍历实际设置的 flag）实现——能区分"未设置"与"设置为空串"，
// 这样 -replace ""（替换为空串，即删除匹配）也能正确进入替换分支。
func isFlagSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func runDemo() {
	cases := []struct {
		pattern, text string
	}{
		{`abc`, "xabcy"},
		{`a.c`, "axc"},
		{`a.c`, "ac"},
		{`ab*c`, "ac"},
		{`ab*c`, "abbbbc"},
		{`ab+c`, "ac"},
		{`(cat|dog)`, "I have a dog"},
		{`(cat|dog)`, "I have a bird"},
		{`[0-9]+`, "abc123"},
		{`\w+@\w+\.\w+`, "user@example.com"},
		{`\w+@\w+\.\w+`, "not-an-email"},
	}
	fmt.Println("=== 正则引擎 demo ===")
	fmt.Println("模式 → 文本 → 是否匹配")
	fmt.Println(strings.Repeat("-", 50))
	for _, c := range cases {
		ast, _ := parser.Parse(c.pattern)
		m := matcher.New(nfa.Build(ast))
		mark := "❌"
		if m.Match(c.text) {
			mark = "✅"
		}
		fmt.Printf("  %-22s %q → %s\n", c.pattern, c.text, mark)
	}
}
