# regex-engine

> 零依赖手写正则表达式引擎 —— Thompson NFA 算法，无回溯，抗 ReDoS，纯标准库实现。

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)
![零依赖](https://img.shields.io/badge/dependencies-0-success)
![license](https://img.shields.io/badge/license-MIT-blue)

## 为什么有这个项目

工作区有 lang-impl（手写编程语言），但**没有手写正则引擎**。正则是"编译器原理"的另一核心应用（词法分析器的基础就是正则）。`regex-engine` 补这一环：用 Thompson NFA 算法实现正则匹配，展示"正则 → AST → NFA → 状态集合模拟"的完整流程。

## 特点：无回溯，抗 ReDoS

主流库的正则引擎（PCRE/Python re）多用**回溯**实现，对某些模式（如 `(a|a)*b`）会指数级慢——这是 ReDoS 攻击的根源。

本引擎用 **Thompson NFA + 状态集合模拟**（Russ Cox 算法），**无回溯，无指数爆炸**。
- `IsFullMatch`（全匹配）：O(n×m)（n=输入长度，m=正则状态数）
- `Match`（子串匹配）：O(n²×m)（从每个位置尝试，但仍无指数爆炸，对比回溯引擎的最坏 O(2^n)）

```go
// 这个在回溯引擎上会卡死，在本引擎上线性时间完成
m := matcher.New(...)
m.Match("(a|a)*b", strings.Repeat("a", 100)) // 快速返回 false
```

## 支持的语法

| 语法 | 含义 | 示例 |
|------|------|------|
| `abc` | 字面量 | `abc` 匹配 "xabcy" |
| `.` | 任意字符（除换行） | `a.c` 匹配 "axc" |
| `*` | 0 或多次 | `ab*c` 匹配 "ac"/"abc"/"abbbc" |
| `+` | 1 或多次 | `ab+c` 匹配 "abc"，不匹配 "ac" |
| `?` | 0 或 1 次 | `ab?c` 匹配 "ac"/"abc" |
| `\|` | 选择 | `cat\|dog` |
| `(...)` | 分组 | `(ab)+` |
| `[abc]` | 字符类 | `[0-9]+` |
| `[^abc]` | 取反字符类 | `[^0-9]` |
| `\d \w \s` | 预定义类 | `\d+` 匹配数字 |
| `\. \\ \*` | 转义 | `a\.b` 匹配 "a.b" |
| `^ $` | 锚点 | 暂不支持（M2 候选，解析阶段直接报错） |

## 快速开始

```bash
cd regex-engine

# demo
go run ./cmd/regex -d

# 单次匹配
go run ./cmd/regex -pattern "a(b|c)*d" -text "abcd"

# 替换所有匹配
go run ./cmd/regex -pattern "cat" -text "the cat sat" -replace "dog"   # → the dog sat
go run ./cmd/regex -pattern '\d+' -text "a1b2c3" -replace "#"           # → a#b#c#

# 全部测试
make test
```

## 作为库使用

```go
package main

import (
	"fmt"
	"github.com/QiuShichang/regex-engine/internal/matcher"
	"github.com/QiuShichang/regex-engine/internal/nfa"
	"github.com/QiuShichang/regex-engine/internal/parser"
)

func main() {
	ast, _ := parser.Parse(`\w+@\w+\.\w+`)
	m := matcher.New(nfa.Build(ast))
	fmt.Println(m.Match("user@example.com")) // true
	fmt.Println(m.Match("not-an-email"))     // false
}
```

## 流水线

```
   正则字符串 "(a|b)*c"
        │
        ▼
   ┌──────────┐  递归下降（选择<连接<量词<原子）
   │ parser   │
   └────┬─────┘
        │  *ast.Node (AST)
        ▼
   ┌──────────┐  Thompson 算法：每节点 → NFA 片段
   │ nfa      │  （用 ε 转移拼接，state 集合模拟）
   └────┬─────┘
        │  *nfa.NFA
        ▼
   ┌──────────┐  状态集合 + ε 闭包
   │ matcher  │  （无回溯，O(n×m)）
   └──────────┘
```

## 核心设计

- **Thompson NFA**（Ken Thompson 1968）：每个 AST 节点对应一个 NFA 片段（start/accept + ε 转移）
- **状态集合模拟**（Russ Cox 2007）：维护"当前所有可能状态"，每字符推进所有状态，无回溯
- **ε 闭包 BFS**：每步求状态集合的 ε 闭包
- **零外部依赖**：纯标准库

## 相关项目

- [`lang-impl`](../lang-impl) —— 手写编程语言（词法分析器的基础就是正则引擎）
- [`consensus-atlas`](../consensus-atlas) —— 同范式的零依赖教学库
