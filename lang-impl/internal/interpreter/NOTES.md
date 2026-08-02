# Interpreter · 设计笔记

## 职责

解释执行（interpretation）：遍历 parser 产出的 AST，按节点类型求值/执行，
产出最终结果（最后一条表达式语句的值，或顶层 return 的值）。

```
AST (Program)
    │
    ▼
  Interpreter.Run
    │   for each top-level stmt:
    │     FnDecl → 注册到 funcs
    │     其他  → execStmt（顶层 let 绑定 globals，表达式求值）
    ▼
最终结果 (any: int64 / bool / string / nil)
```

## 核心循环

```
Run(program):
  last = nil
  for stmt in program.Stmts:
      if stmt 是 FnDecl: funcs[name] = decl        // 注册
      else:
          v = execStmt(stmt, globals)
          if stmt 是 ExprStmt: last = v
  return last

execStmt(stmt, env):
  LetStmt    → v = eval(Init); env.Set(name, v)
  ExprStmt   → eval(Expr)（返回值参与 last）
  IfStmt     → b = asBool(eval(Cond)); 执行 Then/Else
  WhileStmt  → while asBool(eval(Cond)): execBlock(body)
  ReturnStmt → panic(returnSignal{eval(Value)})    // 非局部跳转
  BlockStmt  → execBlock（开子作用域）

evalExpr(expr, env):
  Number/String/Bool → 字面量值
  Ident    → env.Get(name)
  Unary    → eval(right) 后按 op 取反/取非
  Binary   → eval(left), eval(right) 后按 op 算
  Call     → 找函数 → 求值 args → 新建 callEnv → 执行 body（recover return）
```

## 环境 / 作用域

```
Environment { vars: map[name]any, parent: *Environment }
```

- **全局环境**（globals）：顶层 let 绑定到这里；函数表 funcs 独立存放。
- **函数调用环境**：parent 指向 **globals**（不捕获调用者局部变量 → 不形成闭包）。
- **块作用域**：每次执行 BlockStmt 在当前 env 上开一个子 env。

`Get(name)` 沿 parent 链向上查找；`Set(name, val)` 只写当前层（let 语义）。

## 函数调用栈帧

```
caller env ──┐
             │  （不传递）
             ▼
        callEnv (parent = globals)
          ├─ param1 = arg1
          ├─ param2 = arg2
          └─ ...
          execBlock(fn.Body, callEnv)
              │
              ▼
          遇 ReturnStmt → panic(returnSignal{value})
              │
              ▼
          callFunction 的 defer recover 捕获 → 返回 value
```

每个 M 函数调用 = 一个 callEnv + 一次 execBlock。
return 通过 panic 跨任意多层嵌套（if/while/block）直达函数边界。

## return 的实现：panic + recover

树遍历里，return 要从"任意深度的嵌套 block/if/while"跳回函数调用处。
两种做法：

1. 每个执行函数都返回 `(value, returned bool)`——繁琐、易错。
2. 用 panic 抛一个 `returnSignal` 信号，在 `callFunction` / `Run` 边界 recover——**本包采用**。

```go
type returnSignal struct{ value any; loc SourceLoc }

// ReturnStmt:
panic(returnSignal{value: v, loc: s.Loc})

// callFunction:
defer func() {
    if r := recover(); r != nil {
        if sig, ok := r.(returnSignal); ok {
            result = sig.value  // 捕获，正常返回
            return
        }
        panic(r)               // 非 return 信号：重新抛出
    }
}()
```

非 return 的真 panic（如 nil 解引用）会被重新抛出，不会被吞掉。

## 最小可识别特征（少了就不算解释器）

1. **直接遍历 AST 求值**，不是先编译成字节码（那是 VM）。
2. **动态类型**：值在运行时按类型分发（int64/bool/string）。
3. **作用域链**：变量查找沿 parent 链向上。
4. **函数调用栈帧**：每次调用新建环境、执行 body、返回值。
5. **运行时类型检查**：`"a" + 1`、`1/0` 等在执行时报错（不是 parse 时）。

## 判定红线

- `"a" + 1` 不报错（偷偷转字符串或忽略）→ 没做类型检查，**不合格**。
- `1 / 0` 不报错 → 没做除零检查，**不合格**。
- 未定义变量/函数不报错 → **不合格**。
- 函数 return 的值取不到（被嵌套 block 吞掉）→ return 控制流实现错误。
- 静默返回 nil 而非真值（当 return 有值时）→ 调用栈帧处理错误。
- 把所有类型当整数（bool/string 不支持）→ 不是动态类型。

## 与 parser 的边界

interpreter **不重新解析**：
- 接收的是已校验过的 AST（语法合法）。
- 只做"语义/运行时"检查：类型、未定义符号、除零、参数数量。

interpreter **不知道源码字符**：
- 行号来自 AST 节点的 NodeLoc()（parser/lexer 留下的位置）。

## 与编译器（wasm/codegen 后端）的边界

本包是"边遍历边执行"，没有 IR（中间表示）。
编译器后端会另起一套：AST → IR/WASM → 执行。
两者共用 core.AST，但执行模型不同。

## 本包实现要点

- 树遍历：evalExpr / execStmt 双分派（按节点类型 switch）。
- 动态类型：值是 `any`（int64 / bool / string / nil），运行时断言。
- 环境链：`Environment{vars, parent}`，函数调用 parent=globals（无闭包）。
- return 用 `panic(returnSignal)` + `defer recover` 实现非局部跳转。
- `&&`/`||` 短路求值（左操作数已定时不求右）。
- 相等（==/!=）跨类型宽松：类型不同直接判不等（不报错）；
  算术/比较严格：要求两边都是 int64，否则报类型错误。
- M 的 `let` 是**绑定**（一次性），不支持赋值表达式（`= ` 仅在 let 中出现）。

## 参考

- "Crafting Interpreters" Ch.13 Evaluating Expressions / Ch.12 Birth of VM
  https://craftinginterpreters.com/evaluating-expressions.html
- "Writing An Interpreter In Go"（Thorsten Ball）—— 树遍历 + 环境链经典实现
- SICP Ch.4 The Metacircular Evaluator（环境模型 / apply）
