// Package interpreter 实现 M 语言的树遍历（tree-walking）解释器：
// 直接递归遍历 parser 产出的 AST，按节点类型求值/执行。
//
// 特性：
//   - 动态类型：值是 int64 / bool / string，运行时按值类型做运算
//   - 环境（environment）作用域链：函数调用新开一层
//   - 函数调用：求值参数 → 绑定到形参环境 → 执行 body → 遇 return 返回
//   - 错误：除零、类型不匹配、未定义变量/函数、参数数量不匹配
//
// 更多背景见 NOTES.md。
package interpreter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QiuShichang/lang-impl/internal/core"
	"github.com/QiuShichang/lang-impl/internal/lexer"
	"github.com/QiuShichang/lang-impl/internal/parser"
)

// ===== 环境（作用域链） =====

// Environment 是变量绑定环境，构成作用域链。
// 顶层 let 绑定到全局环境；函数调用时新建一个子环境（parent 指向全局）。
type Environment struct {
	vars   map[string]any
	parent *Environment
}

// NewEnvironment 创建一个空环境（parent 可为 nil 表示根）。
func NewEnvironment(parent *Environment) *Environment {
	return &Environment{vars: make(map[string]any), parent: parent}
}

// Get 取变量值（沿作用域链向上查找）。未找到返回错误。
func (e *Environment) Get(name string) (any, bool) {
	if v, ok := e.vars[name]; ok {
		return v, true
	}
	if e.parent != nil {
		return e.parent.Get(name)
	}
	return nil, false
}

// Set 在当前环境层定义/覆盖一个变量（用于 let 绑定）。
// Set 设置变量值。语义：
//   - 若变量在外层（含当前）作用域已存在，则更新那一层的值（真正的"赋值"）。
//   - 若变量不存在，在当前层新建（用于 let 首次绑定）。
//
// "沿链向上找再更新"是 while 循环里 i = i + 1 能改变外层 i 的关键。
// 若总是写当前层，block 作用域会屏蔽外层变量导致死循环。
func (e *Environment) Set(name string, val any) {
	env := e
	for env != nil {
		if _, ok := env.vars[name]; ok {
			env.vars[name] = val // 更新已存在的那一层
			return
		}
		env = env.parent
	}
	e.vars[name] = val // 不存在则当前层新建
}

// Define 在当前环境层无条件新建一个变量（不论外层是否已有同名）。
// 用于 let 绑定和函数参数：它们总是新建当前层变量，实现块级作用域 shadow。
func (e *Environment) Define(name string, val any) {
	e.vars[name] = val
}

// ===== 函数表 =====

// function 是一个已注册的 M 语言函数（来自 FnDecl）。
// closure 存定义时的环境，用于支持闭包（函数捕获外层局部变量）。
// 顶层函数的 closure = globals（行为不变）；嵌套函数的 closure = 外层局部环境。
type function struct {
	decl    *core.FnDecl
	closure *Environment
}

// Interpreter 是树遍历解释器，持有全局环境与函数表。
type Interpreter struct {
	globals *Environment
	funcs   map[string]*function
}

// New 创建 Interpreter，初始化全局环境与空函数表。
func New() *Interpreter {
	return &Interpreter{
		globals: NewEnvironment(nil),
		funcs:   make(map[string]*function),
	}
}

// Run 是包级便捷入口：New().Run(program)。
func Run(program *core.Program) (any, error) {
	return New().Run(program)
}

