# Lexer · 设计笔记

## 职责

词法分析（lexical analysis / scanning / tokenization）：把源码字符流切成一个个有意义的词法单元（token）。

```
源码字符串 "1 + 2 * 3"
    │
    ▼
  Lexer
    │
    ▼
[Number "1"] [Plus] [Number "2"] [Star] [Number "3"] [EOF]
```

## 核心循环

```
for {
    skipWhitespaceAndComments()
    if eof: break
    c = peek()
    if isDigit(c):   readNumber()   → TokNumber
    if isIdentStart(c): readIdent() → TokIdent 或 关键字
    if c == '"':     readString()   → TokString
    else:            readOperator() → 运算符/分隔符
}
追加 EOF token
```

## 最小可识别特征（少了就不算 lexer）

1. **逐字符消费源码**，产出 token 序列（不是一次性正则替换）。
2. **跳过空白和注释**（它们不产 token）。
3. **多字符 token 识别**（`>=` 而非 `>` `=`；`==` 而非 `=` `=`）。
4. **保留源码位置**（行/列），供错误信息定位。
5. **关键字识别**（`let`/`fn` 等是关键字 token，不是普通 ident）。

## 判定红线

- 把 `>=` 当成 `>` 和 `=` 两个 token → 不是合格 lexer。
- 不跳过注释 → 注释会污染 token 流。
- 不记录行列号 → 错误信息无法定位。

## 与 parser 的边界

lexer **不做语法判断**：
- 不管括号是否匹配（那是 parser 的事）
- 不管 `1 + + 2` 这种连续运算符（lexer 正确产出两个 Plus，parser 才报错）

lexer **不做语义判断**：
- 不管 `1 + "a"` 类型不匹配（那是 type checker / interpreter 的事）

## 本包实现要点

- 手写逐字符扫描（不用 regex，教学清晰）
- `advance()` 同时更新行列号（换行时 line++ col=1）
- 字符串支持转义（`\n \t \" \\`）
- 单独 `&` 或 `|` 报错（M 语言只认 `&&` `||`）

## 参考

- "Crafting Interpreters" Ch.4 Scanning（Robert Nystrom）https://craftinginterpreters.com/scanning.html
- Dragon Book Ch.3 Lexical Analysis
