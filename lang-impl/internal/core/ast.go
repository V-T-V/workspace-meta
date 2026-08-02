package core

// AST 节点定义。M 语言的 AST 用接口 + 具体类型（discriminated struct）表示，
// 便于解释器（visitor 模式）和 WASM 后端（codegen）分别遍历。
//
// 节点分两类：
//   - Expr：表达式（有返回值）：Number/String/Bool/Ident/Binary/Unary/Call
//   - Stmt：语句（无返回值）：Let/FnDecl/If/While/For/Return/ExprStmt/Block

// Node 是所有 AST 节点的基接口。
type Node interface {
	NodeLoc() SourceLoc
}

// Expr 是表达式节点接口。
type Expr interface {
	Node
	exprNode()
}

// Stmt 是语句节点接口。
type Stmt interface {
	Node
	stmtNode()
}

// ===== 表达式节点 =====

// NumberExpr 整数字面量。
type NumberExpr struct {
	Loc   SourceLoc
	Value int64
}

func (NumberExpr) exprNode()            {}
func (e NumberExpr) NodeLoc() SourceLoc { return e.Loc }

// StringExpr 字符串字面量（不含引号）。
type StringExpr struct {
	Loc   SourceLoc
	Value string
}

func (StringExpr) exprNode()            {}
func (e StringExpr) NodeLoc() SourceLoc { return e.Loc }

// BoolExpr 布尔字面量。
type BoolExpr struct {
	Loc   SourceLoc
	Value bool
}

func (BoolExpr) exprNode()            {}
func (e BoolExpr) NodeLoc() SourceLoc { return e.Loc }

// IdentExpr 标识符引用。
type IdentExpr struct {
	Loc  SourceLoc
	Name string
}

func (IdentExpr) exprNode()            {}
func (e IdentExpr) NodeLoc() SourceLoc { return e.Loc }

// BinaryExpr 二元运算：left op right。
type BinaryExpr struct {
	Loc   SourceLoc
	Op    TokenType
	Left  Expr
	Right Expr
}

func (BinaryExpr) exprNode()            {}
func (e BinaryExpr) NodeLoc() SourceLoc { return e.Loc }

// UnaryExpr 一元运算：op right（目前仅 ! 和 -）。
type UnaryExpr struct {
	Loc   SourceLoc
	Op    TokenType
	Right Expr
}

func (UnaryExpr) exprNode()            {}
func (e UnaryExpr) NodeLoc() SourceLoc { return e.Loc }

// CallExpr 函数调用：callee(args...)。
type CallExpr struct {
	Loc    SourceLoc
	Callee string // 函数名
	Args   []Expr
}

func (CallExpr) exprNode()            {}
func (e CallExpr) NodeLoc() SourceLoc { return e.Loc }

// ArrayExpr 数组字面量：[expr, expr, ...]。
type ArrayExpr struct {
	Loc      SourceLoc
	Elements []Expr
}

func (ArrayExpr) exprNode()            {}
func (e ArrayExpr) NodeLoc() SourceLoc { return e.Loc }

// IndexExpr 数组索引：arr[index]。
type IndexExpr struct {
	Loc   SourceLoc
	Array Expr
	Index Expr
}

func (IndexExpr) exprNode()            {}
func (e IndexExpr) NodeLoc() SourceLoc { return e.Loc }

// FnExpr 匿名函数表达式（一等函数 / function value）：fn(params) { body }。
// 与 FnDecl 的区别：没有名字，是表达式而非语句，可出现在任意表达式位置
// （let 绑定、函数参数、数组元素、return 值等）。
//
// 例子：let f = fn(x) { return x + 1; };
// 求值时产生一个 function value，捕获定义时的环境作为闭包。
type FnExpr struct {
	Loc    SourceLoc
	Params []string
	Body   *BlockStmt
}

func (FnExpr) exprNode()            {}
func (e FnExpr) NodeLoc() SourceLoc { return e.Loc }

// ===== 语句节点 =====