// Run 执行整个程序：
//   - FnDecl → 注册到函数表
//   - 其他语句 → 立即执行（顶层 let 绑定全局，表达式求值）
//
// 返回最后一条顶层表达式语句的值，或顶层 return 的值；都没有则返回 nil。
//
// 注意：返回值必须命名（result/err），否则 deferred recover 无法在 panic
// 发生后把顶层 return 的值写出去（panic 会跳过 return 语句）。
func (itp *Interpreter) Run(program *core.Program) (result any, err error) {
	// 顶层 return 通过 panic 抛出（returnSignal），这里 recover。
	// recover 必须在 deferred 函数里调用；命名返回值让 recover 能写出结果。
	defer func() {
		if r := recover(); r != nil {
			if sig, ok := r.(returnSignal); ok {
				result = sig.value // 顶层 return 的值
				err = nil
				return
			}
			// 非控制流 panic：重新抛出（让 Go runtime 处理）
			panic(r)
		}
	}()

	var last any
	for _, stmt := range program.Stmts {
		// 函数声明：注册到函数表，捕获当前环境作为闭包。
		// 顶层函数的 closure = globals（行为不变）。
		if fn, ok := stmt.(*core.FnDecl); ok {
			itp.funcs[fn.Name] = &function{decl: fn, closure: itp.globals}
			continue
		}
		// 其他语句：执行
		val, e := itp.execStmt(stmt, itp.globals)
		if e != nil {
			err = e
			return
		}
		// 只把 ExprStmt 的值记为 "最后结果"
		if _, isExpr := stmt.(*core.ExprStmt); isExpr {
			last = val
		}
	}
	result = last
	return
}

// returnSignal 用 panic 实现 return 的非局部跳转（树遍历解释器标准技巧）。
type returnSignal struct {
	value any
	loc   core.SourceLoc
}

// breakSignal / continueSignal 用 panic 实现 break/continue 的非局部跳转。
// 与 returnSignal 同理：break/continue 需要从嵌套的 if/块里跳出多层，
// panic+recover 是树遍历解释器最简洁的实现方式。
// 它们由最近的循环（while/for）的执行体用 defer recover 捕获：
//   - breakSignal：跳出整个循环
//   - continueSignal：跳过本轮剩余语句，进入下一轮（while 重新求值 cond；
//     for 先执行 update 再求值 cond）
//
// 在循环外使用 break/continue 会让信号冒泡到顶层 Run，Run 的 recover 不识别
// 这两种信号（只认 returnSignal），从而重新 panic——表现为运行时错误。
// 这是合理的：循环外 break/continue 本就是非法的。
type breakSignal struct {
	loc core.SourceLoc
}

type continueSignal struct {
	loc core.SourceLoc
}

// ===== 语句执行 =====

