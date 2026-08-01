// Package parser 实现 M 语言的递归下降语法分析，更多背景见 NOTES.md。
//
// 递归下降（recursive descent）原理：
// 每条文法规则对应一个解析函数。函数之间按"运算符优先级阶梯"分层——
// 低优先级运算符在外层调用高优先级解析器，从而让 * 比 + 绑定更紧。
//
//	parseExpr
//	  └─ parseOr        (||)
//	       └─ parseAnd       (&&)
//	            └─ parseEquality  (== !=)
//	                 └─ parseComparison (> < >= <=)
//	                      └─ parseTerm (+ -)
//	                           └─ parseFactor (* / %)
//	                                └─ parseUnary (! -)
//	                                     └─ parsePrimary
//
// 同级运算符用 while 循环卷起实现左结合；一元运算符用递归实现右结合。
//
// 与 lexer 的边界：
//   - lexer 切 token；parser 消费 []core.Token，构造 AST。
//   - lexer 不管"括号是否匹配"，那是 parser 的责任。
//
// 与 interpreter 的边界：
//   - parser 只校验"语法形状"（如 expr ";"、括号配对、关键字位置）。
//   - parser 不做类型检查（"a" + 1 是合法 AST，类型错误由 interpreter 报）。
//   - parser 不求值、不执行——它只产出 *core.Program。
package parser
