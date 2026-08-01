# lang-impl

> 一门玩具编程语言 "M" 的零依赖实现 —— 完整编译器流水线（词法 → 语法 → AST → 解释器 → WASM 后端），用 Go 纯标准库手写，教学向。

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)
![零依赖](https://img.shields.io/badge/dependencies-0-success)
![license](https://img.shields.io/badge/license-MIT-blue)

## 为什么有这个项目

工作区有 `language-research`（研究语言分层）+ `logos-formal`（Lean 形式化）+ `lightai`（轻量算法），但**没有一个"亲手实现一门语言"**的项目。`lang-impl` 补上"研究语言 → 形式化 → 实现语言"闭环的最后一环，用最小可读的代码复刻编译器核心概念（Crafting Interpreters 风格）。

## M 语言一览

```
// 递归 fib
fn fib(n) {
  if (n < 2) { return n; }
  return fib(n - 1) + fib(n - 2);
}
let r = fib(10);   // 55
```

- **类型**：动态类型（int64 / bool / string，运行时按值判断）
- **字面量**：数字 `123` / 字符串 `"hi"` / 布尔 `true false`
- **运算符**：`+ - * / %`（算术）/ `> < >= <= == !=`（比较）/ `&& || !`（逻辑）
- **语句**：`let x = expr;`（绑定）/ `fn name(a,b) {...}`（函数）/ `if (cond) {...} else {...}` / `while (cond) {...}` / `return expr;` / `expr;`
- **表达式**：字面量/标识符/二元运算/一元运算/函数调用/括号
- **注释**：`// 行注释`

## 快速开始

```bash
cd lang-impl

# 各阶段 demo
go run ./cmd/langimpl -d lex        # 词法分析：源码 → token 序列
go run ./cmd/langimpl -d parse      # 语法分析：token → AST 树
go run ./cmd/langimpl -d interpret  # 解释执行：fib(10) = 55
go run ./cmd/langimpl -d all        # 全流程

# 交互式 REPL
go run ./cmd/langimpl -d repl
# m> 1 + 2 * 3
#   = 7
# m> :q

# 运行源文件
go run ./cmd/langimpl -d run examples/fib.m
# 结果: 55

# 全部测试
make test
```

## 编译器流水线

```
   源码字符串
       │
       ▼
   ┌─────────┐  字符流 → token 序列
   │ lexer   │  （数字/标识符/关键字/运算符/分隔符，跳过空白注释）
   └────┬────┘
        │  []Token
        ▼
   ┌─────────┐  递归下降，按优先级阶梯
   │ parser  │  （parseOr → parseAnd → ... → parsePrimary）
   └────┬────┘
        │  *Program (AST 根)
        ▼
   ┌─────────────┐
   │ interpreter │  树遍历求值（环境链 + 函数表）
   └─────┬───────┘
         │  result
         ▼
   ┌─────────────────┐
   │ wasm 后端（M2） │  编译到 WASM 字节码（手写 []byte 拼装）
   └─────────────────┘
```

## 核心设计

- **递归下降 parser**：手写，每级一个 parse 函数，运算符优先级用阶梯（不用 yacc/antlr，教学清晰）
- **树遍历解释器**：直接递归遍历 AST 求值，动态类型，环境链管作用域
- **零外部依赖**：go.mod 无 require，纯标准库
- **错误带源码位置**：所有错误含 `行:列`，精确定位
- **5 件套结构**：每个编译阶段（lexer/parser/interpreter）含 impl + test + demo + doc.go + NOTES.md

## 目录结构

```
lang-impl/
├── go.mod / Makefile / LICENSE
├── README.md / AGENTS.md
├── cmd/langimpl/main.go        # -d lex|parse|interpret|all|repl|run 入口
├── internal/
│   ├── core/                   # 共享底座：Token / AST / Error / SourceLoc
│   ├── lexer/                  # 词法分析（5 件套）
│   ├── parser/                 # 语法分析 → AST（5 件套）
│   └── interpreter/            # 树遍历解释器（5 件套）
│   # WASM 后端（M2 待建）、REPL 在 cmd/langimpl 里实现
└── examples/                   # .m 示例程序（fib/算术/循环/逻辑）
```

## 学习路径

按编译器流水线顺序：

1. **lexer**（最易）—— 理解"字符流 → token"的词法分析
2. **parser**（中难）—— 理解递归下降 + 运算符优先级 + AST 构造
3. **interpreter**（中）—— 理解树遍历求值 + 环境/作用域 + 函数调用栈帧
4. **wasm**（M2，最难）—— 理解目标代码生成（AST → WASM 字节码）

每个阶段的 NOTES.md 写了核心循环 + 判定红线 + 参考（Crafting Interpreters / Dragon Book）。

## 路线图

- **M1（当前）**：lexer + parser + interpreter 完整流水线 + REPL + 示例程序
- **M2**：WASM 字节码后端（手写 []byte 拼装，产出可运行的 .wasm）
- **M3 候选**：闭包/数组/类型系统/可视化 AST 执行器（移植 algorithms-atlas 的 trace 思路）

## 不做的

- 生产级性能（树遍历本就慢，要快得上字节码/LLVM，超出教学范围）
- 完整类型系统（动态类型够教学；静态类型检查是另一门课）
- 标准库（M 语言只做演示，不追求写真实程序）

## 相关项目

- [`go-agent-research`](../go-agent-research) / [`consensus-atlas`](../consensus-atlas) —— 同范式的 Go 零依赖教学库（5 件套 + Demo 入口）
- [`language-research`](../language-research) —— 语言研究（lang-impl 补"实现"闭环）
- [`logos-formal`](../logos-formal) —— Lean 形式化（lang-impl 补"实现"闭环的另一端）