// execStmt 执行一条语句，返回表达式语句的值（非表达式语句返回 nil）。
func (itp *Interpreter) execStmt(stmt core.Stmt, env *Environment) (any, error) {
	switch s := stmt.(type) {
	case *core.LetStmt:
		// let 和裸赋值都解析为 LetStmt，但语义不同：
		// - let：Define（当前层新建，shadow 外层）
		// - 裸赋值：Set（沿链向上找并更新外层）
		// parser 区分二者靠"是否有 let 关键字"，但 AST 都用 LetStmt。
		// 这里用 Set 语义（沿链更新）——对裸赋值正确，对 let 首次绑定也正确
		// （首次绑定时链上找不到该变量，Set 会落到当前层新建）。
		// 唯一差异是"块内 let 同名变量 shadow"，由 parseLet 时新开 block 环境保证。
		v, err := itp.evalExpr(s.Init, env)
		if err != nil {
			return nil, err
		}
		if s.IsAssign {
			// 裸赋值：沿链向上找并更新（while 循环里改外层 i 必需）
			env.Set(s.Name, v)
		} else {
			// let：当前层新建（块级作用域 shadow 语义）
			env.Define(s.Name, v)
		}
		return nil, nil
	case *core.ReturnStmt:
		var v any
		if s.Value != nil {
			val, err := itp.evalExpr(s.Value, env)
			if err != nil {
				return nil, err
			}
			v = val
		}
		// 抛出 return 信号，向上冒泡直到最近一次函数调用 / 顶层 Run
		panic(returnSignal{value: v, loc: s.Loc})
	case *core.BreakStmt:
		// 抛出 break 信号，向上冒泡直到最近的循环（while/for）捕获。
		panic(breakSignal{loc: s.Loc})
	case *core.ContinueStmt:
		// 抛出 continue 信号，向上冒泡直到最近的循环（while/for）捕获。
		panic(continueSignal{loc: s.Loc})
	case *core.IfStmt:
		cond, err := itp.evalExpr(s.Cond, env)
		if err != nil {
			return nil, err
		}
		b, err := asBool(cond, s.Cond.NodeLoc())
		if err != nil {
			return nil, err
		}
		if b {
			return itp.execBlock(s.Then, env)
		}
		if s.Else != nil {
			return itp.execBlock(s.Else, env)
		}
		return nil, nil
	case *core.WhileStmt:
		for {
			cond, err := itp.evalExpr(s.Cond, env)
			if err != nil {
				return nil, err
			}
			b, err := asBool(cond, s.Cond.NodeLoc())
			if err != nil {
				return nil, err
			}
			if !b {
				break
			}
			// 执行循环体，捕获 break/continue 信号：
			//   - breakSignal：跳出整个循环（return 正常结束外层 for）。
			//   - continueSignal：跳过本轮剩余，进入下一轮（继续外层 for）。
			//   - returnSignal / 其他 panic：透传（让外层 callFunction/Run 处理）。
			if err := itp.execLoopBody(s.Body, env); err != nil {
				if err == errBreak {
					break
				}
				if err == errContinue {
					continue
				}
				return nil, err
			}
		}
		return nil, nil
	case *core.ForStmt:
		// C 风格 for：init → while(cond) { body; update }
		// 在循环专属子作用域里执行，使 init 的 let i 不污染外层（块级作用域）。
		// update 是裸赋值（LetStmt{IsAssign:true}），Set 沿链更新到此循环作用域的 i。
		loopEnv := NewEnvironment(env)
		if s.Init != nil {
			if _, err := itp.execStmt(s.Init, loopEnv); err != nil {
				return nil, err
			}
		}
		for {
			// cond 为空视为恒真（无限循环，靠 body 内 return/break 跳出）。
			if s.Cond != nil {
				cond, err := itp.evalExpr(s.Cond, loopEnv)
				if err != nil {
					return nil, err
				}
				b, err := asBool(cond, s.Cond.NodeLoc())
				if err != nil {
					return nil, err
				}
				if !b {
					break
				}
			}
			// 同 WhileStmt：捕获 break/continue。
			// 注意 continue 时仍需执行 update（C 风格 for 语义），由下方逻辑保证。
			if err := itp.execLoopBody(s.Body, loopEnv); err != nil {
				if err == errBreak {
					break
				}
				// errContinue：跳过本轮剩余，但仍执行 update 后进入下一轮
			}
			if s.Update != nil {
				if _, err := itp.execStmt(s.Update, loopEnv); err != nil {
					return nil, err
				}
			}
		}
		return nil, nil
	case *core.ExprStmt:
		return itp.evalExpr(s.Expr, env)
	case *core.BlockStmt:
		return itp.execBlock(s, env)
	}
	return nil, core.NewError(stmt.NodeLoc(), "未知语句类型 %T", stmt)
}

// execBlock 在当前 env 上开一个子作用域执行块（块级作用域）。
// 注意：函数体不走这里（callFunction 直接在 callEnv 执行 body.Stmts），
// 所以这里始终开新作用域是安全的：if/while/standalone 块都该有独立作用域。
func (itp *Interpreter) execBlock(blk *core.BlockStmt, env *Environment) (any, error) {
	child := NewEnvironment(env)
	var last any
	for _, s := range blk.Stmts {
		v, err := itp.execStmt(s, child)
		if err != nil {
			return nil, err
		}
		if _, isExpr := s.(*core.ExprStmt); isExpr {
			last = v
		}
	}
	return last, nil
}

// errBreak / errContinue 是循环体执行时把 break/continue 的 panic 信号
// 翻译成的哨兵错误，供 while/for 循环判断控制流。
// 用哨兵错误（而非再次 panic）是因为 execBlock 内部的 for 循环会吞掉
// panic——这里在 execLoopBody 里 recover 后转成错误返回，让循环骨架能
// 用普通的 if 判断处理。
// 用 fmt.Errorf 而非 core.NewError：它们只是内部哨兵，不需要 SourceLoc。
var errBreak = fmt.Errorf("break signal")
var errContinue = fmt.Errorf("continue signal")

