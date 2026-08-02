# regex-engine · AGENTS.md

## 项目内容（What）

Go 1.25 纯标准库实现的**正则表达式引擎**——Thompson NFA 算法，无回溯，抗 ReDoS。完整流水线：parser（正则→AST）→ nfa（Thompson 构造）→ matcher（状态集合模拟）。

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
   │ nfa      │
   └────┬─────┘
        │  *nfa.NFA
        ▼
   ┌──────────┐  状态集合 + ε 闭包（无回溯，O(n×m)）
   │ matcher  │
   └──────────┘
```

**做**：正则→AST→NFA→匹配的完整流水线，支持字面量/`.`/`*`/`+`/`?`/`|`/分组/字符类/取反/转义/`\d \w \s`，无回溯抗 ReDoS。
**不做**：花括号量词 `{n,m}`、反向引用、零宽断言、命名分组（M2）。

## 目标（Goal）

- **G1**：正则引擎流水线（parse → NFA → match）完整工作。
- **G2**：抗 ReDoS——`(a|a)*b` 对长输入线性时间，不指数爆炸。
- **G3**：支持常用正则语法（覆盖 80% 场景）。
- **成功标准**：`go test ./...` 全绿 + demo 跑通 + ReDoS 测试验证线性时间。

## 当前情况（Status）

- **完成度**：**M1 完成**
- **parser**：递归下降，支持字面量/`.`/转义/字符类/量词/选择/分组；锚点 `^`/`$` 在解析阶段直接报错（M2 候选）
- **nfa**：Thompson 构造，每 AST 节点 → start/accept + ε 转移
- **matcher**：状态集合模拟 + ε 闭包 BFS，O(n×m)，无回溯
- **测试**：matcher 15 测试（含抗 ReDoS 关键测试），全绿
- **cmd**：`-d` demo + `-pattern/-text` 单次匹配

## 技术栈与架构

- **语言**：Go 1.25.6，零外部依赖
- **算法参考**：Ken Thompson 1968（Thompson 构造）、Russ Cox 2007 "Regular Expression Matching: the Visual Guide"
- **目录**：cmd + internal（parser/ast/nfa/matcher 各一包）

## 如何运行

```bash
go run ./cmd/regex -d                          # demo
go run ./cmd/regex -pattern "a(b|c)*d" -text "abcd"  # 单次匹配
make test                                      # 全部测试
```

## 与其他项目的关系

- **与 [`lang-impl`](../lang-impl)**：lang-impl 的 lexer 是手写字符扫描；本引擎是通用正则。lang-impl 的词法分析器可以用本引擎实现（正则是词法分析的理论基础）。
- **与 [`consensus-atlas`](../consensus-atlas)**：同范式的零依赖教学库。
- **工作区定位**：补"编译器原理 → 正则引擎"另一闭环（lang-impl 是通用语言，regex-engine 是正则专用）。
