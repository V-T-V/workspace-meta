# Parser · 设计笔记

## 职责

语法分析（parsing）：把 lexer 产出的 `[]core.Token` 序列，按 M 的文法规则，
组织成抽象语法树（AST）——也就是 `*core.Program`。

```
token 流: [fn] [ident: fib] [(] [ident: n] [)] [{] ... [EOF]
                     │
                     ▼
                   Parser
                     │
                     ▼
                 AST 树
        (FnDecl fib(n) (Block ...))
```

## 核心循环

```
ParseProgram:
  while not EOF:
      stmt = parseStmt()
      append(stmt)
  return Program{stmts}

parseStmt:  // 按首 token 分派
  switch peek().Type:
    TokLet    → parseLet       // let id = expr ;
    TokFn     → parseFn        // fn id ( params ) block
    TokIf     → parseIf        // if ( expr ) block (else block)?
    TokWhile  → parseWhile     // while ( expr ) block
    TokReturn → parseReturn    // return expr? ;
    TokLBrace → parseBlock     // { stmts }
    else      → parseExprStmt  // expr ;
```

辅助（标准递归下降四件套）：

```
peek()      看当前 token（不消费）
advance()   消费并返回当前 token
check(t)    当前 token 是否为 t
expect(t)   当前 token 必须为 t，否则带 Loc 报错
match(ts…)  若当前 ∈ ts 之一则消费并返回 true
```

## 运算符优先级表（从低到高）

| 级别 | 运算符              | 结合性 | 解析函数          |
|------|---------------------|--------|-------------------|
| 1    | `\|\|`              | 左     | parseOr           |
| 2    | `&&`                | 左     | parseAnd          |
| 3    | `== !=`             | 左     | parseEquality     |
| 4    | `> < >= <=`         | 左     | parseComparison   |
| 5    | `+ -`               | 左     | parseTerm         |
| 6    | `* / %`             | 左     | parseFactor       |
| 7    | `! -`（一元前缀）   | 右     | parseUnary        |
| 8    | 字面量 / ident / 调用 / `( )` | — | parsePrimary |

> 同级用 `while` 循环卷起实现左结合（`1-2-3 → ((1-2)-3)`）。
> 一元运算符用递归（`!!a → !(!a)`）实现右结合。
> 括号 `( expr )` 直接返回内层表达式，等于"提到最高优先级"。

## 最小可识别特征（少了就不算 parser）

1. **消费 token 序列**，不是消费字符流（那是 lexer 的活）。
2. **正确的运算符优先级**：`1 + 2 * 3` 解析成 `1 + (2*3)`，不是 `(1+2)*3`。
3. **正确的结合性**：左结合的 `-` 让 `1-2-3` = `((1-2)-3)`。
4. **递归下降**：每条文法一个函数，函数间互相调用形成 AST。
5. **错误带 SourceLoc**：缺分号、缺括号时用出错 token 的 Loc 报错。

## 判定红线

- 把 `1 + 2 * 3` 解析成 `(1+2)*3` → 优先级错，**不合格**。
- 把 `1 - 2 - 3` 解析成 `1 - (2-3)` → 结合性错（应左结合），**不合格**。
- 缺分号/缺括号时静默通过 → 没做语法校验，**不合格**。
- 错误不带行号 → 无法定位，**不合格**。
- 用 `goyacc`/`antlr` 生成 → 不是"手写递归下降"（虽然语法上正确，
  但本包的教学目标是展示手写技术）。

## 与 lexer 的边界

parser **不重新做词法**：
- token 类型、值、位置完全信任 lexer。
- lexer 产出两个连续 `Plus`（`1 + + 2`）是合法的 token 流，parser 才报"期望表达式"。

parser **不扫描源码字符**：
- 不关心空白、注释、转义——那是 lexer 的事。

## 与 interpreter 的边界

parser **不做类型检查**：
- `"a" + 1` 在语法上是合法的 BinaryExpr，类型错误由 interpreter 在求值时报。

parser **不执行**：
- 不求值、不调用函数、不绑定变量。它只产出静态的 AST 结构。

## 本包实现要点

- 手写递归下降（无 yacc/antlr）。
- 优先级阶梯：8 个 parse 函数（parseOr → parsePrimary）层层下降。
- `AST.Printer`：把 AST 转成缩进树形字符串（demo 与调试用）。
- `parseIf` 支持 `else if` 链：把 `else if (...) {...}` 包成单语句 Block。
- 错误统一用 `core.NewError(loc, ...)`，Loc 取自出错 token。

## 参考

- "Crafting Interpreters" Ch.8 Statements and State / Ch.6 Parsing Expressions
  https://craftinginterpreters.com/parsing-expressions.html
  https://craftinginterpreters.com/statements-and-state.html
- Dragon Book Ch.4 Syntax-Directed Translation（递归下降子集）
- Vaughan-Pratt "Top Down Operator Precedence"（优先级阶梯的另一种视角）