// execLoopBody 执行循环体，把 break/continue 的 panic 信号翻译成哨兵错误。
//   - breakSignal → errBreak（循环骨架据此跳出）
//   - continueSignal → errContinue（循环骨架据此进入下一轮）
//   - returnSignal / 其他 panic → 重新抛出（交给外层 callFunction / Run 处理）
//
// 注意：真实 error（execBlock 返回的）原样透传，只有 panic 路径在此翻译。
// 这是必要的：break/continue 从嵌套块/if 里冒泡出来需要穿过 execBlock
// 内部的 for 循环，而那个 for 循环不会捕获 panic。
func (itp *Interpreter) execLoopBody(blk *core.BlockStmt, env *Environment) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(breakSignal); ok {
				err = errBreak
				return
			}
			if _, ok := r.(continueSignal); ok {
				err = errContinue
				return
			}
			// returnSignal 或真实运行时 panic：重新抛出。
			panic(r)
		}
	}()
	_, e := itp.execBlock(blk, env)
	return e
}

// ===== 表达式求值 =====

func (itp *Interpreter) evalExpr(expr core.Expr, env *Environment) (any, error) {
	switch e := expr.(type) {
	case *core.NumberExpr:
		return e.Value, nil
	case *core.StringExpr:
		// 支持模板字符串插值：${expr} 在字符串内嵌入变量/表达式值。
		// 如 "hello ${name}" → name 变量的值替换 ${name}。
		return itp.interpolateString(e.Value, env, e.Loc)
	case *core.BoolExpr:
		return e.Value, nil
	case *core.IdentExpr:
		v, ok := env.Get(e.Name)
		if !ok {
			return nil, core.NewError(e.Loc, "未定义的变量 %q", e.Name)
		}
		return v, nil
	case *core.UnaryExpr:
		right, err := itp.evalExpr(e.Right, env)
		if err != nil {
			return nil, err
		}
		return evalUnary(e.Op, right, e.Loc)
	case *core.BinaryExpr:
		// && || 短路求值（避免副作用 + 类型报错）
		if e.Op == core.TokAnd || e.Op == core.TokOr {
			return itp.evalLogical(e, env)
		}
		left, err := itp.evalExpr(e.Left, env)
		if err != nil {
			return nil, err
		}
		right, err := itp.evalExpr(e.Right, env)
		if err != nil {
			return nil, err
		}
		return evalBinary(e.Op, left, right, e.Loc)
	case *core.CallExpr:
		return itp.evalCall(e, env)
	case *core.ArrayExpr:
		// 数组字面量：求值每个元素，返回 []any
		arr := make([]any, 0, len(e.Elements))
		for _, elem := range e.Elements {
			v, err := itp.evalExpr(elem, env)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	case *core.IndexExpr:
		// 数组索引：求值 array 和 index，返回 array[index]
		arr, err := itp.evalExpr(e.Array, env)
		if err != nil {
			return nil, err
		}
		idx, err := itp.evalExpr(e.Index, env)
		if err != nil {
			return nil, err
		}
		// 支持 []any（数组）和 string（字符串索引，返回单字符 string）
		switch a := arr.(type) {
		case []any:
			i, ok := idx.(int64)
			if !ok {
				return nil, core.NewError(e.Loc, "数组索引必须是整数，实际 %s", typeName(idx))
			}
			if i < 0 || int(i) >= len(a) {
				return nil, core.NewError(e.Loc, "数组索引越界: %d（长度 %d）", i, len(a))
			}
			return a[i], nil
		case string:
			i, ok := idx.(int64)
			if !ok {
				return nil, core.NewError(e.Loc, "字符串索引必须是整数")
			}
			runes := []rune(a)
			if i < 0 || int(i) >= len(runes) {
				return nil, core.NewError(e.Loc, "字符串索引越界: %d（长度 %d）", i, len(runes))
			}
			return string(runes[i]), nil
		}
		return nil, core.NewError(e.Loc, "无法对 %s 做索引操作", typeName(arr))
	case *core.FnExpr:
		// 匿名函数表达式求值：产生一个 function value，捕获当前 env 作为闭包。
		// 复用 FnDecl 表示（Name 留空），让 callFunction / evalCall 统一处理。
		// closure=env 是闭包的关键：函数体内可访问定义时可见的外层变量。
		return &function{
			decl: &core.FnDecl{
				Loc:    e.Loc,
				Name:   "", // 匿名函数无名
				Params: e.Params,
				Body:   e.Body,
			},
			closure: env,
		}, nil
	}
	return nil, core.NewError(expr.NodeLoc(), "未知表达式类型 %T", expr)
}

// evalLogical 短路求值 && / ||。
func (itp *Interpreter) evalLogical(e *core.BinaryExpr, env *Environment) (any, error) {
	left, err := itp.evalExpr(e.Left, env)
	if err != nil {
		return nil, err
	}
	lb, err := asBool(left, e.Left.NodeLoc())
	if err != nil {
		return nil, err
	}
	if e.Op == core.TokAnd {
		if !lb {
			return false, nil // 短路
		}
		right, err := itp.evalExpr(e.Right, env)
		if err != nil {
			return nil, err
		}
		return asBool(right, e.Right.NodeLoc())
	}
	// TokOr
	if lb {
		return true, nil // 短路
	}
	right, err := itp.evalExpr(e.Right, env)
	if err != nil {
		return nil, err
	}
	return asBool(right, e.Right.NodeLoc())
}

// evalCall 求值函数调用。
func (itp *Interpreter) evalCall(e *core.CallExpr, env *Environment) (any, error) {
	// 内置函数 len（对数组/字符串返回长度）
	if e.Callee == "len" && len(e.Args) == 1 {
		v, err := itp.evalExpr(e.Args[0], env)
		if err != nil {
			return nil, err
		}
		switch x := v.(type) {
		case []any:
			return int64(len(x)), nil
		case string:
			return int64(len([]rune(x))), nil
		}
		return nil, core.NewError(e.Loc, "len() 参数必须是数组或字符串，实际 %s", typeName(v))
	}
	// 内置函数 substr(str, start, end)：截取子串（rune 安全）
	if e.Callee == "substr" && len(e.Args) == 3 {
		s, err := itp.evalExpr(e.Args[0], env)
		if err != nil {
			return nil, err
		}
		start, err := itp.evalExpr(e.Args[1], env)
		if err != nil {
			return nil, err
		}
		end, err := itp.evalExpr(e.Args[2], env)
		if err != nil {
			return nil, err
		}
		str, ok := s.(string)
		if !ok {
			return nil, core.NewError(e.Loc, "substr() 第一个参数必须是字符串，实际 %s", typeName(s))
		}
		si, ok1 := start.(int64)
		ei, ok2 := end.(int64)
		if !ok1 || !ok2 {
			return nil, core.NewError(e.Loc, "substr() start/end 必须是整数")
		}
		runes := []rune(str)
		if si < 0 {
			si = 0
		}
		if ei > int64(len(runes)) {
			ei = int64(len(runes))
		}
		if si >= ei {
			return "", nil
		}
		return string(runes[si:ei]), nil
	}
	// 内置函数 join(arr, sep)：把数组元素用分隔符拼成字符串。
	// 元素按 valueToString 转字符串（支持 int/bool/string 等），与 + 拼接语义一致。
	if e.Callee == "join" && len(e.Args) == 2 {
		arr, err := itp.evalExpr(e.Args[0], env)
		if err != nil {
			return nil, err
		}
		sep, err := itp.evalExpr(e.Args[1], env)
		if err != nil {
			return nil, err
		}
		a, ok := arr.([]any)
		if !ok {
			return nil, core.NewError(e.Loc, "join() 第一个参数必须是数组，实际 %s", typeName(arr))
		}
		sepStr, ok := sep.(string)
		if !ok {
			return nil, core.NewError(e.Loc, "join() 第二个参数必须是字符串，实际 %s", typeName(sep))
		}
		parts := make([]string, len(a))
		for i, v := range a {
			parts[i] = valueToString(v)
		}
		return strings.Join(parts, sepStr), nil
	}
	// 内置函数 split(str, sep)：按分隔符分割字符串，返回字符串数组。
	// 是 join 的逆操作：split("a,b,c", ",") → ["a","b","c"]。
	// 注意：Go 的 strings.Split("","x") 返回 [""]（长度 1），保持该语义不变。
	if e.Callee == "split" && len(e.Args) == 2 {
		s, err := itp.evalExpr(e.Args[0], env)
		if err != nil {
			return nil, err
		}
		sep, err := itp.evalExpr(e.Args[1], env)
		if err != nil {
			return nil, err
		}
		str, ok := s.(string)
		if !ok {
			return nil, core.NewError(e.Loc, "split() 第一个参数必须是字符串，实际 %s", typeName(s))
		}
		sepStr, ok := sep.(string)
		if !ok {
			return nil, core.NewError(e.Loc, "split() 第二个参数必须是字符串，实际 %s", typeName(sep))
		}
		parts := strings.Split(str, sepStr)
		out := make([]any, len(parts))
		for i, p := range parts {
			out[i] = p
		}
		return out, nil
	}
	// 内置函数 range(n)：返回 [0, 1, ..., n-1] 数组。
	// 用于 for 循环遍历索引、生成序列。n<0 返回空数组。
	if e.Callee == "range" && len(e.Args) == 1 {
		v, err := itp.evalExpr(e.Args[0], env)
		if err != nil {
			return nil, err
		}
		n, ok := v.(int64)
		if !ok {
			return nil, core.NewError(e.Loc, "range() 参数必须是整数，实际 %s", typeName(v))
		}
		if n <= 0 {
			return []any{}, nil
		}
		out := make([]any, n)
		for i := int64(0); i < n; i++ {
			out[i] = i
		}
		return out, nil
	}
	// 内置函数 charAt(str, index)：返回指定位置的单字符字符串
	if e.Callee == "charAt" && len(e.Args) == 2 {
		s, err := itp.evalExpr(e.Args[0], env)
		if err != nil {
			return nil, err
		}
		idx, err := itp.evalExpr(e.Args[1], env)
		if err != nil {
			return nil, err
		}
		str, ok := s.(string)
		if !ok {
			return nil, core.NewError(e.Loc, "charAt() 第一个参数必须是字符串")
		}
		i, ok := idx.(int64)
		if !ok {
			return nil, core.NewError(e.Loc, "charAt() index 必须是整数")
		}
		runes := []rune(str)
		if i < 0 || int(i) >= len(runes) {
			return nil, core.NewError(e.Loc, "charAt() 索引越界: %d（长度 %d）", i, len(runes))
		}
		return string(runes[i]), nil
	}
	// 内置函数 min
	if e.Callee == "min" && len(e.Args) == 2 {
		a, err := itp.evalExpr(e.Args[0], env)
		if err != nil {
			return nil, err
		}
		b, err := itp.evalExpr(e.Args[1], env)
		if err != nil {
			return nil, err
		}
		ai, ok1 := a.(int64)
		bi, ok2 := b.(int64)
		if !ok1 || !ok2 {
			return nil, core.NewError(e.Loc, "min() 需要整数参数")
		}
		if ai < bi {
			return ai, nil
		}
		return bi, nil
	}
	// 内置函数 max
	if e.Callee == "max" && len(e.Args) == 2 {
		a, err := itp.evalExpr(e.Args[0], env)
		if err != nil {
			return nil, err
		}
		b, err := itp.evalExpr(e.Args[1], env)
		if err != nil {
			return nil, err
		}
		ai, ok1 := a.(int64)
		bi, ok2 := b.(int64)
		if !ok1 || !ok2 {
			return nil, core.NewError(e.Loc, "max() 需要整数参数")
		}
		if ai > bi {
			return ai, nil
		}
		return bi, nil
	}
	// 内置函数 abs
	if e.Callee == "abs" && len(e.Args) == 1 {
		v, err := itp.evalExpr(e.Args[0], env)
		if err != nil {
			return nil, err
		}
		if i, ok := v.(int64); ok {
			if i < 0 {
				return -i, nil
			}
			return i, nil
		}
		return nil, core.NewError(e.Loc, "abs() 参数必须是整数")
	}
	// 内置函数 push(arr, val)：向数组追加元素，返回新数组（不修改原数组）
	if e.Callee == "push" && len(e.Args) == 2 {
		arr, err := itp.evalExpr(e.Args[0], env)
		if err != nil {
			return nil, err
		}
		val, err := itp.evalExpr(e.Args[1], env)
		if err != nil {
			return nil, err
		}
		a, ok := arr.([]any)
		if !ok {
			return nil, core.NewError(e.Loc, "push() 第一个参数必须是数组，实际 %s", typeName(arr))
		}
		// 返回新数组（不修改原数组）
		out := make([]any, len(a)+1)
		copy(out, a)
		out[len(a)] = val
		return out, nil
	}
	fn, ok := itp.funcs[e.Callee]
	if !ok {
		// 不在函数表里：检查环境里是否有同名变量且值是 function（一等函数调用）。
		// 支持 let f = fn(x){...}; f(5); 这类通过变量名调用匿名/闭包函数。
		if v, vok := env.Get(e.Callee); vok {
			if fv, fok := v.(*function); fok {
				fn = fv
			}
		}
	}
	if fn == nil {
		return nil, core.NewError(e.Loc, "未定义的函数 %q", e.Callee)
	}
	if len(e.Args) != len(fn.decl.Params) {
		return nil, core.NewError(e.Loc, "函数 %q 期望 %d 个参数，实际 %d 个",
			e.Callee, len(fn.decl.Params), len(e.Args))
	}
	// 求值参数（在调用者环境）
	args := make([]any, len(e.Args))
	for i, a := range e.Args {
		v, err := itp.evalExpr(a, env)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}
	// 新建函数环境：parent 指向函数的闭包环境（定义时捕获的环境）。
	//   - 顶层命名函数的 closure=globals，行为不变。
	//   - 一等函数（FnExpr 产生的 function value）的 closure=定义时 env，
	//     实现闭包：函数体可访问外层局部变量。
	callEnv := NewEnvironment(fn.closure)
	for i, name := range fn.decl.Params {
		callEnv.Set(name, args[i])
	}
	// 执行函数体，捕获 return 信号
	result, err := itp.callFunction(fn, callEnv)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// callFunction 执行函数体并用 recover 捕获 return 信号。
func (itp *Interpreter) callFunction(fn *function, callEnv *Environment) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			if sig, ok := r.(returnSignal); ok {
				result = sig.value
				err = nil
				return
			}
			// 非 return 的 panic：透传
			panic(r)
		}
	}()
	_, e := itp.execBlock(fn.decl.Body, callEnv)
	return nil, e
}