// LetStmt 变量绑定：let name = expr;
type LetStmt struct {
	Loc  SourceLoc
	Name string
	Init Expr
	// IsAssign 区分"let x = ..."（false，Define 当前层新建）和"x = ..."（true，Set 沿链更新）。
	// parser 的 parseLet 设 false，parseAssign 设 true。
	IsAssign bool
}

func (LetStmt) stmtNode()            {}
func (s LetStmt) NodeLoc() SourceLoc { return s.Loc }

// FnDecl 函数声明：fn name(params) { body }
type FnDecl struct {
	Loc    SourceLoc
	Name   string
	Params []string
	Body   *BlockStmt
}

func (FnDecl) stmtNode()            {}
func (s FnDecl) NodeLoc() SourceLoc { return s.Loc }

// IfStmt 条件分支：if (cond) { then } else { else }
type IfStmt struct {
	Loc  SourceLoc
	Cond Expr
	Then *BlockStmt
	Else *BlockStmt // 可为 nil（无 else）
}

func (IfStmt) stmtNode()            {}
func (s IfStmt) NodeLoc() SourceLoc { return s.Loc }

// WhileStmt 循环：while (cond) { body }
type WhileStmt struct {
	Loc  SourceLoc
	Cond Expr
	Body *BlockStmt
}

func (WhileStmt) stmtNode()            {}
func (s WhileStmt) NodeLoc() SourceLoc { return s.Loc }

// ForStmt 是 C 风格 for 循环：for (init; cond; update) { body }
// 语义等价于：init; while (cond) { body; update; }
//   - Init 是循环前执行一次的语句（通常是 let i = 0），可为 nil
//   - Cond 是每轮执行前求值的条件（假则退出），可为 nil 表示恒真
//   - Update 是每轮 body 后执行的语句（通常是裸赋值 i = i + 1），可为 nil
//   - Body 是循环体
type ForStmt struct {
	Loc    SourceLoc
	Init   Stmt
	Cond   Expr
	Update Stmt
	Body   *BlockStmt
}

func (ForStmt) stmtNode()            {}
func (s ForStmt) NodeLoc() SourceLoc { return s.Loc }

// ReturnStmt 返回：return expr;（expr 可为 nil 表示 return;）
type ReturnStmt struct {
	Loc   SourceLoc
	Value Expr // 可为 nil
}

func (ReturnStmt) stmtNode()            {}
func (s ReturnStmt) NodeLoc() SourceLoc { return s.Loc }

// BreakStmt 跳出循环：break;
// 只在 while/for 循环体内合法；解释器通过 panic(breakSignal{}) 实现非局部跳转，
// 由最近的循环语句（while/for）的执行体用 defer recover 捕获。
type BreakStmt struct {
	Loc SourceLoc
}

func (BreakStmt) stmtNode()            {}
func (s BreakStmt) NodeLoc() SourceLoc { return s.Loc }

// ContinueStmt 跳过本轮循环剩余语句、进入下一轮：continue;
// 同 BreakStmt，由最近的循环语句捕获 continueSignal。
type ContinueStmt struct {
	Loc SourceLoc
}

func (ContinueStmt) stmtNode()            {}
func (s ContinueStmt) NodeLoc() SourceLoc { return s.Loc }

// ExprStmt 表达式语句：expr;
type ExprStmt struct {
	Loc  SourceLoc
	Expr Expr
}

func (ExprStmt) stmtNode()            {}
func (s ExprStmt) NodeLoc() SourceLoc { return s.Loc }

// BlockStmt 语句块：{ stmts... }
type BlockStmt struct {
	Loc   SourceLoc
	Stmts []Stmt
}

func (BlockStmt) stmtNode()            {}
func (s BlockStmt) NodeLoc() SourceLoc { return s.Loc }

// Program 是整个程序的 AST 根：一组顶层语句（函数声明 + 全局 let）。
type Program struct {
	Loc   SourceLoc
	Stmts []Stmt
}
