# lang-impl · AGENTS.md

## 项目内容（What）

Go 1.25 纯标准库实现的**玩具编程语言 "M" 编译器**——完整覆盖编译器核心流水线：词法分析（lexer）→ 语法分析（递归下降 parser）→ AST → 树遍历解释器，含交互式 REPL + 示例程序。

```
   源码字符串
       │
       ▼
   ┌─────────┐  字符流 → token 序列
   │ lexer   │
   └────┬────┘
        │  []Token
        ▼
   ┌─────────┐  递归下降（优先级阶梯）
   │ parser  │
   └────┬────┘
        │  *Program (AST)
        ▼
   ┌─────────────┐  树遍历求值（环境链 + 函数表）
   │ interpreter │
   └─────────────┘
```

**做**：M 语言完整实现（数字/字符串/布尔字面量、算术/比较/逻辑运算符、let/fn/if/while/return 语句、函数调用含递归）、动态类型、错误带源码位置、REPL、4 个示例程序。
**不做**：生产级性能（树遍历慢，要快需字节码/LLVM）、完整类型系统、标准库、闭包（M3 候选）。

## 目标（Goal）

- **G1**：M 语言源码能完整走 lex → parse → interpret 流水线，跑通 fib 等示例。
- **G2**：每个编译阶段（lexer/parser/interpreter）是独立包，5 件套齐全（impl/test/demo/doc.go/NOTES.md）。
- **G3**：错误信息带源码位置（行:列），便于定位。
- **G4**：零外部依赖（go.mod 无 require），教学清晰。
- **G5**：交互式 REPL 支持逐行求值。
- **成功标准**：`go test ./...` 全绿 + 三阶段 demo + REPL + examples/fib.m 跑通（fib(10)=55）。

## 当前情况（Status）

- **完成度**：**M1 完成**——lexer/parser/interpreter 全部完成，测试全绿，demo + REPL + 4 examples 跑通
- **底座**（`internal/core`，已完成）：
  - `token.go`：TokenType 常量 + Token 结构 + TokenName（数字/字符串/布尔/标识符/6 关键字/13 运算符/6 分隔符/EOF）
  - `ast.go`：Expr/Stmt 接口 + 14 个具体节点（Number/String/Bool/Ident/Binary/Unary/Call/Let/FnDecl/If/While/Return/Expr/Block/Program）
  - `error.go`：带 SourceLoc 的 Error
- **lexer**（5 件套，已完成）：手写逐字符扫描 + 多字符运算符识别 + 字符串转义 + 行注释 + 14 测试
- **parser**（5 件套，编写中）：递归下降 + 优先级阶梯
- **interpreter**（5 件套，编写中）：树遍历求值 + 环境链 + 函数表
- **REPL**（在 cmd 里，已完成）：逐行 lex→parse→interpret
- **examples**：fib.m / arithmetic.m / loop.m / logic.m

## 技术栈与架构

- **语言**：Go 1.25.6
- **依赖**：**零外部依赖**（module 只引标准库：fmt / strings / bufio）
- **设计参考**：go-agent-research / consensus-atlas（5 件套 + Demo 入口范式）、Crafting Interpreters（树遍历解释器）、Dragon Book
- **目录**：cmd + internal（参照 go-agent-research），各编译阶段独立包，只共享 internal/core

## 如何运行

```bash
go run ./cmd/langimpl -d lex        # 词法分析 demo
go run ./cmd/langimpl -d parse      # 语法分析 demo（AST 树）
go run ./cmd/langimpl -d interpret  # 解释执行 demo（fib）
go run ./cmd/langimpl -d all        # 全流程
go run ./cmd/langimpl -d repl       # 交互式 REPL
go run ./cmd/langimpl -d run examples/fib.m   # 跑源文件
make test                           # 全部测试
make build                          # 构建到 bin/
```

## 关键约定

- **零外部依赖是灵魂约束**：go.mod 无 require，纯标准库实现。
- **5 件套齐全**：每个编译阶段包含 impl + test + demo + doc.go + NOTES.md。
- **错误带源码位置**：所有 Error 含 SourceLoc（行:列）。
- **递归下降 parser**：手写优先级阶梯，不用生成器（教学清晰）。
- **动态类型**：值是 int64/bool/string，运行时按值类型做运算。
- **确定性**：demo 纯函数式，无 goroutine/time/rand。

## 与其他项目的关系

- **与 [`go-agent-research`](../go-agent-research) / [`consensus-atlas`](../consensus-atlas) 同范式**：Go 零依赖教学库，5 件套 + Demo 入口，本库的目录结构/文档风格全部对齐它们。
- **与 [`language-research`](../language-research)**：lang-impl 补"实现语言"闭环（language-research 研究，lang-impl 实现）。
- **与 [`logos-formal`](../logos-formal)**：lang-impl 补"实现"闭环的另一端（logos-formal 形式化证明，lang-impl 可执行实现）。
- **与 [`algorithms-atlas`](../algorithms-atlas)**：M3 可视化 AST 执行器可移植 algorithms-atlas 的 trace recorder/playback 思路。
- **工作区定位**：补全工作区在"编译器/语言实现"象限的空白。