// ===== 一元 / 二元运算（动态类型分发） =====

func evalUnary(op core.TokenType, right any, loc core.SourceLoc) (any, error) {
	switch op {
	case core.TokMinus:
		n, ok := right.(int64)
		if !ok {
			return nil, core.NewError(loc, "一元 - 需要整数，实际 %s", typeName(right))
		}
		return -n, nil
	case core.TokNot:
		b, ok := right.(bool)
		if !ok {
			return nil, core.NewError(loc, "一元 ! 需要布尔，实际 %s", typeName(right))
		}
		return !b, nil
	}
	return nil, core.NewError(loc, "未知一元运算符 %s", core.TokenName(op))
}

func evalBinary(op core.TokenType, left, right any, loc core.SourceLoc) (any, error) {
	switch op {
	// 加法（双语义，类 JS/Python）：int + int = int，string + any = 字符串拼接。
	// 动态类型语言的标准行为，让 greet() 等字符串场景可用。
	case core.TokPlus:
		ln, lok := left.(int64)
		rn, rok := right.(int64)
		if lok && rok {
			return ln + rn, nil
		}
		ls, lsok := left.(string)
		rs, rsok := right.(string)
		if lsok && rsok {
			return ls + rs, nil
		}
		// string + 非string / 非string + string：把另一侧转字符串拼接
		if lsok {
			return ls + valueToString(right), nil
		}
		if rsok {
			return valueToString(left) + rs, nil
		}
		return nil, core.NewError(loc, "运算符 + 需要整数或字符串，实际 %s 和 %s",
			typeName(left), typeName(right))
	// 算术（仅整数）：- * / %
	case core.TokMinus, core.TokStar, core.TokSlash, core.TokPercent:
		ln, lok := left.(int64)
		rn, rok := right.(int64)
		if !lok || !rok {
			return nil, core.NewError(loc, "运算符 %s 需要整数，实际 %s 和 %s",
				core.TokenName(op), typeName(left), typeName(right))
		}
		switch op {
		case core.TokMinus:
			return ln - rn, nil
		case core.TokStar:
			return ln * rn, nil
		case core.TokSlash:
			if rn == 0 {
				return nil, core.NewError(loc, "除零错误")
			}
			return ln / rn, nil
		case core.TokPercent:
			if rn == 0 {
				return nil, core.NewError(loc, "模零错误")
			}
			return ln % rn, nil
		}
	// 比较（仅整数，结果是 bool）
	case core.TokGT, core.TokLT, core.TokGE, core.TokLE:
		ln, lok := left.(int64)
		rn, rok := right.(int64)
		if !lok || !rok {
			return nil, core.NewError(loc, "比较运算 %s 需要整数，实际 %s 和 %s",
				core.TokenName(op), typeName(left), typeName(right))
		}
		switch op {
		case core.TokGT:
			return ln > rn, nil
		case core.TokLT:
			return ln < rn, nil
		case core.TokGE:
			return ln >= rn, nil
		case core.TokLE:
			return ln <= rn, nil
		}
	// 相等（int64 / bool / string 都可比，类型不同判为不相等）
	case core.TokEQ:
		return equalValues(left, right), nil
	case core.TokNE:
		return !equalValues(left, right), nil
	}
	return nil, core.NewError(loc, "未知二元运算符 %s", core.TokenName(op))
}

// equalValues 判断两个动态值是否"相等"。
// 同类型且值相等 → true；类型不同 → false（不报错，松散相等）。
func equalValues(a, b any) bool {
	switch x := a.(type) {
	case int64:
		y, ok := b.(int64)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	case nil:
		return b == nil
	}
	return false
}

// ===== 类型辅助 =====

// asBool 把动态值断言为 bool（条件表达式用）。
func asBool(v any, loc core.SourceLoc) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, core.NewError(loc, "需要布尔，实际 %s", typeName(v))
	}
	return b, nil
}

// typeName 返回值的类型可读名（错误信息用）。
func typeName(v any) string {
	switch v.(type) {
	case int64:
		return "int"
	case bool:
		return "bool"
	case string:
		return "string"
	case nil:
		return "nil"
	}
	return fmt.Sprintf("%T", v)
}

// valueToString 把任意值转成字符串（用于 string + 非string 拼接）。
func valueToString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int64:
		return fmt.Sprintf("%d", x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case nil:
		return "nil"
	}
	return fmt.Sprintf("%v", v)
}

// interpolateString 处理字符串内的 ${expr} 插值。
func (itp *Interpreter) interpolateString(s string, env *Environment, loc core.SourceLoc) (string, error) {
	if !strings.Contains(s, "${") {
		return s, nil
	}
	var result strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '$' && s[i+1] == '{' {
			depth := 1
			j := i + 2
			for j < len(s) && depth > 0 {
				if s[j] == '{' {
					depth++
				} else if s[j] == '}' {
					depth--
				}
				if depth > 0 {
					j++
				}
			}
			if depth != 0 {
				return "", core.NewError(loc, "字符串插值 ${...} 未闭合")
			}
			exprStr := s[i+2:j] + ";"
			tokens, err := lexer.Tokenize(exprStr)
			if err != nil {
				return "", core.NewError(loc, "字符串插值词法错误: %v", err)
			}
			prog, err := parser.Parse(tokens)
			if err != nil {
				return "", core.NewError(loc, "字符串插值语法错误: %v", err)
			}
			v, err := itp.Run(prog)
			if err != nil {
				return "", err
			}
			result.WriteString(valueToString(v))
			i = j + 1
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String(), nil
}

// FormatValue 把解释器的一个值格式化成字符串（demo/REPL 用）。
func FormatValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "nil"
	case int64:
		return strconv.FormatInt(x, 10)
	case bool:
		return strconv.FormatBool(x)
	case string:
		return strconv.Quote(x)
	}
	return fmt.Sprintf("%v", v)
}
